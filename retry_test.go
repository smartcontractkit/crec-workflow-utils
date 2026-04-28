package workflows

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	fastRetry := &RetryConfig{MaxAttempts: 2}
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

	t.Run("StopRetry_from_callback", func(t *testing.T) {
		callCount := 0
		fn := func() (string, error) {
			callCount++
			// Caller explicitly classifies error as non-retriable.
			return "", StopRetry(errors.New("consensus timeout exceeded"))
		}

		val, err := Retry(logger, "test-stop-classify", &RetryConfig{MaxAttempts: 5}, fn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consensus timeout exceeded")
		assert.Equal(t, "", val)
		assert.Equal(t, 1, callCount, "should not retry after StopRetry")
	})

	t.Run("retriable_errors_exhaust_all_attempts", func(t *testing.T) {
		callCount := 0
		fn := func() (string, error) {
			callCount++
			return "", errors.New("connection refused")
		}

		_, err := Retry(logger, "test-retriable", &RetryConfig{MaxAttempts: 3}, fn)
		assert.Error(t, err)
		assert.Equal(t, 3, callCount)
	})
}

func TestRetry_negativeMaxAttemptsUsesDefault(t *testing.T) {
	logger := slog.Default()
	callCount := 0
	fn := func() (string, error) {
		callCount++
		return "", errors.New("fail")
	}
	_, err := Retry(logger, "test-negative-attempts", &RetryConfig{MaxAttempts: -1}, fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test-negative-attempts failed after 3 attempts")
	assert.Equal(t, 3, callCount)
}

func TestRetry_nilConfigUsesDefaults(t *testing.T) {
	callCount := 0
	fn := func() (string, error) {
		callCount++
		return "", errors.New("fail")
	}
	_, err := Retry(slog.Default(), "test-nil-config", nil, fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test-nil-config failed after 3 attempts")
	assert.Equal(t, 3, callCount, "nil config should default to 3 attempts")
}

func TestRetry_nilLoggerDoesNotPanic(t *testing.T) {
	callCount := 0
	fn := func() (string, error) {
		callCount++
		if callCount < 2 {
			return "", errors.New("transient")
		}
		return "ok", nil
	}
	val, err := Retry(nil, "test-nil-logger", &RetryConfig{MaxAttempts: 3}, fn)
	assert.NoError(t, err)
	assert.Equal(t, "ok", val)
	assert.Equal(t, 2, callCount)
}

func TestRetry_nilLoggerWithStopRetry(t *testing.T) {
	fn := func() (string, error) {
		return "", StopRetry(errors.New("fatal"))
	}
	_, err := Retry(nil, "test-nil-logger-stop", &RetryConfig{MaxAttempts: 3}, fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fatal")
}

func TestRetry_singleAttempt(t *testing.T) {
	callCount := 0
	fn := func() (string, error) {
		callCount++
		return "", errors.New("fail")
	}
	_, err := Retry(slog.Default(), "test-single", &RetryConfig{MaxAttempts: 1}, fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test-single failed after 1 attempts")
	assert.Equal(t, 1, callCount, "MaxAttempts=1 should run exactly once")
}
