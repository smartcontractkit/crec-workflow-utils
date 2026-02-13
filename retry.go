package workflows

import (
	"errors"
	"fmt"
	"log/slog"
	"time"
)

var (
	InitialRetryDelay = 5 * time.Second
	Attempts          = 3
)

type nonRetriableError struct {
	error
}

func (e *nonRetriableError) Unwrap() error { return e.error }

// StopRetry wraps an error to indicate that the retry loop should stop.
func StopRetry(err error) error {
	return &nonRetriableError{err}
}

// Retry is a generic helper to retry operations up to a fixed number of times.
// It uses an exponential backoff strategy starting at 5 seconds.
// If the operation fails after all Attempts, it returns the last error wrapped with context.
// It stops retrying immediately if the function returns an error wrapped with StopRetry.
func Retry[T any](logger *slog.Logger, name string, fn func() (T, error)) (T, error) {
	var val T
	var err error
	delay := InitialRetryDelay
	for i := 0; i < Attempts; i++ {
		if i > 0 {
			if logger != nil {
				logger.Info("retrying operation", "operation", name, "attempt", i+1, "delay", delay)
			}
			time.Sleep(delay)
			delay *= 2
		}
		val, err = fn()
		if err == nil {
			return val, nil
		}

		var ne *nonRetriableError
		if errors.As(err, &ne) {
			return val, ne.Unwrap()
		}

		if logger != nil {
			logger.Warn("operation failed", "operation", name, "error", err)
		}
	}
	return val, fmt.Errorf("%s failed after %d Attempts: %w", name, Attempts, err)
}
