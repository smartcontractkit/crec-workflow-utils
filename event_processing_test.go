package workflows

import (
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	httpcap "github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testABIForCommon = `[
  {"type":"event","name":"Sender","inputs":[{"name":"sender","type":"address","indexed":true,"internalType":"address"}],"anonymous":false}
]`

func TestSanitiseJSON_Conversions(t *testing.T) {
	// prepare a structure with varied types
	b20 := make([]byte, 20) // 20 bytes -> address-like
	b32 := make([]byte, 32) // 32 bytes -> bytes32-like
	b64Addr := base64.StdEncoding.EncodeToString(b20)
	b64Bytes := base64.StdEncoding.EncodeToString(b32)

	raw := map[string]any{
		"Address":     b20,
		"Bytes32":     b32,
		"Base64Addr":  b64Addr,
		"Base64Bytes": b64Bytes,
		"Big":         big.NewInt(1234567890),
		"Bool":        true,
		"SmallInt":    42,
		// Also simulate JSON-decoded numeric arrays for 20/32 bytes
		"Arr20": func() []any {
			a := make([]any, 20)
			for i := range a {
				a[i] = float64(i)
			}
			return a
		}(),
		"Arr32": func() []any {
			a := make([]any, 32)
			for i := range a {
				a[i] = float64(i)
			}
			return a
		}(),
	}

	out := SanitiseJSON(raw).(map[string]any)

	// []byte -> hex
	require.Regexp(t, `^0x[0-9a-f]{40}$`, out["address"].(string))
	require.Regexp(t, `^0x[0-9a-f]{64}$`, out["bytes32"].(string))

	// base64 -> hex (20 or 32 bytes only)
	require.Regexp(t, `^0x[0-9a-f]{40}$`, out["base64_addr"].(string))
	require.Regexp(t, `^0x[0-9a-f]{64}$`, out["base64_bytes"].(string))

	// numeric []interface{} -> hex (for 20/32 lengths)
	require.Regexp(t, `^0x[0-9a-f]{40}$`, out["arr20"].(string))
	require.Regexp(t, `^0x[0-9a-f]{64}$`, out["arr32"].(string))

	// big.Int -> string
	require.Equal(t, "1234567890", out["big"])

	// bool preserved
	require.Equal(t, true, out["bool"])

	// ints preserved as numbers
	require.Equal(t, 42, out["small_int"])
}

func TestSanitiseJSON_AdditionalCases(t *testing.T) {
	t.Run("handles nil big.Int", func(t *testing.T) {
		var nilBig *big.Int
		result := SanitiseJSON(nilBig)
		assert.Nil(t, result)
	})

	t.Run("handles geth address", func(t *testing.T) {
		addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		result := SanitiseJSON(addr)
		assert.Equal(t, "0x1234567890123456789012345678901234567890", result)
	})

	t.Run("handles fixed size byte arrays", func(t *testing.T) {
		var arr32 [32]byte
		for i := range arr32 {
			arr32[i] = byte(i)
		}
		result := SanitiseJSON(arr32)
		assert.Regexp(t, `^0x[0-9a-f]{64}$`, result)
	})

	t.Run("handles nested maps", func(t *testing.T) {
		nested := map[string]any{
			"OuterKey": map[string]any{
				"InnerKey": "value",
			},
		}
		result := SanitiseJSON(nested).(map[string]any)
		inner := result["outer_key"].(map[string]any)
		assert.Equal(t, "value", inner["inner_key"])
	})

	t.Run("handles slice of mixed types", func(t *testing.T) {
		slice := []any{"string", 123, true}
		result := SanitiseJSON(slice).([]any)
		assert.Equal(t, "string", result[0])
		assert.Equal(t, 123, result[1])
		assert.Equal(t, true, result[2])
	})

	t.Run("preserves various int types", func(t *testing.T) {
		result := SanitiseJSON(int64(999))
		assert.Equal(t, int64(999), result)

		result = SanitiseJSON(uint64(888))
		assert.Equal(t, uint64(888), result)

		result = SanitiseJSON(float64(1.5))
		assert.Equal(t, float64(1.5), result)
	})

	t.Run("non-byte array slice not converted", func(t *testing.T) {
		arr15 := make([]any, 15)
		for i := range arr15 {
			arr15[i] = float64(i)
		}
		result := SanitiseJSON(arr15).([]any)
		assert.Len(t, result, 15)
		assert.Equal(t, float64(0), result[0])
	})

	t.Run("returns non-base64 string unchanged", func(t *testing.T) {
		result := SanitiseJSON("hello world")
		assert.Equal(t, "hello world", result)
	})

	t.Run("returns base64 of wrong length unchanged", func(t *testing.T) {
		b10 := make([]byte, 10)
		encoded := base64.StdEncoding.EncodeToString(b10)
		result := SanitiseJSON(encoded)
		assert.Equal(t, encoded, result)
	})
}

func TestEventProcessing_CheckResponse(t *testing.T) {
	testCases := []struct {
		name        string
		resp        *httpcap.Response
		wantErr     bool
		errContains string
	}{
		{
			name:    "returns nil for nil response",
			resp:    nil,
			wantErr: true,
		},
		{
			name:        "returns error for 400 status",
			resp:        &httpcap.Response{StatusCode: 400},
			wantErr:     true,
			errContains: "400",
		},
		{
			name:        "returns error for 500 status",
			resp:        &httpcap.Response{StatusCode: 500},
			wantErr:     true,
			errContains: "500",
		},
		{
			name:        "returns error for 404 status",
			resp:        &httpcap.Response{StatusCode: 404},
			wantErr:     true,
			errContains: "404",
		},
		{
			name:    "passes through 200 response",
			resp:    &httpcap.Response{StatusCode: 200},
			wantErr: false,
		},
		{
			name:    "passes through 201 response",
			resp:    &httpcap.Response{StatusCode: 201},
			wantErr: false,
		},
		{
			name:    "passes through 204 response",
			resp:    &httpcap.Response{StatusCode: 204},
			wantErr: false,
		},
		{
			name:    "passes through 399 response",
			resp:    &httpcap.Response{StatusCode: 399},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := CheckResponse(tc.resp)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.resp, result)
		})
	}
}

func TestEventProcessing_MustEvent(t *testing.T) {
	t.Run("returns event for valid ABI and name", func(t *testing.T) {
		abiJSON := `[{"type":"event","name":"Transfer","inputs":[{"name":"from","type":"address","indexed":true},{"name":"to","type":"address","indexed":true},{"name":"value","type":"uint256","indexed":false}]}]`
		event := MustEvent(abiJSON, "Transfer")
		assert.Equal(t, "Transfer", event.Name)
		assert.Len(t, event.Inputs, 3)
	})

	t.Run("panics on invalid ABI JSON", func(t *testing.T) {
		assert.Panics(t, func() {
			MustEvent("not valid json", "Transfer")
		})
	})

	t.Run("panics when event not found", func(t *testing.T) {
		abiJSON := `[{"type":"event","name":"Transfer","inputs":[]}]`
		assert.Panics(t, func() {
			MustEvent(abiJSON, "NonExistent")
		})
	})
}

func TestEventProcessing_toSnakeCase(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"CamelCase", "camel_case"},
		{"alreadyLower", "already_lower"},
		{"ABC", "a_b_c"},
		{"simple", "simple"},
		{"TwoWords", "two_words"},
		{"", ""},
		{"A", "a"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := toSnakeCase(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEventProcessing_toHexIfB64(t *testing.T) {
	t.Run("converts 20-byte base64 to hex", func(t *testing.T) {
		b20 := make([]byte, 20)
		for i := range b20 {
			b20[i] = byte(i)
		}
		encoded := base64.StdEncoding.EncodeToString(b20)
		result, ok := toHexIfB64(encoded)
		assert.True(t, ok)
		assert.Regexp(t, `^0x[0-9a-f]{40}$`, result)
	})

	t.Run("converts 32-byte base64 to hex", func(t *testing.T) {
		b32 := make([]byte, 32)
		for i := range b32 {
			b32[i] = byte(i)
		}
		encoded := base64.StdEncoding.EncodeToString(b32)
		result, ok := toHexIfB64(encoded)
		assert.True(t, ok)
		assert.Regexp(t, `^0x[0-9a-f]{64}$`, result)
	})

	t.Run("returns false for non-20/32 byte base64", func(t *testing.T) {
		b10 := make([]byte, 10)
		encoded := base64.StdEncoding.EncodeToString(b10)
		_, ok := toHexIfB64(encoded)
		assert.False(t, ok)
	})

	t.Run("returns false for invalid base64", func(t *testing.T) {
		_, ok := toHexIfB64("not-valid-base64!!!")
		assert.False(t, ok)
	})
}

func TestEventProcessing_tryByteArrayHex(t *testing.T) {
	t.Run("converts 20-element numeric array", func(t *testing.T) {
		arr := make([]any, 20)
		for i := range arr {
			arr[i] = float64(i)
		}
		result, ok := tryByteArrayHex(arr)
		assert.True(t, ok)
		assert.Regexp(t, `^0x[0-9a-f]{40}$`, result)
	})

	t.Run("converts 32-element numeric array", func(t *testing.T) {
		arr := make([]any, 32)
		for i := range arr {
			arr[i] = float64(i)
		}
		result, ok := tryByteArrayHex(arr)
		assert.True(t, ok)
		assert.Regexp(t, `^0x[0-9a-f]{64}$`, result)
	})

	t.Run("returns false for wrong length", func(t *testing.T) {
		arr := make([]any, 15)
		_, ok := tryByteArrayHex(arr)
		assert.False(t, ok)
	})

	t.Run("returns false for out-of-range values", func(t *testing.T) {
		arr := make([]any, 20)
		arr[0] = float64(300)
		_, ok := tryByteArrayHex(arr)
		assert.False(t, ok)
	})

	t.Run("returns false for negative values", func(t *testing.T) {
		arr := make([]any, 20)
		arr[0] = float64(-1)
		_, ok := tryByteArrayHex(arr)
		assert.False(t, ok)
	})

	t.Run("returns false for non-numeric types", func(t *testing.T) {
		arr := make([]any, 20)
		arr[0] = "string"
		_, ok := tryByteArrayHex(arr)
		assert.False(t, ok)
	})

	t.Run("handles int types", func(t *testing.T) {
		arr := make([]any, 20)
		for i := range arr {
			arr[i] = int(i)
		}
		result, ok := tryByteArrayHex(arr)
		assert.True(t, ok)
		assert.Regexp(t, `^0x[0-9a-f]{40}$`, result)
	})

	t.Run("handles int64 types", func(t *testing.T) {
		arr := make([]any, 20)
		for i := range arr {
			arr[i] = int64(i)
		}
		result, ok := tryByteArrayHex(arr)
		assert.True(t, ok)
		assert.Regexp(t, `^0x[0-9a-f]{40}$`, result)
	})

	t.Run("handles uint64 types", func(t *testing.T) {
		arr := make([]any, 20)
		for i := range arr {
			arr[i] = uint64(i)
		}
		result, ok := tryByteArrayHex(arr)
		assert.True(t, ok)
		assert.Regexp(t, `^0x[0-9a-f]{40}$`, result)
	})

	t.Run("handles uint8 types", func(t *testing.T) {
		arr := make([]any, 20)
		for i := range arr {
			arr[i] = uint8(i)
		}
		result, ok := tryByteArrayHex(arr)
		assert.True(t, ok)
		assert.Regexp(t, `^0x[0-9a-f]{40}$`, result)
	})

	t.Run("rejects int out of range", func(t *testing.T) {
		arr := make([]any, 20)
		arr[0] = int(300)
		_, ok := tryByteArrayHex(arr)
		assert.False(t, ok)
	})

	t.Run("rejects int64 out of range", func(t *testing.T) {
		arr := make([]any, 20)
		arr[0] = int64(-5)
		_, ok := tryByteArrayHex(arr)
		assert.False(t, ok)
	})

	t.Run("rejects uint64 out of range", func(t *testing.T) {
		arr := make([]any, 20)
		arr[0] = uint64(300)
		_, ok := tryByteArrayHex(arr)
		assert.False(t, ok)
	})
}

func TestEventProcessing_DecodeEventParams(t *testing.T) {
	transferABI := `[{"type":"event","name":"Transfer","inputs":[{"name":"from","type":"address","indexed":true},{"name":"to","type":"address","indexed":true},{"name":"value","type":"uint256","indexed":false}]}]`

	t.Run("returns error for invalid ABI JSON", func(t *testing.T) {
		log := &evm.Log{}
		_, err := DecodeEventParams("not valid json", "Transfer", log)
		require.Error(t, err)
	})

	t.Run("returns error for missing event", func(t *testing.T) {
		log := &evm.Log{}
		_, err := DecodeEventParams(transferABI, "NonExistentEvent", log)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("decodes indexed address parameters from topics", func(t *testing.T) {
		fromAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
		toAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")

		transferEventSig := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

		log := &evm.Log{
			Topics: [][]byte{
				transferEventSig.Bytes(),
				common.BytesToHash(fromAddr.Bytes()).Bytes(),
				common.BytesToHash(toAddr.Bytes()).Bytes(),
			},
			Data: func() []byte {
				amount := new(big.Int).SetUint64(1000)
				return common.LeftPadBytes(amount.Bytes(), 32)
			}(),
		}

		params, err := DecodeEventParams(transferABI, "Transfer", log)
		require.NoError(t, err)

		assert.Contains(t, params, "from")
		assert.Contains(t, params, "to")
		assert.Contains(t, params, "value")
	})

	t.Run("handles log with no data", func(t *testing.T) {
		approvalABI := `[{"type":"event","name":"Approval","inputs":[{"name":"owner","type":"address","indexed":true}]}]`
		ownerAddr := common.HexToAddress("0x3333333333333333333333333333333333333333")
		eventSig := crypto.Keccak256Hash([]byte("Approval(address)"))

		log := &evm.Log{
			Topics: [][]byte{
				eventSig.Bytes(),
				common.BytesToHash(ownerAddr.Bytes()).Bytes(),
			},
			Data: []byte{},
		}

		params, err := DecodeEventParams(approvalABI, "Approval", log)
		require.NoError(t, err)
		assert.Contains(t, params, "owner")
	})

	t.Run("handles empty log", func(t *testing.T) {
		simpleABI := `[{"type":"event","name":"Simple","inputs":[]}]`
		log := &evm.Log{Topics: [][]byte{}, Data: []byte{}}

		params, err := DecodeEventParams(simpleABI, "Simple", log)
		require.NoError(t, err)
		assert.Empty(t, params)
	})
}
