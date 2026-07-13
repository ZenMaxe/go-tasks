package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	workerCount = 5
	jobCount    = 500
)

func main() {
	jobs := make(chan int)
	var wg sync.WaitGroup
	var processed atomic.Int64

	for workerID := 1; workerID <= workerCount; workerID++ {
		wg.Add(1)
		go worker(workerID, jobs, &wg, &processed)
	}

	for i := 1; i <= jobCount; i++ {
		jobs <- i
	}
	close(jobs)

	wg.Wait()

	fmt.Printf("done: %d jobs processed\n", processed.Load())
}

func worker(id int, jobs <-chan int, wg *sync.WaitGroup, processed *atomic.Int64) {
	defer wg.Done()

	for job := range jobs {
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
		fmt.Printf("worker %d processed job %d\n", id, job)
		processed.Add(1)
	}
}
