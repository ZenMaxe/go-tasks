package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Priority int

const (
	Low    Priority = iota
	Medium Priority = iota
	High   Priority = iota
)

type JobStatus int

// NotDefined is the zero value on purpose: an unset/zeroed JobResult must
// never read as Success by accident.
const (
	NotDefined JobStatus = iota
	Success
	PermanentlyFailed
	Cancelled
)

type Job struct {
	Id       int
	Priority Priority
	Attempts int
}

type JobResult struct {
	JobId    int
	Status   JobStatus
	Attempts int
}

const (
	totalJobs   = 5000
	streakLimit = 5
	workerCount = 5
)

func Scheduler() {
	summary := runScheduler()

	fmt.Println("Final summary:")
	fmt.Printf("  Success:            %d\n", summary[Success])
	fmt.Printf("  Permanently failed: %d\n", summary[PermanentlyFailed])
	fmt.Printf("  Cancelled:          %d\n", summary[Cancelled])
}

func runScheduler() map[JobStatus]int {
	jobResults := make(chan JobResult)
	highPriorityJobs := make(chan Job)
	mediumPriorityJobs := make(chan Job)
	lowPriorityJobs := make(chan Job)
	retryJobs := make(chan Job, 100)
	// Shutdown can arrive two ways: an external signal (Ctrl+C / SIGTERM)
	// at any time, or the natural-completion cancel() below once every
	// job has been accounted for. Both go through the same ctx.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dispatcher := NewDispatcher(highPriorityJobs, mediumPriorityJobs, lowPriorityJobs, retryJobs, jobResults, ctx, streakLimit)

	var sendersWg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		worker := NewWorker(dispatcher)
		sendersWg.Add(1)
		go func() {
			defer sendersWg.Done()
			worker.Run()
		}()
	}

	sendersWg.Add(1)
	go func() {
		defer sendersWg.Done()
		dispatcher.Run()
	}()

	go produceJobs(ctx, highPriorityJobs, mediumPriorityJobs, lowPriorityJobs, totalJobs)

	go func() {
		sendersWg.Wait()
		close(jobResults)
	}()

	summary := make(map[JobStatus]int)
	received := 0
	for r := range jobResults {
		summary[r.Status]++
		received++
		if received == totalJobs {
			stop() // natural completion — same shutdown path as a signal
		}
	}

	return summary
}

func produceJobs(ctx context.Context, high, medium, low chan<- Job, count int) {
	for i := 0; i < count; i++ {
		job := Job{
			Id:       i,
			Priority: Priority(rand.Intn(3)),
			Attempts: 0,
		}

		var target chan<- Job
		switch job.Priority {
		case High:
			target = high
		case Medium:
			target = medium
		case Low:
			target = low
		}

		select {
		case target <- job:
		case <-ctx.Done():
			return
		}
	}
}
