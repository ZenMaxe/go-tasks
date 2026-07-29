# Exercise 5 — Context Propagation Through Multiple Layers

Standalone Go module. Run it on its own, independent of any other exercise in this
repo:

```bash
cd exercise-5
go run .
```

## Spec

Build a small chain of function calls, at least 3 layers deep, where cancelling a
`context.Context` at the top layer promptly stops work happening in the deepest
layer — without any layer needing a separate shutdown channel, just `ctx`.

Suggested shape:

```
main()
  → handleRequest(ctx)          // layer 1
      → fetchUserData(ctx)      // layer 2 — simulates a slow "DB call"
          → fetchFromReplica(ctx)  // layer 3 — simulates an even slower "network call"
```

- `fetchFromReplica` simulates real work (3s) in a way that respects cancellation —
  not a plain unconditional sleep.
- `main()` creates a context with a timeout shorter than the simulated work (1s),
  then calls `handleRequest(ctx)`.
- Every layer passes `ctx` through; the deepest layer is where
  cancellation-awareness matters most.
- Timestamps at each layer's entry/exit confirm the chain stops at ~1s, not the
  full 3s.
- Once cancelled, each layer returns promptly with an error indicating why it
  stopped (`ctx.Err()`).

Bonus: a 4th layer that spawns its own goroutine internally (two concurrent
"sub-fetches"), checking that cancellation still propagates into
concurrently-spawned children.

### Key design question

`time.Sleep(d)` has no way to be interrupted — the .NET analogue is the difference
between `Thread.Sleep(d)` (uninterruptible) and `await Task.Delay(d,
cancellationToken)` (interruptible). Go has no `Task.Delay` equivalent built into
`time.Sleep`, so what stands in for it?

**Answer used here:** `sleepCtx` (`main.go`) races a `time.Timer` against
`ctx.Done()` in a `select` — whichever fires first wins. This is the "sleep, but
stop early if the context says stop" primitive every layer's work is built on. A
`context.Context` is the .NET `CancellationToken` equivalent: a read-only signal
threaded through every call, not a shutdown channel any layer owns or closes.

## Requirement → code map

| # | Requirement | Where |
|---|---|---|
| 1 | 3-layer call chain | `handleRequest` → `fetchUserData` → `fetchFromReplica` |
| 2 | Deepest layer does cancellation-aware "work" | `fetchFromReplica` calls `sleepCtx(ctx, 3*time.Second)` instead of `time.Sleep` |
| 3 | Cancellation-aware sleep primitive | `sleepCtx` — `select` on `timer.C` vs `ctx.Done()` |
| 4 | Shorter timeout than the work | `run` — `context.WithTimeout(context.Background(), 1*time.Second)` |
| 5 | Every layer threads `ctx` through, no shutdown channel | all of `handleRequest`/`fetchUserData`/`fetchFromReplica` take `ctx context.Context` as their only cancellation input |
| 6 | Entry/exit timestamps per layer | `log` — prints `time.Since(start)` on every call, `defer log(...)` for exits |
| 7 | Prompt return with `ctx.Err()`-derived error on cancellation | each layer wraps and returns the error immediately; no layer swallows or retries it |
| 8 (bonus) | 4th layer, concurrent sub-fetches, cancellation reaches goroutine children | `fetchFromReplicaConcurrent` + `subFetch`, `sync.WaitGroup` |

## Notable implementation details

- **The naive/broken version was written and discarded, not committed.** Swapping
  `sleepCtx(ctx, d)` for a plain `time.Sleep(d)` in `fetchFromReplica` reproduces the
  trap from the spec: the deepest layer blocks on something with no `ctx.Done()`
  case, so the 1s timeout fires but the call chain keeps running the full 3s before
  anything notices. The fix is exactly the `select` in `sleepCtx` — cancellation has
  to be one of the things being waited on, not something checked before or after a
  blocking call.
- **`time.NewTimer` + `defer timer.Stop()`, not `time.After`.** `time.After`
  leaks its underlying timer until it fires, even if the `select` picks the other
  case — negligible for one call, but the habit matters since this exact pattern
  (`sleepCtx`) gets reused per sub-fetch call in the bonus layer.
- **Errors are wrapped at every layer** (`fmt.Errorf("layer: %w", err)`), so the
  final printed error shows the full call path down to `context deadline exceeded`
  — useful for confirming *which* layer's wait actually got interrupted, not just
  that something did.
- **Bonus layer's cancellation reaches goroutine children for free.** `ctx` is an
  ordinary value passed into each `go func(...)` closure — nothing special is
  needed to make a spawned goroutine cancellation-aware beyond giving it the same
  `ctx` and having it call `sleepCtx` like any other layer. The run output shows
  both `subFetch(replica-a)` and `subFetch(replica-b)` exiting at the ~1s mark
  alongside their parent, not at 2.5s/3s.
- **`run()` resets the package-level `start` per demo** so both the 3-layer and
  4-layer runs report elapsed time from their own `context.WithTimeout` call
  rather than from program start.
