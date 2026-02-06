package workflows

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	// Override configuration for testing to ensure tests run fast.
	// We save the original values and defer their restoration.
	originalDelay := initialRetryDelay
	originalAttempts := attempts
	initialRetryDelay = 1 * time.Millisecond
	attempts = 3
	defer func() {
		initialRetryDelay = originalDelay
		attempts = originalAttempts
	}()

	logger := slog.Default()

	t.Run("InstantAvailability", func(t *testing.T) {
		callCount := 0
		fn := func() (string, error) {
			callCount++
			return "success", nil
		}

		val, err := Retry(logger, "test-instant", fn)
		assert.NoError(t, err)
		assert.Equal(t, "success", val)
		assert.Equal(t, 1, callCount)
	})

	t.Run("CompleteUnavailability", func(t *testing.T) {
		callCount := 0
		expectedErr := errors.New("service down")
		fn := func() (string, error) {
			callCount++
			return "", expectedErr
		}

		val, err := Retry(logger, "test-unavailable", fn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test-unavailable failed after 3 attempts")
		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, "", val)
		assert.Equal(t, 3, callCount)
	})

	t.Run("AvailabilityAfterSomeTime", func(t *testing.T) {
		callCount := 0
		// Fails 2 times, succeeds on the 3rd
		fn := func() (string, error) {
			callCount++
			if callCount < 3 {
				return "", errors.New("temporary error")
			}
			return "recovered", nil
		}

		val, err := Retry(logger, "test-recover", fn)
		assert.NoError(t, err)
		assert.Equal(t, "recovered", val)
		assert.Equal(t, 3, callCount)
	})
}
