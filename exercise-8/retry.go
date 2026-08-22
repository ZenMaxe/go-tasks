package main

import (
	"context"
	"math"
	"math/rand"
	"time"
)

const defaultSleepBetweenAttempts = 100 * time.Millisecond

type Retry interface {
	Retry(ctx context.Context, maxAttempts int, f func(ctx context.Context) error) error
}

func NewRetry(sleepBetweenAttempts time.Duration) Retry {
	if sleepBetweenAttempts <= 0 {
		sleepBetweenAttempts = defaultSleepBetweenAttempts
	}

	return &retry{
		sleepBetweenAttempts: sleepBetweenAttempts,
	}
}

type retry struct {
	sleepBetweenAttempts time.Duration
}

func (r *retry) Retry(ctx context.Context, maxAttempts int, f func(ctx context.Context) error) error {
	i := 0
	for {
		err := f(ctx)
		if err == nil {
			return nil
		}

		i++
		if i == maxAttempts {
			return err
		}

		backOffDelay := withJitter(computeBackoffDelay(i, r.sleepBetweenAttempts))
		select {
		case <-time.After(backOffDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// computeBackoffDelay is pure and deterministic: base * 2^(attempt-1).
func computeBackoffDelay(attempt int, base time.Duration) time.Duration {
	multiplier := math.Pow(2, float64(attempt-1))

	return time.Duration(multiplier) * base
}

// withJitter takes an already-computed delay and returns a randomized value in [0, duration].
func withJitter(duration time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(duration)))
}
