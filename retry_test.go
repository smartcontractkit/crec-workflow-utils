package workflows

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
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
		assert.Contains(t, err.Error(), "test-unavailable failed after 2 Attempts")
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

func TestConfidenceLevelFromString(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected evm.ConfidenceLevel
	}{
		{"empty", "", evm.ConfidenceLevel_CONFIDENCE_LEVEL_LATEST},
		{"whitespace_only", "  ", evm.ConfidenceLevel_CONFIDENCE_LEVEL_LATEST},
		{"unknown", "unknown", evm.ConfidenceLevel_CONFIDENCE_LEVEL_LATEST},
		{"latest_lower", "latest", evm.ConfidenceLevel_CONFIDENCE_LEVEL_LATEST},
		{"latest_mixed", "LATEST", evm.ConfidenceLevel_CONFIDENCE_LEVEL_LATEST},
		{"safe_lower", "safe", evm.ConfidenceLevel_CONFIDENCE_LEVEL_SAFE},
		{"safe_mixed", "SAFE", evm.ConfidenceLevel_CONFIDENCE_LEVEL_SAFE},
		{"finalized_lower", "finalized", evm.ConfidenceLevel_CONFIDENCE_LEVEL_FINALIZED},
		{"finalized_mixed", "FINALIZED", evm.ConfidenceLevel_CONFIDENCE_LEVEL_FINALIZED},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ConfidenceLevelFromString(tt.in))
		})
	}
}
