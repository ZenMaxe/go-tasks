# go-tasks

Go exercises, each self-contained and independently runnable — no shared `main`, no shared module. Every exercise has its own `go.mod`, so `go run` works from inside its own folder without touching the others.

| # | Exercise | Description |
|---|---|---|
| 1 | [`exercise-1/`](exercise-1/) — Distributed Rate-Limited Job Scheduler | Priority-queue job scheduler (`high`/`medium`/`low`) with starvation prevention, a global 20 jobs/sec rate limit, bounded retries, and graceful shutdown that distinguishes in-flight work from queued-but-unstarted work. |
| 2 | [`exercise-2/`](exercise-2/) — Worker Pool with Graceful Shutdown | Fixed pool of worker goroutines draining a single jobs channel, shut down purely by closing the channel (no `context`/`shutdown chan`), `sync.WaitGroup` to confirm every worker exited, final processed-count printed once `wg.Wait()` returns. |
| 3 | [`exercise-3/`](exercise-3/) — Token Bucket Rate Limiter | Buffered-channel token bucket, `Allow(ctx) error` blocks-then-consumes or returns `ctx.Err()`, refill loop uses a manually reset `time.Timer` (no `time.Ticker`), clean goroutine exit on `ctx` cancellation, no busy-waiting. |
| 4 | [`exercise-4/`](exercise-4/) — Circuit Breaker with Health Checker | `Closed`/`Open`/`HalfOpen` circuit breaker (5 failures to open, 5s cooldown, 3-trial half-open), mutex-guarded shared state, wired to a background `HealthChecker` whose polled status changes override the failure-count state machine. |
| 5 | [`exercise-5/`](exercise-5/) — Context Propagation Through Multiple Layers | 3-layer call chain (`handleRequest` → `fetchUserData` → `fetchFromReplica`) where a `context.WithTimeout` set at the top promptly cancels a simulated 3s "network call" at the deepest layer via a `select`-based cancellable sleep, no shutdown channel involved; bonus 4th layer fans out two concurrent sub-fetches and confirms cancellation reaches goroutine children too. |
| 6 | [`exercise-6/`](exercise-6/) — Bounded Retry with Exponential Backoff + Jitter | `Retry(ctx, maxAttempts, f)` wraps an unreliable call, doubling backoff with full jitter (`rand.Int63n` over the full computed delay) to avoid a thundering herd, `select`-based ctx-aware wait that bails immediately on cancellation instead of sleeping out the full backoff, exact attempt count (no off-by-one), last underlying error returned on final failure, base delay configurable via constructor. |

## Running an exercise

```bash
cd exercise-<n>
go run .
```
