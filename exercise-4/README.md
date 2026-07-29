# Exercise 4 — Circuit Breaker with Health Checker

Standalone Go module. Run it on its own, independent of any other exercise in this
repo:

```bash
cd exercise-4
go run .
```

## Spec

Implement a circuit breaker in Go that protects a downstream call from repeated
failures, plus a background health checker that can force the breaker's state
independently of call outcomes.

Build a `CircuitBreaker` type that:

- Starts `Closed` and allows every call through.
- Opens after 5 consecutive failures while `Closed`.
- Stays `Open` for a fixed cooldown (5s), then moves to `HalfOpen`.
- While `HalfOpen`, allows at most 3 trial calls through; if at least 2 of the
  3 outcomes are successes, closes again — otherwise re-opens.
- Exposes `Allow() bool`, `RecordSuccess()`, `RecordFailure()`.
- Is safe for concurrent use — many goroutines call `Allow`/`Record*` at once.

Build a `HealthChecker` type that:

- Polls health on a fixed interval (2s) and exposes `IsHealthy() bool`.
- Publishes state *changes* (not every poll) on a channel via `Changes()`.
- Stops cleanly when its `context.Context` is cancelled.

Wire them together: `CircuitBreaker.AssignHealthChecker` consumes the checker's
initial status plus every subsequent change, and forces the breaker `Open` on
`Unhealthy` / `Closed` on `Healthy` — overriding whatever the failure-count
state machine was doing.

### Key design question

Two independent things can move the breaker's state — call outcomes
(`RecordSuccess`/`RecordFailure`) and external health signals
(`AssignHealthChecker`) — both from different goroutines. How do they share
one state machine without racing?

**Answer used here:** a single `sync.Mutex` guards all breaker state, and
every entry point (`Allow`, `RecordSuccess`, `RecordFailure`,
`applyHealthStatus`) takes the lock before touching `state`/`failures`/
`halfOpen*`. `moveToOpen`/`moveToClosed` are plain unexported helpers with no
locking of their own — they're only ever called from inside an
already-locked method, so the lock is taken exactly once per public call, not
once per helper.

## What each requirement maps to in the code

| # | Concept | Where |
|---|---|---|
| 1 | Starts closed, calls allowed | `NewCircuitBreaker` — `state: CircuitClosed`; `Allow` returns `true` in that state |
| 2 | Opens after 5 consecutive failures | `RecordFailure` (`CircuitClosed` case) — increments `failures`, `moveToOpen` at `>= 5` |
| 3 | Fixed cooldown, then half-open | `Allow` — checks `time.Since(c.openedAt) < 5*time.Second` before letting `CircuitOpen` fall through to `CircuitHalfOpen` |
| 4 | Half-open trial cap (3) | `tryReserveOperation` — refuses once `halfOpenAttempts >= 3` |
| 5 | Half-open close/reopen decision | `evaluateHalfOpen` — waits for 3 completed trials, closes on `>= 2` successes, else reopens |
| 6 | Concurrency safety | `circuitBreaker.mu sync.Mutex`, locked in every exported method |
| 7 | Periodic health polling | `healthChecker.check` — `time.Ticker` (2s) driving `setHealthy`/`setUnHealthy` |
| 8 | Publish changes only | `setHealthy`/`setUnHealthy` — early-return if the status didn't actually change, so `publish` only fires on a transition |
| 9 | Health checker clean shutdown | `check` — `select` on `ctx.Done()`, `defer` stops the ticker and closes `changes` |
| 10 | Health signal overrides breaker state | `AssignHealthChecker` + `applyHealthStatus` — forces `moveToClosed`/`moveToOpen` regardless of the failure-count path |

## Notable implementation details

- **Non-blocking "latest value wins" publish**: `publish` first tries a
  non-blocking send; if the size-1 buffered channel is already full it drains
  the stale value and sends the new one. A slow consumer never blocks the
  health-check loop, and always sees the *latest* status rather than a queue
  of stale ones.
- **Health status is authoritative over the failure counter**: an `Unhealthy`
  signal forces `Open` even mid-half-open-trial, and a `Healthy` signal forces
  `Closed` even after 5 straight failures. This is a deliberate deviation from
  a pure failure-counting breaker — the health checker is treated as
  ground truth, the failure counter as a heuristic for when no external
  signal is available.
- **`getState` is currently unused** — kept as the obvious read-only
  accessor for a future test or metrics hook; harmless since it just takes
  the lock and returns a copy.
