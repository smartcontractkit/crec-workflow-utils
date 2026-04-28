package workflows

import "strings"

// NonRetriableCapabilityErrors lists error substrings that should never be retried
// inside CRE workflows. These represent timeouts or resource limits where a retry
// will either be killed by the ExecutionTimeout or produce the same failure.
var NonRetriableCapabilityErrors = []string{
	"consensus timeout",
	"execution timeout",
	"capability timeout",
	"context deadline exceeded",
	"WASM memory limit",
}

// ClassifyError inspects err and wraps it with [StopRetry] when the error message
// matches any pattern in [NonRetriableCapabilityErrors]. This prevents the [Retry]
// loop from wasting execution budget on errors that cannot succeed on a subsequent attempt.
// Returns nil when err is nil.
func ClassifyError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	for _, pattern := range NonRetriableCapabilityErrors {
		if strings.Contains(msg, strings.ToLower(pattern)) {
			return StopRetry(err)
		}
	}
	return err
}
