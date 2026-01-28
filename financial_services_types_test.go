package workflows_test

import (
	"encoding/json"
	"testing"

	workflows "github.com/smartcontractkit/cre-workflow-utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinancialServicesTypes_Fixed2_MarshalJSON(t *testing.T) {
	testCases := []struct {
		name     string
		value    workflows.Fixed2
		expected string
	}{
		{
			name:     "marshals zero",
			value:    workflows.Fixed2(0),
			expected: "0.00",
		},
		{
			name:     "marshals positive integer",
			value:    workflows.Fixed2(100),
			expected: "100.00",
		},
		{
			name:     "marshals decimal with 2 places",
			value:    workflows.Fixed2(123.45),
			expected: "123.45",
		},
		{
			name:     "truncates to 2 decimal places",
			value:    workflows.Fixed2(99.999),
			expected: "100.00",
		},
		{
			name:     "marshals negative value",
			value:    workflows.Fixed2(-50.25),
			expected: "-50.25",
		},
		{
			name:     "marshals large value",
			value:    workflows.Fixed2(1000000.99),
			expected: "1000000.99",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.value.MarshalJSON()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, string(result))
		})
	}
}

func TestFinancialServicesTypes_Fixed2_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected workflows.Fixed2
		wantErr  bool
	}{
		{
			name:     "unmarshals number",
			input:    "123.45",
			expected: workflows.Fixed2(123.45),
			wantErr:  false,
		},
		{
			name:     "unmarshals integer",
			input:    "100",
			expected: workflows.Fixed2(100),
			wantErr:  false,
		},
		{
			name:     "unmarshals quoted string",
			input:    `"99.99"`,
			expected: workflows.Fixed2(99.99),
			wantErr:  false,
		},
		{
			name:     "unmarshals zero",
			input:    "0",
			expected: workflows.Fixed2(0),
			wantErr:  false,
		},
		{
			name:     "unmarshals negative",
			input:    "-25.50",
			expected: workflows.Fixed2(-25.50),
			wantErr:  false,
		},
		{
			name:    "fails on invalid string",
			input:   `"not a number"`,
			wantErr: true,
		},
		{
			name:    "fails on invalid JSON",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result workflows.Fixed2
			err := result.UnmarshalJSON([]byte(tc.input))

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.InDelta(t, float64(tc.expected), float64(result), 0.001)
		})
	}
}

func TestFinancialServicesTypes_Fixed2_RoundTrip(t *testing.T) {
	type Wrapper struct {
		Amount workflows.Fixed2 `json:"amount"`
	}

	original := Wrapper{Amount: workflows.Fixed2(1234.56)}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Wrapper
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.InDelta(t, float64(original.Amount), float64(decoded.Amount), 0.001)
}

func TestFinancialServicesTypes_ComposeWorkflowEventMetadata(t *testing.T) {
	testCases := []struct {
		name      string
		component string
		chainID   string
		eventType string
		params    map[string]any
		validate  func(t *testing.T, result map[string]any)
	}{
		{
			name:      "composes basic metadata",
			component: "event-listener-dta",
			chainID:   "11155111",
			eventType: "RequestCreated",
			params:    nil,
			validate: func(t *testing.T, result map[string]any) {
				assert.Equal(t, "11155111", result["chainId"])
				assert.Equal(t, "evm", result["network"])

				workflowEvent := result["workflowEvent"].(map[string]any)
				assert.Equal(t, "event-listener-dta", workflowEvent["component"])
				assert.Equal(t, "RequestCreated", workflowEvent["event_type_label"])

				labels := workflowEvent["process_labels"].([]string)
				assert.Contains(t, labels, "dta")
				assert.Contains(t, labels, "RequestCreated")
			},
		},
		{
			name:      "includes custom params in attributes",
			component: "my-workflow",
			chainID:   "1",
			eventType: "Transfer",
			params: map[string]any{
				"from":   "0x1234",
				"to":     "0x5678",
				"amount": 1000,
			},
			validate: func(t *testing.T, result map[string]any) {
				workflowEvent := result["workflowEvent"].(map[string]any)
				attrs := workflowEvent["attributes"].(map[string]map[string]any)

				assert.Contains(t, attrs, "from")
				assert.Equal(t, "0x1234", attrs["from"]["value"])
				assert.Equal(t, true, attrs["from"]["on_chain"])

				assert.Contains(t, attrs, "to")
				assert.Equal(t, "0x5678", attrs["to"]["value"])

				assert.Contains(t, attrs, "amount")
				assert.Equal(t, "1000", attrs["amount"]["value"])

				assert.Contains(t, attrs, "chain_id")
				assert.Contains(t, attrs, "event_type")
			},
		},
		{
			name:      "handles single-segment component",
			component: "simple",
			chainID:   "42161",
			eventType: "Approval",
			params:    nil,
			validate: func(t *testing.T, result map[string]any) {
				workflowEvent := result["workflowEvent"].(map[string]any)
				labels := workflowEvent["process_labels"].([]string)
				assert.Contains(t, labels, "simple")
			},
		},
		{
			name:      "extracts last segment from multi-hyphen component",
			component: "prefix-middle-suffix",
			chainID:   "1",
			eventType: "Event",
			params:    nil,
			validate: func(t *testing.T, result map[string]any) {
				workflowEvent := result["workflowEvent"].(map[string]any)
				labels := workflowEvent["process_labels"].([]string)
				assert.Contains(t, labels, "suffix")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := workflows.ComposeWorkflowEventMetadata(tc.component, tc.chainID, tc.eventType, tc.params)
			require.NotNil(t, result)
			tc.validate(t, result)
		})
	}
}
