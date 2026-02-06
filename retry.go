package workflows

import (
	"fmt"
	"log/slog"
	"time"
)

var (
	initialRetryDelay = 5 * time.Second
	attempts          = 3
)

// Retry is a generic helper to retry operations up to a fixed number of times.
// It uses an exponential backoff strategy starting at 5 seconds.
// If the operation fails after all attempts, it returns the last error wrapped with context.
func Retry[T any](logger *slog.Logger, name string, fn func() (T, error)) (T, error) {
	var val T
	var err error
	delay := initialRetryDelay
	for i := 0; i < attempts; i++ {
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
		if logger != nil {
			logger.Warn("operation failed", "operation", name, "error", err)
		}
	}
	return val, fmt.Errorf("%s failed after %d attempts: %w", name, attempts, err)
}
