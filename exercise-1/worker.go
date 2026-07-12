package main

import (
	"context"
	"math/rand"
)

const (
	failureRate = 0.3
	maxAttempts = 2
)

type Worker struct {
	ctx        context.Context
	retry      chan Job
	result     chan JobResult
	workerChan chan Job
}

func NewWorker(d *Dispatcher) *Worker {
	return &Worker{
		ctx:        d.ctx,
		retry:      d.retry,
		result:     d.result,
		workerChan: d.workerChan,
	}
}

func (w *Worker) Run() {
	for job := range w.workerChan {
		w.handlePickedJob(job)
	}
}

func (w *Worker) handlePickedJob(job Job) {
	roll := rand.Float32()
	if roll < failureRate {
		job.Attempts++

		if job.Attempts == maxAttempts {
			w.sendResult(JobResult{JobId: job.Id, Status: PermanentlyFailed, Attempts: job.Attempts})
			return
		}
		w.sendRetry(job)
	} else {
		w.sendResult(JobResult{JobId: job.Id, Status: Success, Attempts: job.Attempts})
	}
}

func (w *Worker) sendResult(result JobResult) {
	w.result <- result
}

func (w *Worker) sendRetry(job Job) {
	select {
	case w.retry <- job:
	case <-w.ctx.Done():
		// Dispatcher already stopped reading retry — this job can never
		// come back around, so it must still be reported, not dropped.
		w.result <- JobResult{JobId: job.Id, Status: Cancelled, Attempts: job.Attempts}
	}
}
