# Exercise 3 — Token Bucket Rate Limiter

Standalone Go module. Run it on its own, independent of any other exercise in this repo:

```bash
cd exercise-3
go run .
```

## Spec

Implement a token bucket rate limiter in Go that controls how many concurrent
operations are allowed to proceed over time.

Build a `TokenBucket` type that:

- Holds a fixed maximum number of tokens (`maxTokens`).
- Starts fully loaded with `maxTokens` tokens available.
- Refills tokens at a fixed rate (`tokenIncreaseRate` tokens added per
  interval), without exceeding `maxTokens`.
- Exposes an `Allow(ctx context.Context) error` method that:
  - Blocks the caller until a token is available, then consumes one token
    and returns `nil`.
  - Returns `ctx.Err()` immediately if the context is cancelled or times
    out while waiting.
- Exposes a `Start(ctx context.Context)` method that begins the
  token-refill process and stops cleanly when `ctx` is cancelled (no
  leaked goroutines, no leaked resources).

### Constraints

- No `time.Ticker` for the refill mechanism — refill via a manually
  managed timer loop instead.
- Safe for concurrent use — multiple goroutines call `Allow` at once.
- No token generation once `ctx` is cancelled; every internally-started
  goroutine exits when `ctx` is done.
- No busy-waiting / no CPU spinning while waiting for tokens or ticks.

### Key design question

How do you get periodic refill without `time.Ticker` and without
busy-waiting?

**Answer used here:** a single `time.Timer`, reset at the end of every
refill cycle (`timer.Reset(refillInterval)` inside the loop) instead of a
free-running ticker. The refill goroutine blocks on `select { case
<-timer.C: ...; case <-ctx.Done(): return }` — it's parked on a channel
receive the whole time, so there's zero CPU spend between refills, and the
`ctx.Done()` branch means it can exit mid-wait instead of leaking.

## What each requirement maps to in the code

| # | Concept | Where |
|---|---|---|
| 1 | Fixed capacity, starts full | `NewTokenBucket` — buffered channel of size `maxTokens`, pre-filled in `Start` |
| 2 | Bounded refill (never exceeds max) | `Start`'s `refillLoop` — a non-blocking `select`/`default` send that stops as soon as the buffered channel is full |
| 3 | Refill without `time.Ticker` | `Start` — `time.NewTimer` + `timer.Reset(...)` re-armed manually each cycle, instead of a `time.Ticker` |
| 4 | `Allow` blocks then consumes | `Allow` — `select` receiving from the token channel |
| 5 | `Allow` respects cancellation | `Allow` — same `select`, `case <-ctx.Done(): return ctx.Err()` |
| 6 | Concurrency safety | The buffered channel itself is the only shared state — Go channels are safe for concurrent send/receive, so no separate mutex is needed |
| 7 | Clean shutdown, no leaks | `Start`'s goroutine — `ctx.Done()` case returns, and `defer timer.Stop()` releases the timer; nothing runs after cancellation |
| 8 | No busy-waiting | Both `Allow` and the refill loop block on channel receives (`<-t.tokens`, `<-timer.C`, `<-ctx.Done()`) — no polling loop anywhere |

## Notable implementation details

- **Channel-as-bucket**: tokens are represented by empty-struct values in a
  buffered channel rather than an integer counter + mutex. A send only
  succeeds while the channel has spare capacity, so "never exceed
  `maxTokens`" falls out of the channel's own buffer size for free.
- **Timer reset vs. ticker**: `time.NewTimer` + manual `Reset` was chosen
  over `time.AfterFunc`/repeated `time.Sleep` in a loop because it keeps a
  single timer object alive (one deferred `Stop()`), and reads cleanly as
  "wait, then re-arm" without spawning a new goroutine or timer per cycle.
- **`Allow` returns `ctx.Err()`, not a generic error**: propagates the
  concrete reason (`context.Canceled` vs. `context.DeadlineExceeded`)
  instead of masking it, so callers can distinguish shutdown from timeout.
