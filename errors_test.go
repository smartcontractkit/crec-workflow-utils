package workflows

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyError_nil(t *testing.T) {
	assert.Nil(t, ClassifyError(nil))
}

func TestClassifyError_retriable(t *testing.T) {
	err := errors.New("connection refused")
	result := ClassifyError(err)
	// Should pass through unchanged (no StopRetry wrapping).
	assert.Equal(t, err, result)

	var ne *nonRetriableError
	assert.False(t, errors.As(result, &ne), "retriable error should not be wrapped as nonRetriableError")
}

func TestClassifyError_nonRetriable(t *testing.T) {
	cases := []string{
		"consensus timeout exceeded",
		"Execution Timeout reached",
		"capability timeout: GenerateReport",
		"context deadline exceeded",
		"WASM memory limit exceeded",
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			err := errors.New(msg)
			result := ClassifyError(err)
			require.Error(t, result)

			var ne *nonRetriableError
			require.True(t, errors.As(result, &ne), "expected non-retriable wrapping for: %s", msg)
			// The unwrapped error should be the original.
			assert.ErrorIs(t, ne.Unwrap(), err)
		})
	}
}

func TestClassifyError_caseInsensitive(t *testing.T) {
	err := errors.New("CONSENSUS TIMEOUT")
	result := ClassifyError(err)
	var ne *nonRetriableError
	assert.True(t, errors.As(result, &ne), "pattern matching should be case-insensitive")
}
