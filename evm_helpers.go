package workflows

import (
	"fmt"
	"strconv"
	"strings"

	gethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	"github.com/smartcontractkit/cre-sdk-go/cre"
)

// PBToUint64 converts a protobuf BigInt (unsigned) to a uint64.
func PBToUint64(b *pb.BigInt) uint64 {
	if b == nil || len(b.AbsVal) == 0 {
		return 0
	}
	// Sign is ignored as we only expect non-negative block numbers
	u := uint64(0)
	for _, v := range b.AbsVal {
		u = u<<8 | uint64(v)
	}
	return u
}

func ConfidenceLevelFromString(s string) evm.ConfidenceLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "finalized":
		return evm.ConfidenceLevel_CONFIDENCE_LEVEL_FINALIZED
	case "safe":
		return evm.ConfidenceLevel_CONFIDENCE_LEVEL_SAFE
	case "latest":
		return evm.ConfidenceLevel_CONFIDENCE_LEVEL_LATEST
	default:
		return evm.ConfidenceLevel_CONFIDENCE_LEVEL_LATEST
	}
}

// NewEVMLogFilter returns a FilterLogTriggerRequest for a single-address subscription for one or more events.
// Includes wildcard slots for up to 3 indexed parameters. Confidence is chosen by the caller.
func NewEVMLogFilter(contractAddr string, eventSigHashes [][]byte, confidence evm.ConfidenceLevel) *evm.FilterLogTriggerRequest {
	return &evm.FilterLogTriggerRequest{
		Addresses: [][]byte{
			gethCommon.HexToAddress(contractAddr).Bytes(),
		},
		Topics: []*evm.TopicValues{
			{Values: eventSigHashes}, // Topic 0: Event signatures (OR logic if multiple)
			{},                       // Topic 1 (indexed param wildcard)
			{},                       // Topic 2 (indexed param wildcard)
			{},                       // Topic 3 (indexed param wildcard)
		},
		Confidence: confidence,
	}
}

// EnsureChainSelector returns cfg.ChainSelector if set, otherwise returns the provided fallback.
func EnsureChainSelector(cfg *Config, fallback string) string {
	if cfg.ChainSelector != "" && cfg.ChainSelector != "0" {
		return cfg.ChainSelector
	}
	return fallback
}

// ParseChainSelector validates and parses a chain_selector string parameter to uint64.
// Returns the parsed value or an error if the string is invalid.
func ParseChainSelector(chainSelectorStr *string) (*uint64, error) {
	if chainSelectorStr == nil || *chainSelectorStr == "" {
		return nil, nil
	}

	chainSelector, err := strconv.ParseUint(*chainSelectorStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chain_selector: must be a valid uint64")
	}

	return &chainSelector, nil
}

// GetBlockTimestamp fetches the block timestamp via EVM HeaderByNumber.
// It returns an error if the block number is nil or the header cannot be fetched,
// rather than falling back to wall-clock time which would break cross-node consensus.
func GetBlockTimestamp(rt cre.Runtime, chainSelector string, blockNumber *pb.BigInt) (uint64, error) {
	if blockNumber == nil {
		return 0, fmt.Errorf("block number is nil")
	}
	chainSelectorUint64, err := ParseChainSelector(&chainSelector)
	if err != nil {
		return 0, fmt.Errorf("invalid chain selector for timestamp lookup: %w", err)
	}
	if chainSelectorUint64 == nil {
		return 0, fmt.Errorf("chain selector is empty")
	}

	cli := &evm.Client{ChainSelector: *chainSelectorUint64}
	hdr, err := cli.HeaderByNumber(rt, &evm.HeaderByNumberRequest{BlockNumber: blockNumber}).Await()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch block header: %w", err)
	}
	if hdr == nil || hdr.Header == nil {
		return 0, fmt.Errorf("block header is nil for block %d", PBToUint64(blockNumber))
	}
	return hdr.Header.Timestamp, nil
}

// CursorFromPB builds a "block-logIndex-txHash" cursor string from pb.BigInt block-number, a log-index,
// and an optional tx-hash. If txHash is empty, "0x" is used to match existing consumers.
func CursorFromPB(blockNumber *pb.BigInt, logIndex uint64, txHash string) string {
	if txHash == "" {
		txHash = "0x"
	}
	return fmt.Sprintf("%d-%d-%s", PBToUint64(blockNumber), logIndex, txHash)
}

// TxHashFromLog returns the 0x-hex transaction hash from a log if present; otherwise "0x".
func TxHashFromLog(l *evm.Log) string {
	if l == nil || len(l.TxHash) != 32 {
		return "0x"
	}
	return gethCommon.BytesToHash(l.TxHash).Hex()
}
