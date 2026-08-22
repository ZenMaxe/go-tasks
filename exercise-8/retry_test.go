package main

import (
	"testing"
	"time"
)

// Expected values are hand-calculated (100, 200, 400, 800ms), not re-derived
// via the same 2^n formula the code under test uses — otherwise the test
// would only prove self-consistency, not correctness.
func TestComputeBackOffDelay(t *testing.T) {
	tests := []struct {
		name      string
		attempt   int
		baseDelay time.Duration
		want      time.Duration
	}{
		{name: "attempt 1", attempt: 1, baseDelay: 100 * time.Millisecond, want: 100 * time.Millisecond},
		{name: "attempt 2", attempt: 2, baseDelay: 100 * time.Millisecond, want: 200 * time.Millisecond},
		{name: "attempt 3", attempt: 3, baseDelay: 100 * time.Millisecond, want: 400 * time.Millisecond},
		{name: "attempt 4", attempt: 4, baseDelay: 100 * time.Millisecond, want: 800 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeBackoffDelay(tt.attempt, tt.baseDelay)
			if got != tt.want {
				t.Errorf("computeBackoffDelay(%d, %v) = %v, want %v", tt.attempt, tt.baseDelay, got, tt.want)
			}
		})
	}
}

// withJitter's output is random, so this checks bounds across many
// iterations instead of exact equality — one lucky call could miss an
// off-by-range bug that a hundred calls won't.
func TestWithJitter(t *testing.T) {
	base := 400 * time.Millisecond
	for range 100 {
		got := withJitter(base)
		if got < 0 || got > base {
			t.Errorf("withJitter(%v) = %v, out of expected range [0, %v]", base, got, base)
		}
	}
}
