package workflows

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type RawMessageType string

const (
	RawMessageTypeMap RawMessageType = "map"
)

const (
	RawMessageTypePaymentRequest RawMessageType = "payment_request"
	RawMessageTypeReferenceData  RawMessageType = "reference_data"
)

// TypeAndValue is a type that holds a type and a value.
type TypeAndValue struct {
	Type  RawMessageType  `json:"type"`
	Value json.RawMessage `json:"value"`
}

// ReferenceData is a structured set of fields that can be used for reference data from on-chain and off-chain sources
// as well as requests to be forwarded to off-chain applications.
type ReferenceData struct {
	OnChain  []OnChainReferenceData  `json:"on_chain,omitempty"`  // The on-chain reference data.
	OffChain []OffChainReferenceData `json:"off_chain,omitempty"` // The off-chain reference data.
	Requests []TypeAndValue          `json:"requests,omitempty"`  // The requests to be forwarded to the off-chain applications.
}

// OnChainReferenceData is a structured set of fields that can be used for reference data from on-chain sources.
type OnChainReferenceData struct {
	Source OnChainReferenceDataSource `json:"source"` // The source of the on-chain reference data.
	Data   map[string]any             `json:"data"`   // The data returned from the source call.
}

// OnChainReferenceDataSource is the source of the on-chain reference data.
type OnChainReferenceDataSource struct {
	ContractAddress           string `json:"contract_address"`            // The contract address to call.
	ContractFunctionSignature string `json:"contract_function_signature"` // The function signature to call.
	CallData                  string `json:"call_data"`                   // The call data to pass to the function.
	Block                     string `json:"block"`                       // The block number to call the function at.
}

// OffChainReferenceData is a structured set of fields that can be used for reference data from off-chain sources.
type OffChainReferenceData struct {
	Source OffChainReferenceDataSource `json:"source"` // The source of the off-chain reference data.
	Data   map[string]any              `json:"data"`   // The data returned from the source call.
}

// OffChainReferenceDataSource is the source of the off-chain reference data.
type OffChainReferenceDataSource struct {
	Type       string `json:"type"`       // The type of the off-chain reference data.
	Identifier string `json:"identifier"` // Typically the URN for the off-chain source or identifier of the standardised data format.
}

// PaymentRequest contains the details needed for an off-chain payment request.
type PaymentRequest struct {
	ApplicationType string           `json:"applicationType"`          // The type of the application that generates the payment request.
	ApplicationAddr string           `json:"applicationAddr"`          // The application that generates the payment request.
	E2EID           string           `json:"e2eId"`                    // The E2E ID of the payment request.
	Sender          string           `json:"sender"`                   // The sender of the payment request.
	Receiver        string           `json:"receiver"`                 // The receiver of the payment.
	Currency        string           `json:"currency"`                 // The currency of the payment.
	ChainID         string           `json:"chainId"`                  // The chain ID of the payment.
	Amount          Fixed2           `json:"amount"`                   // The amount of the payment in fixed-point decimal format with 2 decimal places.
	Expiration      *int64           `json:"expiration,omitempty"`     // The expiration time of the payment request in seconds since epoch.
	CustomCallback  *PaymentCallback `json:"customCallback,omitempty"` // The custom callback to be used for the payment request.
}

// Fixed2 represents a fixed-point decimal number with 2 decimal places, stored as a string.
type Fixed2 float64

// MarshalJSON marshals the Fixed2 value to a JSON string.
func (f Fixed2) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, "%.2f", f), nil
}

// UnmarshalJSON unmarshals the Fixed2 value from a JSON string.
func (f *Fixed2) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as a float64
	var num float64
	err := json.Unmarshal(data, &num)
	if err == nil {
		*f = Fixed2(num)
		return nil
	}

	// Fallback: try as a quoted string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("Fixed2: invalid JSON input: %w", err)
	}
	parsed, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("Fixed2: cannot parse string value: %w", err)
	}
	*f = Fixed2(parsed)
	return nil
}

// PaymentCallback specifies how to call back to the application after payment processing.
type PaymentCallback struct {
	ContractAddress   string `json:"contractAddress,omitempty"`   // The contract address to call. If empty, uses ApplicationAddr from PaymentRequest.
	FunctionName      string `json:"functionName,omitempty"`      // The name of the function to call.
	FunctionSignature string `json:"functionSignature,omitempty"` // The ABI function signature to call (e.g., "fulfillPayment(bytes32,uint256)")
}

// ComposeWorkflowEventMetadata builds a "workflowEvent" metadata payload similar to the
// original event-listener-dta workflow, carrying human-friendly attributes.
func ComposeWorkflowEventMetadata(component, chainID, eventType string, params map[string]any) map[string]any {
	// Build attributes map[string]map[string]any
	attrs := map[string]map[string]any{
		"chain_id": {
			"key":        "chain_id",
			"value":      chainID,
			"on_chain":   true,
			"visibility": "info",
		},
		"event_type": {
			"key":        "event_type",
			"value":      eventType,
			"on_chain":   true,
			"visibility": "info",
		},
	}
	for k, v := range params {
		attrs[k] = map[string]any{
			"key":        k,
			"value":      fmt.Sprint(v),
			"on_chain":   true,
			"visibility": "info",
		}
	}

	// process label: keep last segment of component (e.g. event-listener-dta -> dta)
	label := component
	if parts := strings.Split(component, "-"); len(parts) > 0 {
		label = parts[len(parts)-1]
	}

	return map[string]any{
		"chainId": chainID, // keep exactly as before; network is provided separately
		"network": "evm",
		"workflowEvent": map[string]any{
			"attributes":       attrs,
			"component":        component,
			"event_type_label": eventType,
			"process_labels":   []string{strings.ToLower(label), eventType},
		},
	}
}
