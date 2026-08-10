# Exercise 6 — Bounded Retry with Exponential Backoff + Jitter, Context-Aware

Standalone Go module. Run it on its own, independent of any other exercise in this
repo:

```bash
cd exercise-6
go run .
```

## Spec

Build a `Retry` function that wraps a call to an unreliable operation and retries it
intelligently.

- Signature roughly: `func Retry(ctx context.Context, maxAttempts int, f func(ctx context.Context) error) error`
- On failure, wait before retrying — with exponential backoff (e.g., 100ms, 200ms,
  400ms, 800ms, doubling each time).
- Add jitter to the backoff, so many simultaneous callers don't all retry at the exact
  same instant (avoids a "thundering herd" against a recovering downstream service).
- The wait between retries must respect `ctx` — cancellation should stop the wait
  immediately, not sleep out the full backoff duration.
- After `maxAttempts` failures, return the last underlying error (don't lose the
  original failure reason).
- Base delay should be configurable (constructor parameter), not hardcoded.

### Key design question

Where does the attempt count live, and how does jitter interact with exponential
growth? The .NET analogue for the ctx-aware wait is `await Task.Delay(d,
cancellationToken)` racing a timeout — Go has no built-in equivalent, so it has to be
built by hand with `select`.

**Answer used here:**
- Attempt count is a local variable inside `Retry`, not a field on the `retry`
  struct — a struct field would be shared/mutated across concurrent, unrelated calls
  through the same `Retry` instance. Local state per call keeps the struct itself
  stateless and safe to reuse concurrently.
- Backoff uses **full jitter**: `rand.Int63n(baseDelay * 2^(attempt-1))` — the wait is
  uniformly random between 0 and the full exponentially-grown delay, not a fixed delay
  plus a small random offset. This is a named, intentional strategy (see AWS's
  "Exponential Backoff and Jitter" writeup), not a bug — it trades a consistent
  minimum wait for better herd-avoidance. "Equal jitter" (`delay/2 +
  random(0, delay/2)`) is the alternative if a guaranteed minimum wait matters more.
- The wait is `select { case <-time.After(jittered): case <-ctx.Done(): return
  ctx.Err() }` — cancellation always wins if it fires first, regardless of how much of
  the backoff has elapsed.

## Requirement → code map

| # | Requirement | Where |
|---|---|---|
| 1 | `Retry(ctx, maxAttempts, f)` signature | `Retry` interface + `(*retry).Retry` in `retry.go` |
| 2 | Exponential backoff, doubling | `math.Pow(2, float64(i-1))` multiplier on `sleepBetweenAttempts` |
| 3 | Jitter to avoid thundering herd | `rand.Int63n(...)` — full jitter over the computed delay |
| 4 | Wait respects `ctx` cancellation | `select` on `time.After(jittered)` vs `ctx.Done()` |
| 5 | Returns last underlying error after `maxAttempts` | `i == maxAttempts` branch returns `err` from the last `f` call, not a wrapped/synthetic error |
| 6 | Base delay configurable, not hardcoded | `NewRetry(sleepBetweenAttempts time.Duration)` constructor param, `defaultSleepBetweenAttempts` only used as a fallback when `<= 0` |

## Notable implementation details

- **Attempt counter increments before the `maxAttempts` check**, not after — checking
  first and incrementing after would let `f` run one extra time beyond
  `maxAttempts`. `i++` then `if i == maxAttempts` gives exactly `maxAttempts` calls to
  `f`.
- **No sleep after the last allowed attempt fails.** The `maxAttempts` check and
  `return err` happen before the backoff `select`, so the function returns immediately
  on final failure instead of waiting out a backoff nothing will use.
- **`for { }` with no loop-header condition, every exit an explicit `return`.**
  Satisfies Go's "missing return" check without a dead trailing `return` after the
  loop — every code path through the function body already returns.
- **Verified against the spec's test scenario** (always-failing `f`, `maxAttempts=5`):
  exact attempt count with no off-by-one, delays trend upward per the doubling
  schedule (full jitter makes individual samples noisy — that's expected, not a bug),
  and a `context.WithTimeout` shorter than the full retry budget (500ms budget vs. a
  backoff schedule that would otherwise run past it) causes an early bail-out
  mid-backoff — 4 attempts completed, the 5th cut off by `ctx.Err()` instead of
  waiting out its full jittered delay.
