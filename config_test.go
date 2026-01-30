package workflows_test

import (
	"testing"

	workflows "github.com/smartcontractkit/cre-workflow-utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTransferABI = `[
	{"type":"event","name":"Transfer","inputs":[
		{"name":"from","type":"address","indexed":true},
		{"name":"to","type":"address","indexed":true},
		{"name":"value","type":"uint256","indexed":false}
	]}
]`

func TestConfig_GetContractABI(t *testing.T) {
	testCases := []struct {
		name         string
		cfg          *workflows.Config
		contractName string
		wantErr      bool
		errContains  string
		validate     func(t *testing.T, abi string)
	}{
		{
			name: "returns ABI string when contract exists",
			cfg: &workflows.Config{
				DetectEventTriggerConfig: workflows.DetectEventTriggerConfig{
					ContractReaderConfig: workflows.ContractReaderConfig{
						Contracts: map[string]workflows.ContractDef{
							"TestContract": {ContractABI: testTransferABI},
						},
					},
				},
			},
			contractName: "TestContract",
			wantErr:      false,
			validate: func(t *testing.T, abi string) {
				assert.Contains(t, abi, "Transfer")
				assert.Contains(t, abi, "address")
			},
		},
		{
			name: "returns error when contract not found",
			cfg: &workflows.Config{
				DetectEventTriggerConfig: workflows.DetectEventTriggerConfig{
					ContractReaderConfig: workflows.ContractReaderConfig{
						Contracts: map[string]workflows.ContractDef{},
					},
				},
			},
			contractName: "NonExistent",
			wantErr:      true,
			errContains:  "not found",
		},
		{
			name: "returns error when contractABI is nil",
			cfg: &workflows.Config{
				DetectEventTriggerConfig: workflows.DetectEventTriggerConfig{
					ContractReaderConfig: workflows.ContractReaderConfig{
						Contracts: map[string]workflows.ContractDef{
							"EmptyContract": {ContractABI: nil},
						},
					},
				},
			},
			contractName: "EmptyContract",
			wantErr:      true,
			errContains:  "missing",
		},
		{
			name: "marshals non-string ABI to JSON",
			cfg: &workflows.Config{
				DetectEventTriggerConfig: workflows.DetectEventTriggerConfig{
					ContractReaderConfig: workflows.ContractReaderConfig{
						Contracts: map[string]workflows.ContractDef{
							"ObjectABI": {ContractABI: []map[string]any{
								{"type": "event", "name": "Test"},
							}},
						},
					},
				},
			},
			contractName: "ObjectABI",
			wantErr:      false,
			validate: func(t *testing.T, abi string) {
				assert.Contains(t, abi, `"type":"event"`)
				assert.Contains(t, abi, `"name":"Test"`)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := workflows.GetContractABI(tc.cfg, tc.contractName)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				return
			}

			require.NoError(t, err)
			if tc.validate != nil {
				tc.validate(t, result)
			}
		})
	}
}

func TestConfig_GetEventSignature(t *testing.T) {
	testCases := []struct {
		name      string
		cfg       *workflows.Config
		eventName string
		expected  string
	}{
		{
			name: "returns event signature when found",
			cfg: &workflows.Config{
				DetectEventTriggerConfig: workflows.DetectEventTriggerConfig{
					ContractName: "TestContract",
					ContractReaderConfig: workflows.ContractReaderConfig{
						Contracts: map[string]workflows.ContractDef{
							"TestContract": {ContractABI: testTransferABI},
						},
					},
				},
			},
			eventName: "Transfer",
			expected:  "Transfer(address,address,uint256)",
		},
		{
			name: "returns empty when contract not found",
			cfg: &workflows.Config{
				DetectEventTriggerConfig: workflows.DetectEventTriggerConfig{
					ContractName: "NonExistent",
					ContractReaderConfig: workflows.ContractReaderConfig{
						Contracts: map[string]workflows.ContractDef{},
					},
				},
			},
			eventName: "Transfer",
			expected:  "",
		},
		{
			name: "returns empty when event not found",
			cfg: &workflows.Config{
				DetectEventTriggerConfig: workflows.DetectEventTriggerConfig{
					ContractName: "TestContract",
					ContractReaderConfig: workflows.ContractReaderConfig{
						Contracts: map[string]workflows.ContractDef{
							"TestContract": {ContractABI: testTransferABI},
						},
					},
				},
			},
			eventName: "NonExistentEvent",
			expected:  "",
		},
		{
			name: "returns empty for invalid ABI JSON",
			cfg: &workflows.Config{
				DetectEventTriggerConfig: workflows.DetectEventTriggerConfig{
					ContractName: "BadContract",
					ContractReaderConfig: workflows.ContractReaderConfig{
						Contracts: map[string]workflows.ContractDef{
							"BadContract": {ContractABI: "not valid json"},
						},
					},
				},
			},
			eventName: "Transfer",
			expected:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := workflows.GetEventSignature(tc.cfg, tc.eventName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestConfig_ParseWorkflowConfig(t *testing.T) {
	testCases := []struct {
		name        string
		input       []byte
		wantErr     bool
		errContains string
		validate    func(t *testing.T, cfg *workflows.Config)
	}{
		{
			name: "parses valid YAML config",
			input: []byte(`
network: evm
chainID: "1"
chainSelector: "11155111"
courierURL: https://courier.example.com
watcherID: test-watcher
workflowName: test-workflow
detectEventTriggerConfig:
  contractName: TestContract
  contractAddress: "0x1234"
  contractEventNames: ["Transfer"]
`),
			wantErr: false,
			validate: func(t *testing.T, cfg *workflows.Config) {
				assert.Equal(t, "evm", cfg.Network)
				assert.Equal(t, "1", cfg.ChainID)
				assert.Equal(t, "11155111", cfg.ChainSelector)
				assert.Equal(t, "TestContract", cfg.DetectEventTriggerConfig.ContractName)
				assert.Equal(t, []string{"Transfer"}, cfg.DetectEventTriggerConfig.ContractEventNames)
			},
		},
		{
			name: "parses valid JSON config",
			input: []byte(`{
				"network": "evm",
				"chainID": "1",
				"chainSelector": "16015286601757825753",
				"courierURL": "https://courier.example.com",
				"detectEventTriggerConfig": {
					"contractName": "MyContract",
					"contractEventNames": ["EventA", "EventB"]
				}
			}`),
			wantErr: false,
			validate: func(t *testing.T, cfg *workflows.Config) {
				assert.Equal(t, "evm", cfg.Network)
				assert.Equal(t, "16015286601757825753", cfg.ChainSelector)
				assert.Equal(t, "MyContract", cfg.DetectEventTriggerConfig.ContractName)
				assert.Equal(t, []string{"EventA", "EventB"}, cfg.DetectEventTriggerConfig.ContractEventNames)
			},
		},
		{
			name:        "fails when chainSelector is missing",
			input:       []byte(`{"network": "evm", "chainID": "1"}`),
			wantErr:     true,
			errContains: "chain selector is required",
		},
		{
			name:        "fails when chainSelector is zero",
			input:       []byte(`{"network": "evm", "chainID": "1", "chainSelector": "0"}`),
			wantErr:     true,
			errContains: "chain selector is required",
		},
		{
			name:        "fails on invalid input",
			input:       []byte(`not valid yaml or json {{{{`),
			wantErr:     true,
			errContains: "failed to parse",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := workflows.ParseWorkflowConfig(tc.input)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			if tc.validate != nil {
				tc.validate(t, result)
			}
		})
	}
}
