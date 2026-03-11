package workflows

import (
	"encoding/json"
	"fmt"
	"strings"

	gethAbi "github.com/ethereum/go-ethereum/accounts/abi"
	"gopkg.in/yaml.v3"
)

// Config is the shared configuration for all event-detection workflows (YAML or JSON).
// It mirrors the structure produced by our existing config.tmpl files and remains
// compatible with server-side gomplate rendering used during e2e runs.
type Config struct {
	Network       string  `yaml:"network"                json:"network"`
	ChainID       string  `yaml:"chainID"                json:"chainID"`
	CourierURL    string  `yaml:"courierURL"             json:"courierURL"`
	Service       *string `yaml:"service,omitempty"      json:"service,omitempty"`
	ApiKeySecret  string  `yaml:"apiKeySecret,omitempty" json:"apiKeySecret,omitempty"`
	ChainSelector string  `yaml:"chainSelector"          json:"chainSelector"`
	WatcherID     string  `yaml:"watcherID"              json:"watcherID"`
	WorkflowName  string  `yaml:"workflowName"           json:"workflowName"`

	DetectEventTriggerConfig DetectEventTriggerConfig `yaml:"detectEventTriggerConfig" json:"detectEventTriggerConfig"`
}

// DetectEventTriggerConfig matches the template data for the log trigger.
type DetectEventTriggerConfig struct {
	ContractName         string               `yaml:"contractName"      json:"contractName"`
	ContractAddress      string               `yaml:"contractAddress"   json:"contractAddress"`
	ContractEventNames   []string             `yaml:"contractEventNames"          json:"contractEventNames"`
	ContractReaderConfig ContractReaderConfig `yaml:"contractReaderConfig"        json:"contractReaderConfig"`
}

// ContractReaderConfig contains the contracts map. We only need contractABI for this workflow.
type ContractReaderConfig struct {
	Contracts map[string]ContractDef `yaml:"contracts" json:"contracts"`
}

// ContractDef holds the ABI string in "contractABI".
type ContractDef struct {
	ContractABI any `yaml:"contractABI" json:"contractABI"`
}

// GetContractABI returns the ABI string for the specified contract-name.
// Only the canonical "contractABI" field is supported.
func GetContractABI(cfg *Config, contractName string) (string, error) {
	raw, ok := cfg.DetectEventTriggerConfig.ContractReaderConfig.Contracts[contractName]
	if !ok {
		return "", fmt.Errorf("contract %q not found in contractReaderConfig", contractName)
	}
	if raw.ContractABI == nil {
		return "", fmt.Errorf("contractABI missing for %q", contractName)
	}
	switch v := raw.ContractABI.(type) {
	case string:
		return v, nil
	default:
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}

// GetEventSignature returns the ABI event signature (topic hash) for the given event name
// from the workflow config. It returns an empty string if the contract ABI cannot be loaded,
// parsed, or if the event is not found.
func GetEventSignature(cfg *Config, eventName string) string {
	abiJSON, err := GetContractABI(cfg, cfg.DetectEventTriggerConfig.ContractName)
	if err != nil {
		return ""
	}

	parsedABI, err := gethAbi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return ""
	}
	eventDef, ok := parsedABI.Events[eventName]
	if !ok {
		return ""
	}
	return eventDef.Sig
}

// ParseWorkflowConfig accepts YAML or JSON and returns a Config.
// chainSelector is expected to be provided by the workflow config.
func ParseWorkflowConfig(b []byte) (*Config, error) {
	var cfg Config
	// Try YAML first
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		// If YAML failed, try JSON
		if jerr := json.Unmarshal(b, &cfg); jerr != nil {
			return nil, fmt.Errorf("failed to parse workflow config (yaml/json)")
		}
	}
	if cfg.ChainSelector == "" || cfg.ChainSelector == "0" {
		return nil, fmt.Errorf("chain selector is required")
	}

	// No hard validation here; downstream helpers handle defaults/fallbacks.
	return &cfg, nil
}
