# Exercise 8 — Testing Concurrent Go Code

Standalone Go module. Run its tests on their own, independent of any other exercise
in this repo:

```bash
cd exercise-8
go test -race -v ./...
```

## Spec

### Part 1 — Table-driven test on pure logic

Extract the backoff-delay calculation out of `Retry` into small, pure,
independently-testable functions:

- `computeBackoffDelay(attempt int, base time.Duration) time.Duration` —
  deterministic, no randomness: `base * 2^(attempt-1)`
- `withJitter(duration time.Duration) time.Duration` — takes an already-computed
  delay, returns a randomized value in `[0, duration]`

Write a table-driven test for `computeBackoffDelay` using hand-calculated expected
values (100ms, 200ms, 400ms, 800ms for attempts 1-4) — not values re-derived using
the same formula as the code under test, since that only checks self-consistency,
not correctness.

Write a separate test for `withJitter` using a range/bounds check instead of exact
equality (the output is random), run across many iterations (100) to catch a subtle
off-by-range bug that a single lucky call could miss.

### Part 2 — Testing a blocking, cancellable call (`TokenBucket.Allow`)

Write a test that proves `Allow` returns promptly when its context is cancelled
**while it's actively blocked waiting** — not just when handed an already-cancelled
context from the start (those are different things to prove).

### Part 3 — `-race`, seen for real

Deliberately remove the mutex lock from one method of a concurrency-safe type
(`CircuitBreaker.RecordFailure`), hammer it from many concurrent goroutines, and run
`go test -race` to see a real data race reported — then restore the lock and confirm
it's gone.

## Key design question

**Part 2 — how do you prove cancellation-while-blocked, not just cancellation-before-call?**

Run `Allow` in its own goroutine, synchronize with a `ready` channel (`close(ready)`
right before the blocking call in the goroutine) so the test knows the goroutine has
started before it cancels, then assert the result arrives via a buffered result
channel, guarded by a `time.After` timeout so a genuinely broken/hanging
implementation fails the test cleanly instead of hanging the whole suite.

**Answer used here — why `close()` is safe even without eliminating every
nanosecond of interleaving:** `context.Context` cancellation is implemented via
`close()` on an internal channel, and a closed channel is a *permanent* state any
receiver observes correctly regardless of exact timing — there's no window where a
goroutine can "miss" a close the way it can miss a sent value. That's different from
racing against a *sent value* on a channel, which genuinely can be dropped if no
receiver is listening at the instant of the send. Rule of thumb applied throughout
this module's tests: use `close()` for one-time broadcast-style signals (shutdown,
cancellation, "ready"); use a send for delivering a discrete value to exactly one
recipient.

## Requirement → code map

| # | Requirement | Where |
|---|---|---|
| 1 | `computeBackoffDelay` extracted, pure, deterministic | `retry.go` |
| 1 | `withJitter` extracted, takes computed delay, returns `[0, duration]` | `retry.go` |
| 1 | Table-driven test, hand-calculated expected values | `TestComputeBackOffDelay` in `retry_test.go` |
| 1 | Bounds-check test over many iterations for `withJitter` | `TestWithJitter` in `retry_test.go` (100 iterations) |
| 2 | Blocked-then-cancelled proof, not just pre-cancelled | `TestAllow/context_canceled_while_blocked` in `tokenbucket_test.go` — `ready` channel + `time.After` timeout guard |
| 3 | Concurrent hammer test on a mutex-guarded method | `TestCircuitBreaker_RecordFailure_ConcurrentSafe` in `circuit_breaker_test.go` |
| 3 | Race reproduced by removing the lock, then fixed | See "Notable implementation details" below for the captured `-race` output |

## Notable implementation details

- **The committed `circuit_breaker.go` keeps its mutex** — Part 3 is a
  demonstration, not a permanent regression. The lock was actually commented out in
  `RecordFailure`, `go test -race` was run against the concurrent hammer test, the
  race was captured, and the lock was restored before committing. Captured output:

  ```
  === RUN   TestCircuitBreaker_RecordFailure_ConcurrentSafe
  ==================
  WARNING: DATA RACE
  Read at 0x00c00001a360 by goroutine 14:
    circuitBreaker.RecordFailure()
        circuit_breaker.go:47 +0x30
  Previous write at 0x00c00001a360 by goroutine 9:
    circuitBreaker.moveToOpen()
        circuit_breaker.go:125 +0x104
      circuitBreaker.RecordFailure()
          circuit_breaker.go:51 +0xff
  ==================
      testing.go:1712: race detected during execution of test
  --- FAIL: TestCircuitBreaker_RecordFailure_ConcurrentSafe (0.00s)
  FAIL
  ```

  The read at line 47 is the `switch c.state` in `RecordFailure`; the write at line
  125 is `c.state = CircuitOpen` inside `moveToOpen`, called from a *different*
  goroutine's concurrent `RecordFailure`. With the lock restored, `go test -race
  ./...` passes clean — no other method needed touching, since `Allow`,
  `RecordSuccess`, and `getState` were never modified.
- **`circuit_breaker.go` is reused as-is** from exercise-7 (originally exercise-4) —
  this exercise is about testing technique, not rewriting the breaker. The only new
  file relative to that lineage is `circuit_breaker_test.go`.
- **The hammer test only asserts the end state is one of the three valid
  `CircuitState` values**, not an exact failure count or exact transition path —
  with the mutex in place the *values* are deterministic, but asserting a precise
  count purely to prove "no race" would be over-specifying; `-race` is the actual
  signal, the state check just confirms nothing got corrupted into an invalid value.
