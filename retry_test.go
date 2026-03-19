package workflows

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	fastRetry := &RetryConfig{MaxAttempts: 2, InitialDelay: "1ms"}
	logger := slog.Default()

	t.Run("InstantAvailability", func(t *testing.T) {
		callCount := 0
		fn := func() (string, error) {
			callCount++
			return "success", nil
		}

		val, err := Retry(logger, "test-instant", fastRetry, fn)
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

		val, err := Retry(logger, "test-unavailable", fastRetry, fn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "test-unavailable failed after 2 attempts")
		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, "", val)
		assert.Equal(t, 2, callCount)
	})

	t.Run("AvailabilityAfterSomeTime", func(t *testing.T) {
		callCount := 0
		fn := func() (string, error) {
			callCount++
			if callCount < 2 {
				return "", errors.New("temporary error")
			}
			return "recovered", nil
		}

		val, err := Retry(logger, "test-recover", fastRetry, fn)
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

		val, err := Retry(logger, "test-stop-retry", fastRetry, fn)
		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, "", val)
		assert.Equal(t, 1, callCount)
	})
}

func TestRetry_negativeMaxAttemptsUsesDefault(t *testing.T) {
	logger := slog.Default()
	callCount := 0
	fn := func() (string, error) {
		callCount++
		return "", errors.New("fail")
	}
	_, err := Retry(logger, "test-negative-attempts", &RetryConfig{MaxAttempts: -1, InitialDelay: "1ms"}, fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test-negative-attempts failed after 3 attempts")
	assert.Equal(t, 3, callCount)
}
