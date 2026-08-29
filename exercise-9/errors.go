package main

import "errors"

var (
	ErrHalfOpenLimitReached error = errors.New("half-open limit reached")
	ErrCircuitOpen          error = errors.New("circuit open")
	ErrUnknown              error = errors.New("unknown error")
)
