// Package workflows provides common utilities for CRE (Chainlink Runtime Environment)
// event watcher workflow development. It is used by CREC workflow extensions that
// listen for EVM contract events and post verifiable events to CREC.
//
// # Key Features
//
//   - Configuration parsing — [ParseWorkflowConfig] for YAML/JSON workflow configs
//   - Event processing — [BuildEVMEventFromLog], [BuildVerifiableEventForEVMEvent], [SignAndPostVerifiableEvent]
//   - Workflow initialization — [InitEventListenerWorkflow] wires EVM log triggers with [LogHandler]
//   - Testing utilities — [PrepareTestingRuntime] for unit tests
//
// # Usage
//
// Create a workflow entry point:
//
//	r := wasm.NewRunner(workflows.ParseWorkflowConfig)
//	r.Run(func(cfg *workflows.Config, _ *slog.Logger, _ cre.SecretsProvider) (cre.Workflow[*workflows.Config], error) {
//	    return workflows.InitEventListenerWorkflow(cfg, OnLog)
//	})
//
// Implement a [LogHandler] that decodes the EVM log, builds a verifiable event, and posts it:
//
//	func OnLog(cfg *workflows.Config, rt cre.Runtime, payload *evm.Log) (string, error) {
//	    evmEvent, err := workflows.BuildEVMEventFromLog(rt, cfg, payload)
//	    if err != nil { return "", err }
//	    abiJSON, err := workflows.GetContractABI(cfg, cfg.DetectEventTriggerConfig.ContractName)
//	    if err != nil { return "", err }
//	    eventName, err := workflows.GetEventNameFromLog(cfg, payload, abiJSON)
//	    if err != nil { return "", err }
//	    ve, err := workflows.BuildVerifiableEventForEVMEvent(cfg, evmEvent, cfg.Service, eventName, nil)
//	    if err != nil { return "", err }
//	    return workflows.SignAndPostVerifiableEvent(cfg, rt, ve)
//	}
//
// # Error Handling
//
// Errors from [ParseWorkflowConfig] indicate invalid YAML/JSON or missing required fields (e.g. chainSelector).
// [GetContractABI] returns an error if the contract is not found in config.
// [SignAndPostVerifiableEvent] returns errors from consensus generation or CREC API failures.
package workflows
