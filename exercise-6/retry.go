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

		multiplier := math.Pow(2, float64(i-1))
		jittered := time.Duration(rand.Int63n(int64(r.sleepBetweenAttempts * time.Duration(multiplier))))
		select {
		case <-time.After(jittered):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
