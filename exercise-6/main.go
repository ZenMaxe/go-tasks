package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

func main() {
	var retry Retry = NewRetry(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := retry.Retry(ctx, 5, func(ctx context.Context) (err error) {
		if rand.Float32() < 0.5 {
			return errors.New("random failure")
		}
		return nil
	})

	if err != nil {
		fmt.Println(err)
	}
}
