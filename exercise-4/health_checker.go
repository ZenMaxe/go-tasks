package main

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type HealthChecker interface {
	Run()
	IsHealthy() bool
	Changes() <-chan HealthStatus
}

type HealthStatus int

const (
	Healthy HealthStatus = iota
	Unhealthy
)

type healthChecker struct {
	isStarted    bool
	HealthStatus HealthStatus
	ticker       *time.Ticker
	changes      chan HealthStatus
	ctx          context.Context
	mu           sync.RWMutex
}

func NewHealthChecker(ctx context.Context) HealthChecker {
	return &healthChecker{
		HealthStatus: Healthy,
		isStarted:    false,
		changes:      make(chan HealthStatus, 1),
		ctx:          ctx,
	}
}

func (h *healthChecker) Run() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.isStarted {
		return
	}
	h.isStarted = true
	h.ticker = time.NewTicker(2 * time.Second)
	go h.check()

}

func (h *healthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.HealthStatus == Healthy
}

func (h *healthChecker) Changes() <-chan HealthStatus {
	return h.changes
}

func (h *healthChecker) check() {
	defer func() {
		h.ticker.Stop()
		close(h.changes)
	}()

	for {
		select {
		case <-h.ticker.C:
			if rand.Intn(10) < 3 {
				h.setUnHealthy()
				continue
			}
			h.setHealthy()
		case <-h.ctx.Done():
			return
		}
	}

}

func (h *healthChecker) setUnHealthy() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.HealthStatus == Unhealthy {
		return
	}
	h.HealthStatus = Unhealthy
	h.publish(h.HealthStatus)
}

func (h *healthChecker) setHealthy() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.HealthStatus == Healthy {
		return
	}
	h.HealthStatus = Healthy
	h.publish(h.HealthStatus)
}

func (h *healthChecker) publish(status HealthStatus) {
	select {
	case h.changes <- status:
		return
	default:
	}

	// Remove stale status.
	select {
	case <-h.changes:
	default:
	}

	// Store latest status.
	select {
	case h.changes <- status:
	default:
	}
}
