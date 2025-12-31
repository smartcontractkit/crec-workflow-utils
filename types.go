package workflows

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	gethCommon "github.com/ethereum/go-ethereum/common"
)

const (
	PaymentRequestType = "payment_request"
)

// CursorInfo contains parsed info from a "block-logIndex-txHash" string.
type CursorInfo struct {
	BlockNumber uint64
	LogIndex    uint64
	TxHash      string
}

type VerifiableEvent struct {
	Domain        string        `json:"domain"`
	Event         Event         `json:"event"`
	ReferenceData ReferenceData `json:"reference_data"`
	Trigger       Trigger       `json:"trigger"`
}

type Trigger struct {
	ChainID  string `json:"chain_id"`
	LogIndex uint64 `json:"log_index"`
	TxHash   string `json:"tx_hash"`
}

type Event struct {
	BlockNumber     uint64         `json:"block_number"`
	BlockTimestamp  time.Time      `json:"block_timestamp"`
	ContractAddress string         `json:"contract_address"`
	EventName       string         `json:"event_name"`
	EventSignature  string         `json:"event_signature"`
	TopicHash       string         `json:"topic_hash"`
	Args            map[string]any `json:"args"`
}

type ReferenceData struct {
	OnChain  []OnChainReferenceData  `json:"on_chain,omitempty"`
	OffChain []OffChainReferenceData `json:"off_chain,omitempty"`
	Requests []OffChainRequest       `json:"requests,omitempty"`
}

type OnChainReferenceData struct {
	Source OnChainReferenceDataSource `json:"source"`
	Data   map[string]any             `json:"data"`
}

type OnChainReferenceDataSource struct {
	ContractAddress           string `json:"contract_address"`
	ContractFunctionSignature string `json:"contract_function_signature"`
	CallData                  string `json:"call_data"`
	Block                     string `json:"block"`
}

type OffChainReferenceData struct {
	Source OffChainReferenceDataSource `json:"source"`
	Data   map[string]any              `json:"data"`
}

type OffChainReferenceDataSource struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}

// OffChainRequest represents a request to be forwarded to an off-chain application.
// The Type field determines which request type is contained in the Request field.
// Use json.RawMessage to allow consumers to unmarshal the Request based on Type.
type OffChainRequest struct {
	Type    string          `json:"type" example:"payment"`
	Request json.RawMessage `json:"request"`
}

// Fixed2 represents a fixed-point decimal number with 2 decimal places, stored as a string.
// Example: "100.00" represents 100.00
type Fixed2 float64

func (f Fixed2) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, "%.2f", f), nil
}

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

// PaymentRequest contains the details needed for an off-chain payment request.
type PaymentRequest struct {
	ApplicationType string `json:"applicationType" example:"DVP"`
	ApplicationAddr string `json:"applicationAddr" example:"0xApplicationAddress"`
	E2EID           string `json:"e2eId" example:"E2E1234"`
	Sender          string `json:"sender" example:"0xSenderAddress"`
	Receiver        string `json:"receiver" example:"0xReceiverAddress"`
	Currency        string `json:"currency" example:"USD"`
	ChainID         string `json:"chainId" example:"1337"`
	Amount          Fixed2 `json:"amount" example:"100.00"`
	Expiration      *int64 `json:"expiration,omitempty" example:"1257894000"`
	// Callback specifies how the off-chain payment handler should call back to the application.
	// This can be a function signature (e.g., "fulfillPayment(bytes32,uint256)") or
	// a more detailed callback specification.
	CustomCallback *PaymentCallback `json:"customCallback,omitempty"`
}

// PaymentCallback specifies how to call back to the application after payment processing.
type PaymentCallback struct {
	// FunctionSignature is the ABI function signature to call (e.g., "fulfillPayment(bytes32,uint256)")
	FunctionSignature string `json:"functionSignature,omitempty" example:"fulfillPayment(bytes32,uint256)"`
	// ContractAddress is the contract address to call. If empty, uses ApplicationAddr from PaymentRequest.
	ContractAddress string `json:"contractAddress,omitempty" example:"0xContractAddress"`
}

// PreConsensusEventResults holds the encoded event and hash used for consensus.
type PreConsensusEventResults struct {
	Base64Event    string
	Type           string
	EventHash      gethCommon.Hash
	BlockTimestamp uint64
}
