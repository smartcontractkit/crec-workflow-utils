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
	originalDelay := InitialRetryDelay
	originalAttempts := Attempts
	InitialRetryDelay = 1 * time.Millisecond
	Attempts = 2
	defer func() {
		InitialRetryDelay = originalDelay
		Attempts = originalAttempts
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
		assert.Contains(t, err.Error(), "test-unavailable failed after 2 Attempts")
		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, "", val)
		assert.Equal(t, 2, callCount)
	})

	t.Run("AvailabilityAfterSomeTime", func(t *testing.T) {
		callCount := 0
		// Fails 1 time, succeeds on the 2nd
		fn := func() (string, error) {
			callCount++
			if callCount < 2 {
				return "", errors.New("temporary error")
			}
			return "recovered", nil
		}

		val, err := Retry(logger, "test-recover", fn)
		assert.NoError(t, err)
		assert.Equal(t, "recovered", val)
		assert.Equal(t, 2, callCount)
	})

	t.Run("StopRetry", func(t *testing.T) {
		callCount := 0
		expectedErr := errors.New("fatal error")
		fn := func() (string, error) {
			callCount++
			return "", StopRetry(expectedErr)
		}

		val, err := Retry(logger, "test-stop-retry", fn)
		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, "", val)
		assert.Equal(t, 1, callCount) // Should stop after first attempt
	})
}
