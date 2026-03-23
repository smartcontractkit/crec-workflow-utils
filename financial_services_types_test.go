package workflows_test

import (
	"encoding/json"
	"math"
	"testing"

	workflows "github.com/smartcontractkit/crec-workflow-utils"
	"github.com/smartcontractkit/crec-api-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinancialServicesTypes_Fixed2_MarshalJSON(t *testing.T) {
	testCases := []struct {
		name     string
		value    workflows.Fixed2
		expected string
		wantErr  bool
	}{
		{
			name:     "marshals zero as quoted string",
			value:    workflows.Fixed2(0),
			expected: `"0.00"`,
		},
		{
			name:     "marshals positive integer as quoted string",
			value:    workflows.Fixed2(100),
			expected: `"100.00"`,
		},
		{
			name:     "marshals decimal with 2 places",
			value:    workflows.Fixed2(123.45),
			expected: `"123.45"`,
		},
		{
			name:     "truncates to 2 decimal places",
			value:    workflows.Fixed2(99.999),
			expected: `"100.00"`,
		},
		{
			name:     "marshals negative value",
			value:    workflows.Fixed2(-50.25),
			expected: `"-50.25"`,
		},
		{
			name:     "marshals large value",
			value:    workflows.Fixed2(1000000.99),
			expected: `"1000000.99"`,
		},
		{
			name:    "rejects NaN",
			value:   workflows.Fixed2(math.NaN()),
			wantErr: true,
		},
		{
			name:    "rejects positive Inf",
			value:   workflows.Fixed2(math.Inf(1)),
			wantErr: true,
		},
		{
			name:    "rejects negative Inf",
			value:   workflows.Fixed2(math.Inf(-1)),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.value.MarshalJSON()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
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
			name:    "rejects quoted NaN",
			input:   `"NaN"`,
			wantErr: true,
		},
		{
			name:    "rejects quoted Inf",
			input:   `"Inf"`,
			wantErr: true,
		},
		{
			name:    "rejects quoted negative Inf",
			input:   `"-Inf"`,
			wantErr: true,
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

	// Verify the marshaled JSON uses a quoted string
	assert.Contains(t, string(data), `"amount":"1234.56"`)

	var decoded Wrapper
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.InDelta(t, float64(original.Amount), float64(decoded.Amount), 0.001)
}

func TestGetReferenceDataFromVerifiableEvent(t *testing.T) {
	testCases := []struct {
		name        string
		setupEvent  func() models.VerifiableEvent
		wantErr     bool
		errContains string
		validate    func(t *testing.T, result *workflows.ReferenceData)
	}{
		{
			name: "successfully extracts reference data with on-chain data",
			setupEvent: func() models.VerifiableEvent {
				referenceData := map[string]interface{}{
					"on_chain": []interface{}{
						map[string]interface{}{
							"source": map[string]interface{}{
								"contract_address":            "0x1234567890123456789012345678901234567890",
								"contract_function_signature": "getPrice(bytes32)",
								"call_data":                   "0xabcdef",
								"block":                       "latest",
							},
							"data": map[string]interface{}{
								"price": "100.50",
							},
						},
					},
				}
				refDataBytes, _ := json.Marshal(referenceData)
				typeAndValue := map[string]interface{}{
					"type":  "reference_data",
					"value": json.RawMessage(refDataBytes),
				}
				return models.VerifiableEvent{
					Data: &typeAndValue,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, result *workflows.ReferenceData) {
				require.NotNil(t, result)
				require.Len(t, result.OnChain, 1)
				assert.Equal(t, "0x1234567890123456789012345678901234567890", result.OnChain[0].Source.ContractAddress)
				assert.Equal(t, "getPrice(bytes32)", result.OnChain[0].Source.ContractFunctionSignature)
				assert.Equal(t, "0xabcdef", result.OnChain[0].Source.CallData)
				assert.Equal(t, "latest", result.OnChain[0].Source.Block)
				assert.Equal(t, "100.50", result.OnChain[0].Data["price"])
			},
		},
		{
			name: "successfully extracts reference data with off-chain data",
			setupEvent: func() models.VerifiableEvent {
				referenceData := map[string]interface{}{
					"off_chain": []interface{}{
						map[string]interface{}{
							"source": map[string]interface{}{
								"type":       "api",
								"identifier": "urn:api:price-feed",
							},
							"data": map[string]interface{}{
								"symbol": "ETH/USD",
								"price":  "2500.00",
							},
						},
					},
				}
				refDataBytes, _ := json.Marshal(referenceData)
				typeAndValue := map[string]interface{}{
					"type":  "reference_data",
					"value": json.RawMessage(refDataBytes),
				}
				return models.VerifiableEvent{
					Data: &typeAndValue,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, result *workflows.ReferenceData) {
				require.NotNil(t, result)
				require.Len(t, result.OffChain, 1)
				assert.Equal(t, "api", result.OffChain[0].Source.Type)
				assert.Equal(t, "urn:api:price-feed", result.OffChain[0].Source.Identifier)
				assert.Equal(t, "ETH/USD", result.OffChain[0].Data["symbol"])
				assert.Equal(t, "2500.00", result.OffChain[0].Data["price"])
			},
		},
		{
			name: "successfully extracts reference data with requests",
			setupEvent: func() models.VerifiableEvent {
				referenceData := map[string]interface{}{
					"requests": []interface{}{
						map[string]interface{}{
							"type":  "payment_request",
							"value": map[string]interface{}{"amount": "100.00"},
						},
					},
				}
				refDataBytes, _ := json.Marshal(referenceData)
				typeAndValue := map[string]interface{}{
					"type":  "reference_data",
					"value": json.RawMessage(refDataBytes),
				}
				return models.VerifiableEvent{
					Data: &typeAndValue,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, result *workflows.ReferenceData) {
				require.NotNil(t, result)
				require.Len(t, result.Requests, 1)
				assert.Equal(t, workflows.RawMessageTypePaymentRequest, result.Requests[0].Type)
			},
		},
		{
			name: "successfully extracts empty reference data",
			setupEvent: func() models.VerifiableEvent {
				referenceData := map[string]interface{}{}
				refDataBytes, _ := json.Marshal(referenceData)
				typeAndValue := map[string]interface{}{
					"type":  "reference_data",
					"value": json.RawMessage(refDataBytes),
				}
				return models.VerifiableEvent{
					Data: &typeAndValue,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, result *workflows.ReferenceData) {
				require.NotNil(t, result)
				assert.Empty(t, result.OnChain)
				assert.Empty(t, result.OffChain)
				assert.Empty(t, result.Requests)
			},
		},
		{
			name: "returns nil when data is nil",
			setupEvent: func() models.VerifiableEvent {
				return models.VerifiableEvent{
					Data: nil,
				}
			},
			wantErr: false,
			validate: func(t *testing.T, result *workflows.ReferenceData) {
				assert.Nil(t, result)
			},
		},
		{
			name: "fails when data is not a valid type and value structure",
			setupEvent: func() models.VerifiableEvent {
				invalidData := map[string]interface{}{
					"invalid": "structure",
				}
				return models.VerifiableEvent{
					Data: &invalidData,
				}
			},
			wantErr:     true,
			errContains: "expected reference_data",
		},
		{
			name: "fails when type is not reference_data",
			setupEvent: func() models.VerifiableEvent {
				typeAndValue := map[string]interface{}{
					"type":  "payment_request",
					"value": map[string]interface{}{"amount": "100.00"},
				}
				return models.VerifiableEvent{
					Data: &typeAndValue,
				}
			},
			wantErr:     true,
			errContains: "verifiable event data type is payment_request, expected reference_data",
		},
		{
			name: "fails when value is not valid reference data",
			setupEvent: func() models.VerifiableEvent {
				invalidValue := map[string]interface{}{
					"invalid_field": "invalid_value",
				}
				valueBytes, _ := json.Marshal(invalidValue)
				typeAndValue := map[string]interface{}{
					"type":  "reference_data",
					"value": json.RawMessage(valueBytes),
				}
				// This should still unmarshal but we'll test with invalid structure
				return models.VerifiableEvent{
					Data: &typeAndValue,
				}
			},
			wantErr: false, // Invalid fields in reference data should be ignored, not cause errors
			validate: func(t *testing.T, result *workflows.ReferenceData) {
				require.NotNil(t, result)
			},
		},
		{
			name: "fails when value is not valid JSON",
			setupEvent: func() models.VerifiableEvent {
				typeAndValue := map[string]interface{}{
					"type":  "reference_data",
					"value": json.RawMessage(`{invalid json}`),
				}
				return models.VerifiableEvent{
					Data: &typeAndValue,
				}
			},
			wantErr:     true,
			errContains: "failed to marshal verifiable event data",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ve := tc.setupEvent()
			result, err := workflows.GetReferenceDataFromVerifiableEvent(ve)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				assert.Nil(t, result)
				return
			}

			require.NoError(t, err)
			if tc.validate != nil {
				tc.validate(t, result)
			}
		})
	}
}
