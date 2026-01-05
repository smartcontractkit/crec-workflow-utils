package workflows

import (
	"encoding/json"
	"time"

	gethCommon "github.com/ethereum/go-ethereum/common"
)

type RawMessageType string

const (
	RawMessageTypeMap RawMessageType = "map"
)

// CursorInfo contains parsed info from a "block-logIndex-txHash" string.
type CursorInfo struct {
	BlockNumber uint64
	LogIndex    uint64
	TxHash      string
}

// TypeAndValue is a type that holds a type and a value.
type TypeAndValue struct {
	Type  RawMessageType  `json:"type"`
	Value json.RawMessage `json:"value"`
}

// VerifiableEvent is the core structure for verifiable events.
type VerifiableEvent struct {
	Domain        *string       `json:"domain,omitempty"`
	Event         Event         `json:"event"`
	ReferenceData *TypeAndValue `json:"reference_data,omitempty"`
	Trigger       Trigger       `json:"trigger"`
}

// Trigger is the information about the trigger of the event.
type Trigger struct {
	ChainID  string `json:"chain_id"`
	LogIndex uint64 `json:"log_index"`
	TxHash   string `json:"tx_hash"`
}

// Event is the information about the event.
type Event struct {
	BlockNumber     uint64         `json:"block_number"`
	BlockTimestamp  time.Time      `json:"block_timestamp"`
	ContractAddress string         `json:"contract_address"`
	EventName       string         `json:"event_name"`
	EventSignature  string         `json:"event_signature"`
	TopicHash       string         `json:"topic_hash"`
	Args            map[string]any `json:"args"`
}

// VerifiableEventEvelope holds the encoded event and hash used for consensus.
type VerifiableEventEvelope struct {
	Base64Event    string
	Type           string
	EventHash      gethCommon.Hash
	BlockTimestamp uint64
}
