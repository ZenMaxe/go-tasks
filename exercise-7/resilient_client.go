package main

import (
	"context"
	"errors"
	"sync"
)

const defaultMaxAttempts = 5

type ResilientClient interface {
	Start(ctx context.Context) error
	Call(ctx context.Context, fn func(ctx context.Context) error) error
}

type resilientClient struct {
	rateLimiter    rateLimiter
	circuitBreaker CircuitBreaker
	retrier        Retry
	maxAttempts    int

	mu        sync.Mutex
	isStarted bool
}

func NewResilientClient(bucket rateLimiter, ret Retry, breaker CircuitBreaker, maxAttempts int) ResilientClient {
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	return &resilientClient{
		rateLimiter:    bucket,
		circuitBreaker: breaker,
		retrier:        ret,
		maxAttempts:    maxAttempts,
	}
}

func (c *resilientClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isStarted {
		return errors.New("resilient client already started")
	}
	c.rateLimiter.Start(ctx)
	c.isStarted = true
	return nil
}

// Call is invoked once per logical request; Retry may invoke wrappedFn up to
// maxAttempts times for that single Call.
func (c *resilientClient) Call(ctx context.Context, fn func(ctx context.Context) error) error {
	c.mu.Lock()
	started := c.isStarted
	c.mu.Unlock()
	if !started {
		return errors.New("resilient client is not started")
	}

	// Per-attempt order: circuit breaker first, rate limiter second. A
	// rejected breaker check costs nothing and must not burn a rate-limit
	// token on a call that was never going to be attempted.
	wrappedFn := func(ctx context.Context) error {
		if !c.circuitBreaker.Allow() {
			return errors.New("circuit breaker rejected this attempt")
		}

		if err := c.rateLimiter.Allow(ctx); err != nil {
			return err
		}

		err := fn(ctx)
		if err != nil {
			c.circuitBreaker.RecordFailure()
			return err
		}
		c.circuitBreaker.RecordSuccess()
		return nil
	}

	return c.retrier.Retry(ctx, c.maxAttempts, wrappedFn)
}
