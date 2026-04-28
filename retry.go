package workflows

import (
	"errors"
	"fmt"
	"log/slog"
)

const (
	defaultRetryMaxAttempts = 3
)

// RetryConfig controls how many times [Retry] runs fn.
//
// Management and defaults:
//   - Pass nil for rc to use the library defaults (3 attempts).
//   - Pass a non-nil value to override; fields that are "unset" fall back to the same defaults:
//     MaxAttempts less than or equal to 0 means 3.
//
// CRE-compliance note: CRE workflows execute across DON nodes that must produce
// identical execution traces. Therefore **no wall-clock sleep or jitter** is used
// between retry attempts — retries happen immediately. If the CRE runtime exposes
// a deterministic wait primitive in the future, it should be used here.
type RetryConfig struct {
	MaxAttempts int `yaml:"maxAttempts,omitempty" json:"maxAttempts,omitempty"`
}

type nonRetriableError struct {
	error
}

func (e *nonRetriableError) Unwrap() error { return e.error }

func defaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: defaultRetryMaxAttempts,
	}
}

func resolveRetry(rc *RetryConfig) int {
	effective := rc
	if effective == nil {
		effective = defaultRetryConfig()
	}
	attempts := effective.MaxAttempts
	if attempts <= 0 {
		attempts = defaultRetryMaxAttempts
	}
	return attempts
}

// StopRetry wraps an error to indicate that the retry loop should stop.
func StopRetry(err error) error {
	return &nonRetriableError{err}
}

// Retry is a generic, CRE-deterministic helper to retry operations up to a fixed
// number of times. It retries immediately (no sleep) to preserve DON execution
// determinism. If the operation fails after all attempts are exhausted, it returns
// the last error wrapped with context.
//
// It stops retrying immediately if the function returns an error wrapped with
// [StopRetry].
//
// TODO: Once CRE error strings are verified, integrate [ClassifyError] here to
// automatically classify capability/consensus timeouts as non-retriable.
func Retry[T any](logger *slog.Logger, name string, rc *RetryConfig, fn func() (T, error)) (T, error) {
	attempts := resolveRetry(rc)
	var val T
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			if logger != nil {
				logger.Info("retrying operation (immediate, no sleep)", "operation", name, "attempt", i+1)
			}
		}
		val, err = fn()
		if err == nil {
			return val, nil
		}

		var ne *nonRetriableError
		if errors.As(err, &ne) {
			if logger != nil {
				logger.Warn("non-retriable error, stopping retry", "operation", name, "error", ne.Unwrap())
			}
			return val, ne.Unwrap()
		}

		if logger != nil {
			logger.Warn("operation failed", "operation", name, "attempt", i+1, "error", err)
		}
	}
	return val, fmt.Errorf("%s failed after %d attempts: %w", name, attempts, err)
}
