package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var start = time.Now()

func elapsed() time.Duration {
	return time.Since(start).Round(time.Millisecond)
}

func log(layer string, format string, args ...any) {
	fmt.Printf("[%6s] %-6s %s\n", elapsed(), layer, fmt.Sprintf(format, args...))
}

// sleepCtx is a cancellation-aware sleep: it blocks for d, but returns early
// with ctx.Err() if ctx is cancelled first. This is the ctx analogue of
// `await Task.Delay(d, cancellationToken)` in .NET — plain time.Sleep(d) has
// no ctx equivalent and can't be interrupted.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Layer 3 (deepest, non-bonus chain): simulates a slow network call to a DB
// replica. This is the layer where cancellation-awareness matters most,
// since it holds the longest sleep.
func fetchFromReplica(ctx context.Context) (string, error) {
	log("L3", "fetchFromReplica: enter")
	defer log("L3", "fetchFromReplica: exit")

	if err := sleepCtx(ctx, 3*time.Second); err != nil {
		return "", fmt.Errorf("fetchFromReplica: %w", err)
	}
	return "replica-data", nil
}

// Layer 2: simulates a DB call that itself depends on a replica fetch.
func fetchUserData(ctx context.Context) (string, error) {
	log("L2", "fetchUserData: enter")
	defer log("L2", "fetchUserData: exit")

	data, err := fetchFromReplica(ctx)
	if err != nil {
		return "", fmt.Errorf("fetchUserData: %w", err)
	}
	return data, nil
}

// Layer 1: top of the request-handling chain.
func handleRequest(ctx context.Context) (string, error) {
	log("L1", "handleRequest: enter")
	defer log("L1", "handleRequest: exit")

	data, err := fetchUserData(ctx)
	if err != nil {
		return "", fmt.Errorf("handleRequest: %w", err)
	}
	return data, nil
}

// --- Bonus: 4th layer, concurrent sub-fetches ---

// subFetch simulates one of two concurrent "sub-fetches" a layer-4 call
// fans out to. Each is cancellation-aware on its own — a goroutine ctx
// spawns still sees the same Done() channel, since ctx is just a value
// passed down, not tied to the calling goroutine.
func subFetch(ctx context.Context, name string, d time.Duration) (string, error) {
	log("L4", "subFetch(%s): enter", name)
	defer log("L4", "subFetch(%s): exit", name)

	if err := sleepCtx(ctx, d); err != nil {
		return "", fmt.Errorf("subFetch(%s): %w", name, err)
	}
	return name + "-data", nil
}

// fetchFromReplicaConcurrent is layer 4: it spawns two sub-fetches in their
// own goroutines and waits for both. Cancelling ctx must reach both
// children even though neither was started by the caller directly.
func fetchFromReplicaConcurrent(ctx context.Context) (string, error) {
	log("L4", "fetchFromReplicaConcurrent: enter")
	defer log("L4", "fetchFromReplicaConcurrent: exit")

	var wg sync.WaitGroup
	results := make([]string, 2)
	errs := make([]error, 2)

	jobs := []struct {
		name string
		d    time.Duration
	}{
		{"replica-a", 3 * time.Second},
		{"replica-b", 2500 * time.Millisecond},
	}

	for i, job := range jobs {
		wg.Add(1)
		go func(i int, name string, d time.Duration) {
			defer wg.Done()
			res, err := subFetch(ctx, name, d)
			results[i] = res
			errs[i] = err
		}(i, job.name, job.d)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return "", fmt.Errorf("fetchFromReplicaConcurrent: %w", err)
		}
	}
	return results[0] + "+" + results[1], nil
}

func run(label string, work func(ctx context.Context) (string, error)) {
	fmt.Printf("\n=== %s ===\n", label)
	start = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	data, err := work(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log("main", "chain stopped early as expected: %v", err)
		} else {
			log("main", "chain failed: %v", err)
		}
		return
	}
	log("main", "chain completed: %q", data)
}

func main() {
	// 3-layer chain: handleRequest -> fetchUserData -> fetchFromReplica.
	// ctx times out at 1s; fetchFromReplica sleeps for 3s. Expect the whole
	// chain to unwind at ~1s, not run the full 3.
	run("3-layer chain (ctx timeout 1s, work 3s)", handleRequest)

	// Bonus 4th layer: fetchFromReplicaConcurrent spawns two goroutines.
	// Same 1s timeout should cancel both children promptly.
	run("bonus: 4th layer with concurrent sub-fetches", fetchFromReplicaConcurrent)
}
