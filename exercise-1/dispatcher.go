package main

import (
	"context"
	"time"
)

type Dispatcher struct {
	high       chan Job
	medium     chan Job
	low        chan Job
	retry      chan Job
	result     chan JobResult
	workerChan chan Job
	ctx        context.Context

	streakLimit  int
	highStreak   int
	mediumStreak int

	ticker *time.Ticker
}

func NewDispatcher(high, medium, low chan Job, retry chan Job, result chan JobResult,
	ctx context.Context, streakLimit int) *Dispatcher {
	return &Dispatcher{
		high:        high,
		medium:      medium,
		low:         low,
		retry:       retry,
		result:      result,
		workerChan:  make(chan Job),
		ctx:         ctx,
		streakLimit: streakLimit,
		ticker:      time.NewTicker(50 * time.Millisecond),
	}
}

func (d *Dispatcher) Run() {
	for {
		var job Job
		var ok bool

		select {
		case job = <-d.retry:
			ok = true
		default:
		}

		if !ok && d.highStreak < d.streakLimit {
			select {
			case job = <-d.high:
				ok = true
			default:
			}
		}

		if !ok && d.mediumStreak < d.streakLimit {
			select {
			case job = <-d.medium:
				ok = true
			default:
			}
		}

		if !ok {
			select {
			case job = <-d.low:
				ok = true
			default:
			}
		}

		if !ok {
			select {
			case job = <-d.retry:
			case job = <-d.high:
			case job = <-d.medium:
			case job = <-d.low:
			case <-d.ctx.Done():
				close(d.workerChan)
				return
			}
		}

		d.calculateJobStreak(job)

		select {
		case <-d.ticker.C:
			d.workerChan <- job
		case <-d.ctx.Done():
			select {
			case d.workerChan <- job:
			default:
				d.result <- JobResult{
					JobId:    job.Id,
					Status:   Cancelled,
					Attempts: job.Attempts,
				}
			}
			close(d.workerChan)
			return
		}
	}
}

func (d *Dispatcher) calculateJobStreak(job Job) {
	switch job.Priority {
	case High:
		d.highStreak++
	case Medium:
		d.mediumStreak++
		d.highStreak = 0
	case Low:
		d.highStreak = 0
		d.mediumStreak = 0
	}
}
