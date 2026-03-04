// Package workflows provides common utilities for CRE event watcher workflow development.
//
// Key features:
//   - Configuration parsing (YAML/JSON workflow configs)
//   - Event processing (decode EVM logs, build consensus reports)
//   - Workflow initialization (wire up EVM log triggers with handlers)
//   - Testing utilities (mock runtime for unit testing)
//
// Usage:
//
//	import workflows "github.com/smartcontractkit/crec-workflow-utils"
//
//	func OnLog(cfg *workflows.Config, rt cre.Runtime, payload *evm.Log) (string, error) {
//	    // Decode and process event...
//	}
package workflows
