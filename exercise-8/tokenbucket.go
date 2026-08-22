package main

import (
	"context"
	"time"
)

const (
	MaxTokenBucket       int = 50
	TokenGenerationCount int = 25
	RefillInterval           = time.Second
	JobCount             int = 100
)

type rateLimiter interface {
	Allow(ctx context.Context) error
	Start(ctx context.Context)
}

type TokenBucket struct {
	tokens            chan struct{}
	maxTokens         int
	tokenIncreaseRate int
	refillInterval    time.Duration
}

func NewTokenBucket(maxTokens, tokenIncreaseRate int, refillInterval time.Duration) *TokenBucket {
	return &TokenBucket{
		maxTokens:         maxTokens,
		tokenIncreaseRate: tokenIncreaseRate,
		refillInterval:    refillInterval,
		tokens:            make(chan struct{}, maxTokens),
	}
}

func (t *TokenBucket) Allow(ctx context.Context) error {
	select {
	case <-t.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *TokenBucket) Start(ctx context.Context) {
	for i := 0; i < t.maxTokens; i++ {
		t.tokens <- struct{}{}
	}

	go func() {
		timer := time.NewTimer(t.refillInterval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			refillLoop:
				for i := 0; i < t.tokenIncreaseRate; i++ {
					select {
					case t.tokens <- struct{}{}:
					default:
						break refillLoop
					}
				}
				timer.Reset(t.refillInterval)
			}
		}
	}()
}
