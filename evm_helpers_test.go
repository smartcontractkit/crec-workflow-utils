package workflows_test

import (
	"testing"

	"github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	workflows "github.com/smartcontractkit/cre-workflow-utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEVMHelpers_PBToUint64(t *testing.T) {
	testCases := []struct {
		name     string
		input    *pb.BigInt
		expected uint64
	}{
		{
			name:     "nil input returns 0",
			input:    nil,
			expected: 0,
		},
		{
			name:     "empty AbsVal returns 0",
			input:    &pb.BigInt{AbsVal: []byte{}},
			expected: 0,
		},
		{
			name:     "single byte",
			input:    &pb.BigInt{AbsVal: []byte{0x0A}},
			expected: 10,
		},
		{
			name:     "two bytes",
			input:    &pb.BigInt{AbsVal: []byte{0x01, 0x00}},
			expected: 256,
		},
		{
			name:     "three bytes",
			input:    &pb.BigInt{AbsVal: []byte{0x01, 0x00, 0x00}},
			expected: 65536,
		},
		{
			name:     "four bytes",
			input:    &pb.BigInt{AbsVal: []byte{0x00, 0x01, 0x51, 0x80}},
			expected: 86400,
		},
		{
			name:     "max uint8 byte",
			input:    &pb.BigInt{AbsVal: []byte{0xFF}},
			expected: 255,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := workflows.PBToUint64(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEVMHelpers_EnsureChainSelector(t *testing.T) {
	testCases := []struct {
		name     string
		cfg      *workflows.Config
		fallback string
		expected string
	}{
		{
			name:     "returns config selector when set",
			cfg:      &workflows.Config{ChainSelector: "11155111"},
			fallback: "fallback",
			expected: "11155111",
		},
		{
			name:     "returns fallback when selector empty",
			cfg:      &workflows.Config{ChainSelector: ""},
			fallback: "16015286601757825753",
			expected: "16015286601757825753",
		},
		{
			name:     "returns fallback when selector is zero",
			cfg:      &workflows.Config{ChainSelector: "0"},
			fallback: "42161",
			expected: "42161",
		},
		{
			name:     "returns config selector over fallback",
			cfg:      &workflows.Config{ChainSelector: "1"},
			fallback: "999",
			expected: "1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := workflows.EnsureChainSelector(tc.cfg, tc.fallback)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEVMHelpers_ParseChainSelector(t *testing.T) {
	testCases := []struct {
		name     string
		input    *string
		expected *uint64
		wantErr  bool
	}{
		{
			name:     "nil input returns nil",
			input:    nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "empty string returns nil",
			input:    ptr(""),
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "valid number returns parsed value",
			input:    ptr("11155111"),
			expected: ptrUint64(11155111),
			wantErr:  false,
		},
		{
			name:     "large chain selector",
			input:    ptr("16015286601757825753"),
			expected: ptrUint64(16015286601757825753),
			wantErr:  false,
		},
		{
			name:    "invalid string returns error",
			input:   ptr("not-a-number"),
			wantErr: true,
		},
		{
			name:    "negative number returns error",
			input:   ptr("-123"),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := workflows.ParseChainSelector(tc.input)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tc.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tc.expected, *result)
			}
		})
	}
}

func TestEVMHelpers_CursorFromPB(t *testing.T) {
	testCases := []struct {
		name        string
		blockNumber *pb.BigInt
		logIndex    uint64
		txHash      string
		expected    string
	}{
		{
			name:        "creates cursor with all fields",
			blockNumber: &pb.BigInt{AbsVal: []byte{0x64}},
			logIndex:    5,
			txHash:      "0xabc123",
			expected:    "100-5-0xabc123",
		},
		{
			name:        "uses 0x for empty txHash",
			blockNumber: &pb.BigInt{AbsVal: []byte{0x01}},
			logIndex:    0,
			txHash:      "",
			expected:    "1-0-0x",
		},
		{
			name:        "nil block number uses 0",
			blockNumber: nil,
			logIndex:    10,
			txHash:      "0xdef",
			expected:    "0-10-0xdef",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := workflows.CursorFromPB(tc.blockNumber, tc.logIndex, tc.txHash)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEVMHelpers_TxHashFromLog(t *testing.T) {
	testCases := []struct {
		name     string
		log      *evm.Log
		expected string
	}{
		{
			name:     "nil log returns 0x",
			log:      nil,
			expected: "0x",
		},
		{
			name:     "empty TxHash returns 0x",
			log:      &evm.Log{TxHash: []byte{}},
			expected: "0x",
		},
		{
			name:     "short TxHash returns 0x",
			log:      &evm.Log{TxHash: make([]byte, 20)},
			expected: "0x",
		},
		{
			name:     "valid 32-byte TxHash returns hex",
			log:      &evm.Log{TxHash: make([]byte, 32)},
			expected: "0x0000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "non-zero TxHash returns correct hex",
			log: &evm.Log{TxHash: func() []byte {
				b := make([]byte, 32)
				b[31] = 0x01
				return b
			}()},
			expected: "0x0000000000000000000000000000000000000000000000000000000000000001",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := workflows.TxHashFromLog(tc.log)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEVMHelpers_NewEVMLogFilter(t *testing.T) {
	t.Run("creates filter with correct structure for multiple events", func(t *testing.T) {
		contractAddr := "0x1234567890123456789012345678901234567890"
		eventSigHash1 := []byte{0x01, 0x02, 0x03, 0x04}
		eventSigHash2 := []byte{0x05, 0x06, 0x07, 0x08}

		result := workflows.NewEVMLogFilter(contractAddr, [][]byte{eventSigHash1, eventSigHash2})

		require.NotNil(t, result)
		assert.Len(t, result.Addresses, 1)
		assert.Len(t, result.Topics, 4)
		assert.Len(t, result.Topics[0].Values, 2)
		assert.Contains(t, result.Topics[0].Values, eventSigHash1)
		assert.Contains(t, result.Topics[0].Values, eventSigHash2)
		assert.Equal(t, evm.ConfidenceLevel_CONFIDENCE_LEVEL_FINALIZED, result.Confidence)
	})
}

func ptrUint64(v uint64) *uint64 {
	return &v
}

