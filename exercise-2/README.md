# Exercise 2 — Worker Pool with Graceful Shutdown

Standalone Go module. Run it on its own, independent of any other exercise in this repo:

```bash
cd exercise-2
go run .
```

## Spec

Build a small system that:

- Has a pool of N worker goroutines (e.g. 4-5)
- A single jobs channel that a producer sends work items into
- Each worker picks up jobs from that channel and "processes" them
  (simulate with a short `time.Sleep` + printing the job ID and which
  worker handled it)
- A producer that sends a batch of jobs (e.g. 30-500), then stops
- Once the producer is done sending, the system should shut down
  cleanly:
  - No workers should be left blocked waiting forever
  - `main` should not return until all workers have actually
    finished, not just until jobs stopped being sent
  - Print a final count of how many jobs were processed, once
    everything is confirmed done

### Constraints

- No `shutdown chan struct{}` or `context.Context` — signal "no more
  work" purely via closing the jobs channel, workers exit via
  `for range`
- `sync.WaitGroup` tracks when all workers have exited
- Only the producer calls `close()` on the jobs channel — never a
  receiver

### Key design question

Does it matter whether the jobs channel is closed before or after `wg.Wait()`?

**Answer used here:** close first, then `wg.Wait()`. `wg.Wait()` blocks until
every worker's `for range jobs` loop drains and exits, and that loop only
exits once the channel is both closed *and* empty. Calling `wg.Wait()`
before `close(jobs)` would deadlock — workers still ranging over an open
channel with no more sends coming, waiting forever for either a value or
a close signal that never arrives.

## What each requirement maps to in the code

| # | Concept | Where |
|---|---|---|
| 1 | Worker pool | `main` — spawns `workerCount` goroutines running `worker` |
| 2 | Job processing + logging | `worker` — `time.Sleep` + `fmt.Printf` with worker ID and job ID |
| 3 | Producer | `main`'s send loop — pushes `jobCount` ints into `jobs` |
| 4 | Clean shutdown signal | `close(jobs)` called by the producer (sole sender) right after the send loop finishes |
| 5 | Workers exit without blocking forever | `worker` — `for job := range jobs` exits once the channel is closed and drained |
| 6 | Wait for actual completion | `sync.WaitGroup` — `wg.Add(1)` per worker, `wg.Done()` deferred, `wg.Wait()` in `main` before printing the summary |
| 7 | Final processed count | `atomic.Int64` incremented by each worker after processing a job, printed after `wg.Wait()` returns |

## Notable implementation details

- **Ordering matters**: `close(jobs)` happens before `wg.Wait()`. Closing
  doesn't discard anything already in the channel — a `range` still drains
  every value sent before the close; only once empty does it exit. Closing
  after `wg.Wait()` would be a permanent deadlock, since the workers would
  never see a reason to stop ranging.
- **Single-owner close**: the producer (sender) is the only goroutine that
  calls `close()`. Never a receiver — closing a channel that's still being
  read elsewhere, or closing it twice, panics (`close of closed channel`).
- **Counting without a shared map/slice**: `atomic.Int64` avoids a mutex for
  the simple sum-of-processed-jobs case, since workers only ever increment,
  never read each other's contributions mid-flight.
