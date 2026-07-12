# go-tasks

Go learning exercises. Author is a .NET developer picking up Go — favor idiomatic Go
over clever Go, and prefer explanations that map to .NET concepts (`CancellationToken`
≈ `context.Context`, `BlockingCollection` ≈ channel + `select`, etc.) when introducing
something new.

## Adding a new exercise

Each exercise is a fully independent Go module — no shared `main`, no shared `go.mod`.
This lets `go run .` work from inside any single exercise folder without touching the
others, and lets exercises depend on different things without conflict.

1. Create `exercise-N/` at repo root.
2. `cd exercise-N && go mod init github.com/ZenMaxe/go-tasks/exercise-N`
3. `package main` with a `main.go` entrypoint.
4. Write `exercise-N/README.md` with these sections (see `exercise-1/README.md` for
   the reference shape):
   - **Spec** — the exercise requirements as given, verbatim or lightly cleaned up.
   - **Key design question** — the one thing worth thinking through before coding
     (e.g. "who owns this state under concurrency?"), plus the answer actually used.
   - **Requirement → code map** — a table pointing each spec requirement at the
     file/function that implements it. This is the part that pays off later when
     revisiting old exercises.
   - **Notable implementation details** — anything non-obvious: a bug that was fixed
     along the way, a deliberate deviation from the literal spec, a pattern worth
     remembering.
5. Add one row to the root `README.md` table.

## Verifying an exercise before committing

```bash
cd exercise-N
go build ./... && go vet ./... && gofmt -l .
```
`gofmt -l .` should print nothing. If the exercise has a shutdown/cancellation path,
actually exercise it — don't just typecheck:

```bash
go build -o /tmp/exN .
/tmp/exN & PID=$!; sleep 2; kill -INT $PID; wait $PID; echo "exit: $?"
```
Confirm it exits cleanly (code 0, no hang) and prints a sane summary.

**Never commit a compiled binary.** `go build ./...` without `-o` drops an executable
named after the package into the current directory — check `git status` before
`git add`. `.gitignore` already excludes `exercise-*/exercise-*`, but double check
new exercise dirs match that pattern (name the dir and the binary the same way).

## Concurrency conventions learned the hard way (exercise-1)

These came from real bugs hit while building exercise-1 — worth applying to any
future exercise involving goroutines/channels:

- **Guard every blocking channel send that outlives its reader.** A goroutine that
  can still be sending after the thing reading from it has stopped will hang
  forever. Pattern: `select { case ch <- v: case <-ctx.Done(): /* handle */ }`.
  The `/* handle */` matters — see next point.
- **Never let a cancelled send silently drop work.** If a `ctx.Done()` branch means
  "this job can't go where it was headed," it still needs to produce *something*
  (a `Cancelled` result, a log line) — not just `return`. A silently dropped job
  breaks completion-counting and produces an inaccurate summary.
- **Prefer "last writer closes the channel" over "listener quits on ctx.Done()"**
  when you need to guarantee every in-flight result is drained before shutdown.
  Track writers with a `sync.WaitGroup`, close the channel once `Wait()` returns,
  and let the reader just `range` over it. This avoids the listener racing a
  cancellation signal and dropping results that were already in flight.
- **`context.Context` is a read-only signal, not a data channel.** Every goroutine
  may read `ctx.Done()`; nothing should try to pass data through it.
- **A zero-value enum should not read as a valid success state.** If a status type
  has a natural "unset" meaning, give it the zero value explicitly (`NotDefined
  StatusType = iota`), so an accidentally-zeroed struct doesn't silently look like
  a success.
