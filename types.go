package workflows

import (
	"time"

	gethCommon "github.com/ethereum/go-ethereum/common"
)

// CursorInfo contains parsed info from a "block-logIndex-txHash" string.
type CursorInfo struct {
	BlockNumber uint64
	LogIndex    uint64
	TxHash      string
}

type VerifiableEvent struct {
	CreatedAt     time.Time     `json:"created_at"`
	Workflow      Workflow      `json:"workflow"`
	Trigger       Trigger       `json:"trigger"`
	Event         Event         `json:"event"`
	ReferenceData ReferenceData `json:"reference_data"`
}

type Workflow struct {
	WorkflowName string `json:"workflow_name"`
	WatcherID    string `json:"watcher_id"`
	DonID        string `json:"don_id"`
	Domain       string `json:"domain"`
}

type Trigger struct {
	ChainID  string `json:"chain_id"`
	TxHash   string `json:"tx_hash"`
	LogIndex uint64 `json:"log_index"`
}

type Event struct {
	EventName       string         `json:"event_name"`
	EventSignature  string         `json:"event_signature"`
	ContractAddress string         `json:"contract_address"`
	TopicHash       string         `json:"topic_hash"`
	BlockNumber     uint64         `json:"block_number"`
	BlockTimestamp  time.Time      `json:"block_timestamp"`
	Args            map[string]any `json:"args"`
}

type ReferenceData struct {
	OnChain  []OnChainReferenceData  `json:"on_chain"`
	OffChain []OffChainReferenceData `json:"off_chain"`
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

// PreConsensusEventResults holds the encoded event and hash used for consensus.
type PreConsensusEventResults struct {
	Base64Event    string
	Type           string
	EventHash      gethCommon.Hash
	BlockTimestamp uint64
}
