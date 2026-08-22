package main

import (
	"context"
	"sync"
	"testing"
)

// Hammers RecordFailure from many goroutines at once. With the mutex in
// place (as committed here) this passes clean under -race. Commenting out
// the Lock()/Unlock() pair in circuitBreaker.RecordFailure turns this into a
// reliable race detector: c.failures++ and the c.state read/write in
// moveToOpen become concurrent unsynchronized accesses, and `go test -race`
// reports a DATA RACE within the first run — see README.md "Part 3" for the
// captured output.
func TestCircuitBreaker_RecordFailure_ConcurrentSafe(t *testing.T) {
	cb := NewCircuitBreaker(context.Background()).(*circuitBreaker)

	const goroutines = 50
	const failuresEach = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < failuresEach; j++ {
				cb.RecordFailure()
			}
		}()
	}
	wg.Wait()

	// Just confirm the breaker landed in a valid, non-corrupted state —
	// the real assertion here is "go test -race reports nothing".
	state := cb.getState()
	if state != CircuitOpen && state != CircuitHalfOpen && state != CircuitClosed {
		t.Fatalf("circuit breaker ended in invalid state: %v", state)
	}
}
