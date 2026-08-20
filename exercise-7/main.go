package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// unreliableOp fails deterministically for its first `failUntil` calls (enough
// to trip the breaker's 5-consecutive-failures threshold), then recovers and
// succeeds forever after.
func unreliableOp(failUntil int32) (func(ctx context.Context) error, *int32) {
	var calls int32
	return func(ctx context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n <= failUntil {
			return fmt.Errorf("downstream failure #%d", n)
		}
		return nil
	}, &calls
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rateLimit := NewTokenBucket(5, 5, 200*time.Millisecond)
	retries := NewRetry(50 * time.Millisecond)
	breaker := NewCircuitBreaker(ctx)
	client := NewResilientClient(rateLimit, retries, breaker, 3)

	if err := client.Start(ctx); err != nil {
		fmt.Println("start:", err)
		return
	}

	fn, calls := unreliableOp(5) // fails the first 5 real attempts, then recovers

	// Phase 1: enough calls to drive the breaker Closed -> Open. Each Call
	// retries up to 3 times internally, so a handful of Call()s is enough to
	// burn through the 5-failure threshold.
	fmt.Println("--- phase 1: downstream failing ---")
	for i := 1; i <= 4; i++ {
		err := client.Call(ctx, fn)
		fmt.Printf("call %d: err=%v (real attempts so far=%d)\n", i, err, atomic.LoadInt32(calls))
	}

	// Phase 2: breaker should now be open — calls fail fast, no real attempt.
	fmt.Println("--- phase 2: breaker open, calls should fail fast ---")
	before := atomic.LoadInt32(calls)
	err := client.Call(ctx, fn)
	after := atomic.LoadInt32(calls)
	fmt.Printf("call 5: err=%v, real attempts made=%d (expected 0)\n", err, after-before)
	if after != before {
		fmt.Println("UNEXPECTED: breaker let a real attempt through while open")
	}

	// Phase 3: wait out the breaker's cooldown, then retry — downstream has
	// since "recovered" so the half-open trial calls should succeed and
	// close the breaker again.
	fmt.Println("--- phase 3: waiting for breaker cooldown, then retrying recovered downstream ---")
	select {
	case <-time.After(5500 * time.Millisecond):
	case <-ctx.Done():
	}

	var errs error
	for i := 1; i <= 3; i++ {
		err := client.Call(ctx, fn)
		fmt.Printf("recovery call %d: err=%v\n", i, err)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}
	if errs == nil {
		fmt.Println("breaker recovered: all half-open trial calls succeeded")
	} else {
		fmt.Println("breaker did not fully recover:", errs)
	}
}
