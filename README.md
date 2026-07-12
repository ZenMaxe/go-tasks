# go-tasks

Go exercises, each self-contained and independently runnable — no shared `main`, no shared module. Every exercise has its own `go.mod`, so `go run` works from inside its own folder without touching the others.

| # | Exercise | Description |
|---|---|---|
| 1 | [`exercise-1/`](exercise-1/) — Distributed Rate-Limited Job Scheduler | Priority-queue job scheduler (`high`/`medium`/`low`) with starvation prevention, a global 20 jobs/sec rate limit, bounded retries, and graceful shutdown that distinguishes in-flight work from queued-but-unstarted work. |

## Running an exercise

```bash
cd exercise-<n>
go run .
```
