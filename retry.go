package workflows

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"
)

const (
	defaultRetryMaxAttempts  = 3
	defaultRetryInitialDelay = "5s"
)

// RetryConfig controls how many times [Retry] runs fn and how long to wait between retries.
//
// Management and defaults:
//   - Pass nil for rc to use the library defaults (3 attempts, initial delay 5s).
//   - Pass a non-nil value to override; fields that are "unset" fall back to the same defaults:
//     MaxAttempts less than or equal to 0 means 3; InitialDelay "" or a string that [time.ParseDuration] rejects means 5s.
//   - InitialDelay must be a duration string accepted by ParseDuration (e.g. "5s", "1s", "500ms").
//
// Workflows that load retry settings from their own YAML/JSON can define a struct that embeds or
// duplicates these fields with the same tags, then pass the populated *RetryConfig into [Retry].
// Exponential backoff doubles the delay after each failed attempt (after the first).
type RetryConfig struct {
	MaxAttempts  int    `yaml:"maxAttempts,omitempty" json:"maxAttempts,omitempty"`
	InitialDelay string `yaml:"initialDelay,omitempty" json:"initialDelay,omitempty"`
}

type nonRetriableError struct {
	error
}

func (e *nonRetriableError) Unwrap() error { return e.error }

func defaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  defaultRetryMaxAttempts,
		InitialDelay: defaultRetryInitialDelay,
	}
}

func resolveRetry(rc *RetryConfig) (attempts int, initialDelay time.Duration) {
	effective := rc
	if effective == nil {
		effective = defaultRetryConfig()
	}
	attempts = effective.MaxAttempts
	if attempts <= 0 {
		attempts = defaultRetryMaxAttempts
	}
	delayStr := effective.InitialDelay
	if delayStr == "" {
		delayStr = defaultRetryInitialDelay
	}
	d, err := time.ParseDuration(delayStr)
	if err != nil {
		d, _ = time.ParseDuration(defaultRetryInitialDelay)
	}
	return attempts, d
}

// StopRetry wraps an error to indicate that the retry loop should stop.
func StopRetry(err error) error {
	return &nonRetriableError{err}
}

// jitter applies ±25% randomization to the given duration to avoid synchronized retry bursts.
func jitter(d time.Duration) time.Duration {
	delta := float64(d) * 0.25
	return d + time.Duration((rand.Float64()*2-1)*delta)
}

// Retry is a generic helper to retry operations up to a fixed number of times.
// It uses an exponential backoff strategy with jitter (±25%); the starting delay comes from rc (see [RetryConfig]).
// If the operation fails after all attempts are exhausted, it returns the last error wrapped with context.
// It stops retrying immediately if the function returns an error wrapped with StopRetry.
func Retry[T any](logger *slog.Logger, name string, rc *RetryConfig, fn func() (T, error)) (T, error) {
	attempts, delayDuration := resolveRetry(rc)
	var val T
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			jitteredDelay := jitter(delayDuration)
			if logger != nil {
				logger.Info("retrying operation", "operation", name, "attempt", i+1, "delay", jitteredDelay)
			}
			time.Sleep(jitteredDelay)
			delayDuration *= 2
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
	return val, fmt.Errorf("%s failed after %d attempts: %w", name, attempts, err)
}
