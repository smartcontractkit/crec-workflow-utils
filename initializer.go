package workflows

import (
	"fmt"
	"strconv"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/crec-api-go/models"
)

// LogHandler is the function signature implemented by each event-listener workflow's
// per-project handler (e.g. OnLog, OnCoordinatorLog). It processes an EVM log
// and returns a base64-encoded verifiable event (or empty string) and an error.
type LogHandler func(*Config, cre.Runtime, *evm.Log, models.ConfidenceLevel) (string, error)

// InitEventListenerWorkflow wires the standard EVM Log trigger for event-listener
// workflows and attaches the provided handler - once for each of the supplied confidence levels.
// It resolves the event signatures from the ABI for all events in ContractEventNames and uses
// cfg.ChainSelector (required in the config).
func InitEventListenerWorkflow(
	cfg *Config, handler LogHandler,
) (cre.Workflow[*Config], error) {
	abiJSON, err := GetContractABI(cfg, cfg.DetectEventTriggerConfig.ContractName)
	if err != nil {
		return nil, err
	}

	var eventSigHashes [][]byte
	for _, eventName := range cfg.DetectEventTriggerConfig.ContractEventNames {
		ev := MustEvent(abiJSON, eventName)
		eventSigHashes = append(eventSigHashes, ev.ID.Bytes())
	}

	if len(eventSigHashes) == 0 {
		return nil, fmt.Errorf("no valid event names found to trigger on")
	}

	// Convert chainSelector string to uint64 for EVM client
	chainSelector, err := strconv.ParseUint(cfg.ChainSelector, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chain selector: %w", err)
	}

	var executionHandlers []cre.ExecutionHandler[*Config, cre.Runtime]

	for _, confidenceStr := range *cfg.ConfidenceLevels {
		confidence, err := ConfidenceLevelFromString(confidenceStr)
		if err != nil {
			return nil, err
		}
		filter := NewEVMLogFilter(cfg.DetectEventTriggerConfig.ContractAddress, eventSigHashes, confidence)
		executionHandlers = append(
			executionHandlers, cre.Handler(
				evm.LogTrigger(chainSelector, filter),
				func(cfg *Config, rt cre.Runtime, payload *evm.Log) (string, error) {
					return handler(cfg, rt, payload, models.ConfidenceLevel(confidenceStr))
				},
			),
		)

	}

	return executionHandlers, nil
}
