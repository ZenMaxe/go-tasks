package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
)

func main() {
	var hc HealthChecker = NewHealthChecker(context.Background())
	var cb = NewCircuitBreaker(context.Background())
	var wg sync.WaitGroup

	hc.Run()
	go cb.AssignHealthChecker(hc)

	for range 10000 {
		wg.Go(func() {
			isAllowed := cb.Allow()
			if !isAllowed {
				fmt.Printf("Not allowed circuit breaker\n")
				return
			}

			if rand.Intn(10) < 2 {
				cb.RecordFailure()
				fmt.Printf("Reached failure\n")
			} else {
				cb.RecordSuccess()
				fmt.Printf("Reached success\n")
			}
		})
	}

	wg.Wait()
}
