package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// classify turns an Allow() result into a human-readable reason. This is the
// whole point of returning a sentinel error instead of a bool: the caller can
// tell "circuit open" apart from "half-open trial slots exhausted" and react
// (log, meter, back off) differently.
func classify(err error) string {
	switch {
	case err == nil:
		return "allowed"
	case errors.Is(err, ErrCircuitOpen):
		return "rejected: circuit open"
	case errors.Is(err, ErrHalfOpenLimitReached):
		return "rejected: half-open limit reached"
	default:
		return fmt.Sprintf("rejected: %v", err)
	}
}

func main() {
	cb := NewCircuitBreaker(context.Background())

	// Phase 1: closed breaker allows traffic. Five failures trip it open.
	fmt.Println("--- phase 1: closed ---")
	fmt.Println(classify(cb.Allow()))
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	// Phase 2: open breaker fails fast with ErrCircuitOpen.
	fmt.Println("--- phase 2: open, fail fast ---")
	fmt.Println(classify(cb.Allow()))

	// Phase 3: after the 5s cooldown the next Allow() flips to half-open. It
	// hands out 3 trial slots, then rejects with ErrHalfOpenLimitReached.
	fmt.Println("--- phase 3: half-open trial slots ---")
	time.Sleep(5 * time.Second)
	for i := 1; i <= 4; i++ {
		fmt.Printf("call %d: %s\n", i, classify(cb.Allow()))
	}
}
