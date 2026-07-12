# Exercise 1 — Distributed Rate-Limited Job Scheduler

Standalone Go module. Run it on its own, independent of any other exercise in this repo:

```bash
cd exercise-1
go run .
```

Shutdown can be triggered two ways:
- **Ctrl+C** (or `SIGTERM`) at any point while it's running — in-flight jobs finish, queued-but-unstarted jobs are cancelled and reported.
- **Naturally**, once every produced job has a final result.

## Spec

Build a job scheduler that:

- Processes tasks from three priority queues: `high`, `medium`, `low`
- Always prefers `high` over `medium`, and `medium` over `low` — but must never completely starve `medium` or `low`
- Respects a global rate limit: max 20 jobs processed per second, system-wide (using `time.Ticker` or a token bucket)
- Simulates random job failures (~30% failure rate)
- Retries failed jobs, incrementing an `Attempts` counter, putting them back onto a dedicated retry channel that the dispatcher drains ahead of fresh work
- After `Attempts == 2` (2 failures), marks the job as `permanently_failed`
- Handles a shutdown signal that can arrive at any time:
  - Stop accepting new work
  - Let in-flight jobs finish
  - Cancel all queued-but-unstarted jobs
  - Print a clean final summary regardless of when shutdown occurs

### Key design question

Before coding: who owns the job state at each stage of the system?

**Answer used here:** the `Dispatcher` is the sole owner of the queues and the streak counters (`highStreak`/`mediumStreak`) — nothing else touches them, so there's no data race to guard with a mutex. `Worker`s own nothing but the job in their hand; once they hand a result back, that job is gone from their world. `context.Context` is the one piece of state every goroutine is allowed to read (never write) — it's the shutdown signal, not a data channel.

## What each requirement maps to in the code

| # | Concept | Where |
|---|---|---|
| 1 | Priority-ordered scheduling | `Dispatcher.Run` — non-blocking checks in `high → medium → low` order, falling back to a blocking `select` when nothing is ready (Go's `select` has no built-in priority, so this has to be explicit) |
| 2 | Starvation prevention | `highStreak` / `mediumStreak` counters + `streakLimit` — after N high-priority jobs in a row, high is skipped for one round so medium/low get a turn |
| 3 | Rate limiting | `time.Ticker` (50ms tick = 20 jobs/sec) gates every send to a worker |
| 4 | Concurrency limiting | `workerCount` — a fixed pool of workers reading off one `workerChan`, independent of the rate limiter |
| 5 | Backpressure | Emerges naturally: unbuffered `high`/`medium`/`low` channels mean the dispatcher (and by extension the producer) can't outrun the workers |
| 6 | Bounded retry | `Worker.handlePickedJob` — failed jobs go to a dedicated `retry` channel (checked first by the dispatcher) until `Attempts == maxAttempts`, then `PermanentlyFailed` |
| 7 | State ownership | Dispatcher owns queues/streaks exclusively; workers own nothing shared; `context.Context` is read-only shutdown state |
| 8 | Graceful shutdown | `context.Context` cancelled via `signal.NotifyContext` (external) or once all results are in (natural). Every code path that can abandon a job — dispatcher can't hand off in time, worker can't retry in time — reports a `Cancelled` result instead of silently dropping it |

## Notable implementation details

- **Zero-value safety**: `JobStatus`'s zero value is `NotDefined`, not `Success` — an unset `JobResult{}` can never be silently misread as a success.
- **Drain-before-close on shutdown**: `jobResults` is closed only after every writer (all workers + the dispatcher) has returned, tracked via a `sync.WaitGroup`. The summary loop just `range`s over the channel until it closes — it never races a cancellation signal and drops a result that was already in flight.
- **No silent job loss**: every `ctx.Done()` escape branch (a worker unable to retry, a dispatcher unable to hand off) still emits exactly one `Cancelled` `JobResult` for the abandoned job, so `Cancelled` in the final summary is always accurate.
