# Exercise 9 — Idiomatic Error Handling (Sentinel Errors)

Standalone Go module. Run it on its own:

```bash
cd exercise-9
go run .
```

The demo drives the breaker Closed → Open → HalfOpen and prints the reason
`Allow()` returns at each stage.

## Spec

Replace ad-hoc `errors.New("...")` calls scattered at call sites with
package-level sentinel error variables:

```go
var (
    ErrCircuitOpen          = errors.New("circuit open")
    ErrHalfOpenLimitReached = errors.New("half-open limit reached")
)
```

Change `CircuitBreaker.Allow()` from returning `bool` to returning `error`
(`nil` = allowed). This is a real design improvement, not just a mechanical
change — a `bool` can only say "no," while a distinct sentinel error lets a
caller know *why* they were rejected (open vs. half-open limit), which matters
for logging / metrics / behaviour differences downstream.

### Key design question

Should `Allow()` return `bool` or `error`?

**Answer used here:** `error`. The half-open state and the open state are two
different rejection reasons a caller legitimately wants to tell apart — an open
breaker means "come back after the cooldown," a half-open limit means "a trial
is already in flight, retry in a moment." A `bool` collapses both into "no." The
sentinels are compared with `errors.Is`, so the breaker stays free to wrap them
with context later without breaking callers. This mirrors .NET moving from a
`bool TryX()` to throwing a specific exception subclass when the failure reason
carries information.

## Requirement → code map

| # | Requirement | Where |
|---|---|---|
| 1 | Package-level sentinel `var`s | `errors.go` — `ErrCircuitOpen`, `ErrHalfOpenLimitReached`, `ErrUnknown` |
| 2 | `Allow()` returns `error`, not `bool` | `CircuitBreaker.go` — interface `Allow() error`; `circuitBreaker.Allow` returns `nil` when permitted |
| 3 | Open breaker → distinct reason | `Allow` — returns `ErrCircuitOpen` while `time.Since(openedAt) < 5s`, and in the `CircuitOpen` switch arm |
| 4 | Half-open trial limit → distinct reason | `Allow` — `tryReserveOperation()` caps trials at 3; 4th call returns `ErrHalfOpenLimitReached` |
| 5 | No `errors.New` at call sites | grep: only `errors.go` calls `errors.New` |
| 6 | Caller can branch on the reason | `main.go` — `classify()` uses `errors.Is` to map each sentinel to a log line |

## Notable implementation details

- **`errors.Is`, not `==`.** `main.go` compares with `errors.Is` even though the
  breaker returns the sentinels bare today. Costs nothing now, keeps the door
  open for `fmt.Errorf("...: %w", ErrCircuitOpen)` later.
- **`ErrUnknown` for the unreachable `default`.** The state `switch` in `Allow`
  is exhaustive, but Go doesn't know that — the `default` arm returns a sentinel
  rather than `nil` (which would silently *allow* an impossible state) or a
  freshly built `errors.New` (back to the anti-pattern this exercise removes).
- **Demo is deterministic.** No random failure rate — phase 1 records exactly 5
  failures to open the breaker, phase 3 sleeps out the exact 5s cooldown, so the
  output is the same every run and each of the three `Allow()` outcomes shows up
  once.
