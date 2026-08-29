package main

import (
	"context"
	"sync"
	"time"
)

type CircuitBreaker interface {
	Allow() error
	RecordSuccess()
	RecordFailure()
}

type CircuitState int

const (
	CircuitClosed   CircuitState = iota
	CircuitHalfOpen CircuitState = iota
	CircuitOpen     CircuitState = iota
)

func NewCircuitBreaker(ctx context.Context) CircuitBreaker {
	return &circuitBreaker{
		state: CircuitClosed,
		ctx:   ctx,
	}
}

type circuitBreaker struct {
	state CircuitState

	failures          int
	halfOpenSuccesses int
	halfOpenCompleted int
	halfOpenAttempts  int
	openedAt          time.Time

	ctx context.Context
	mu  sync.Mutex
}

func (c *circuitBreaker) RecordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case CircuitClosed:
		c.failures++
		if c.failures >= 5 {
			c.moveToOpen()
		}

	case CircuitHalfOpen:
		c.halfOpenCompleted++
		c.evaluateHalfOpen()

	case CircuitOpen:
		return
	}
}

func (c *circuitBreaker) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case CircuitClosed:
		c.failures = 0

	case CircuitHalfOpen:
		c.halfOpenCompleted++
		c.halfOpenSuccesses++
		c.evaluateHalfOpen()

	case CircuitOpen:
		return
	}
}

func (c *circuitBreaker) Allow() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == CircuitOpen {
		if time.Since(c.openedAt) < 5*time.Second {
			return ErrCircuitOpen
		}
		c.state = CircuitHalfOpen
		c.resetHalfOpen()
	}

	switch c.state {
	case CircuitClosed:
		return nil

	case CircuitHalfOpen:
		reserveResult := c.tryReserveOperation()
		if !reserveResult {
			return ErrHalfOpenLimitReached
		}
		return nil

	case CircuitOpen:
		return ErrCircuitOpen

	default:
		return ErrUnknown

	}
}

func (c *circuitBreaker) tryReserveOperation() bool {
	if c.halfOpenAttempts >= 3 {
		return false
	}
	c.halfOpenAttempts++
	return true
}

func (c *circuitBreaker) moveToClosed() {
	c.state = CircuitClosed
	c.failures = 0
	c.openedAt = time.Time{}
	c.resetHalfOpen()
}

func (c *circuitBreaker) moveToOpen() {
	c.state = CircuitOpen
	c.openedAt = time.Now()
	c.resetHalfOpen()
}

func (c *circuitBreaker) resetHalfOpen() {
	c.halfOpenSuccesses = 0
	c.halfOpenCompleted = 0
	c.halfOpenAttempts = 0
}

func (c *circuitBreaker) evaluateHalfOpen() {
	if c.halfOpenCompleted < 3 {
		return
	}

	if c.halfOpenSuccesses >= 2 {
		c.moveToClosed()
		return
	}

	c.moveToOpen()
}
func (c *circuitBreaker) getState() CircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}
