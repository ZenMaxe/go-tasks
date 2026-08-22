package main

import (
	"context"
	"testing"
	"time"
)

// The "context canceled" case proves Allow returns promptly when cancelled
// while it is actively blocked waiting for a token — not just when handed an
// already-cancelled context up front. ready/close synchronizes the test with
// the goroutine so cancel() only fires once Allow is confirmed to be blocking.
// close() is safe here (unlike a value send) because ctx cancellation is a
// one-time broadcast: a closed channel is a permanent state any receiver
// observes correctly regardless of exact timing.
func TestAllow(t *testing.T) {
	tests := []struct {
		name          string
		maxTokens     int
		cancelContext bool
		expectErr     bool
	}{
		{
			name:          "token available",
			maxTokens:     1,
			cancelContext: false,
			expectErr:     false,
		},
		{
			name:          "context canceled while blocked",
			maxTokens:     0,
			cancelContext: true,
			expectErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			bucket := NewTokenBucket(tt.maxTokens, 1, time.Second)

			bucket.Start(ctx)

			ready := make(chan struct{})
			errChan := make(chan error, 1)
			go func() {
				close(ready)
				errChan <- bucket.Allow(ctx)
			}()

			<-ready
			if tt.cancelContext {
				cancel()
			}

			select {
			case err := <-errChan:
				if tt.expectErr {
					if err == nil {
						t.Errorf("expected error, got <nil>")
					}
				} else {
					if err != nil {
						t.Errorf("expected nil error, got %v", err)
					}
				}
			case <-time.After(1 * time.Second):
				t.Fatal("Allow did not return after cancellation")
			}
		})
	}
}
