package workflows

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	httpcap "github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http"
	httpmock "github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http/mock"
	"github.com/smartcontractkit/cre-sdk-go/cre/testutils"
	"github.com/stretchr/testify/require"
)

const testABIForCommon = `[
  {"type":"event","name":"Sender","inputs":[{"name":"sender","type":"address","indexed":true,"internalType":"address"}],"anonymous":false}
]`

func TestParseCursor(t *testing.T) {
	ci, err := ParseCursor("100-2-0xabc")
	require.NoError(t, err)
	require.Equal(t, uint64(100), ci.BlockNumber)
	require.Equal(t, uint64(2), ci.LogIndex)
	require.Equal(t, "0xabc", ci.TxHash)

	_, err = ParseCursor("bad-cursor")
	require.Error(t, err)
}

func TestBuildAndHashEventEnvelope_WithNilService(t *testing.T) {
	// prepare parameters
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	raw := map[string]any{
		"Sender": addr.Bytes(),
	}
	params := SanitiseJSON(raw).(map[string]any)

	// Build event envelope with nil service
	res, err := BuildAndHashEventEnvelope(
		nil,
		"Sender",
		"0xContract",
		testABIForCommon,
		"1",
		100,
		2,
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		1_700_000_000,
		params,
		map[string]any{"extra": "meta"},
	)
	require.NoError(t, err)
	require.NotEmpty(t, res.Base64Event)
	// When service is nil, typeName should be just the event name (no service prefix)
	require.Equal(t, "Sender", res.Type)
	require.NotEqual(t, common.Hash{}, res.EventHash)

	decoded, err := base64.StdEncoding.DecodeString(res.Base64Event)
	require.NoError(t, err)
	var obj VerifiableEvent
	require.NoError(t, json.Unmarshal(decoded, &obj))

	ev := obj.Event
	// service field should not be present when service is nil
	require.Empty(t, obj.Domain, "domain should be empty when service is nil")
	require.Equal(t, "Sender", ev.EventName, "event name should be Sender")
	require.Equal(t, "0xContract", ev.ContractAddress, "contract address should be 0xContract")
}

func TestBuildAndHashEventEnvelope_ServiceHashCompatibility(t *testing.T) {
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	raw := map[string]any{
		"Sender": addr.Bytes(),
	}
	params := SanitiseJSON(raw).(map[string]any)
	metadata := map[string]any{"extra": "meta"}

	eventName := "Sender"

	resNilService, err := BuildAndHashEventEnvelope(
		nil,
		eventName,
		"0xContract",
		testABIForCommon,
		"1",
		100,
		2,
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		1_700_000_000,
		params,
		metadata,
	)
	require.NoError(t, err)

	testService := "operations"
	resWithService, err := BuildAndHashEventEnvelope(
		&testService,
		eventName,
		"0xContract",
		testABIForCommon,
		"1",
		100,
		2,
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		1_700_000_000,
		params,
		metadata,
	)
	require.NoError(t, err)

	require.Equal(t, eventName, resNilService.Type, "nil service should produce typeName = eventName")
	require.Equal(t, "operations."+eventName, resWithService.Type, "service should produce typeName = service.eventName")

	// Note: base64 payloads differ because service is included in the event JSON when present
	// This is expected behavior - the service field is part of the verifiable event structure
	require.NotEqual(t, resNilService.Base64Event, resWithService.Base64Event,
		"base64 verifiable event payloads differ because service is included in JSON when present")

	require.NotEqual(t, resNilService.EventHash, resWithService.EventHash,
		"event hashes must differ when service is nil vs present (compatibility boundary)")

	expectedNilHash := common.BytesToHash(crypto.Keccak256([]byte(eventName + "." + resNilService.Base64Event)))
	expectedServiceHash := common.BytesToHash(crypto.Keccak256([]byte("operations." + eventName + "." + resWithService.Base64Event)))

	require.Equal(t, expectedNilHash, resNilService.EventHash,
		"nil-service hash should match keccak256(eventName + \".\" + base64payload)")
	require.Equal(t, expectedServiceHash, resWithService.EventHash,
		"service-prefixed hash should match keccak256(service.eventName + \".\" + base64payload)")
}

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

func TestPostSignedEvent_HTTPPayloadStructure(t *testing.T) {
	// Provide a runtime with a preloaded secret for the API key.
	// Note: secrets are stored under empty namespace when GetSecret is called without one.
	rt := testutils.NewRuntime(t, testutils.Secrets{
		"": map[testutils.ID]string{
			"courier": "API-KEY",
		},
	})
	// HTTP capability mock to capture POST payload
	httpCap, err := httpmock.NewClientCapability(t)
	require.NoError(t, err)
	httpCap.SendRequest = func(_ context.Context, req *httpcap.Request) (*httpcap.Response, error) {
		// headers
		require.Equal(t, "application/json", req.Headers["Content-Type"])
		require.Equal(t, "API-KEY", req.Headers["Api-Key"])
		require.Equal(t, "http://example.com/system/onchain-watcher-events", req.Url)
		require.Equal(t, "POST", req.Method)

		// JSON body
		var body map[string]any
		require.NoError(t, json.Unmarshal(req.Body, &body))

		// required fields
		require.Equal(t, "test", body["domain"])
		require.Equal(t, "Sender", body["name"])
		require.Equal(t, "11155111", body["chain_selector"], "chain_selector should be a string")
		require.Equal(t, "0xABCDEF", body["address"])

		// ocr report/context hex encoded
		ocrCtx := body["ocr_context"].(string)
		ocrRpt := body["ocr_report"].(string)
		require.Equal(t, "0x", ocrCtx[:2])
		require.Equal(t, "0x", ocrRpt[:2])
		_, err := hex.DecodeString(ocrCtx[2:])
		require.NoError(t, err)
		_, err = hex.DecodeString(ocrRpt[2:])
		require.NoError(t, err)

		// signatures present
		sigs := body["signatures"].([]any)
		require.GreaterOrEqual(t, len(sigs), 1)
		for _, s := range sigs {
			str := s.(string)
			require.Greater(t, len(str), 2)
			require.Equal(t, "0x", str[:2])
		}

		// verifiable_event matches provided one (checked below)
		require.NotEmpty(t, body["verifiable_event"])
		return &httpcap.Response{StatusCode: 200}, nil
	}

	// Workflow config for POST (use secret id, not inline key)
	testService := "test"
	cfg := &Config{
		Network:       "evm",
		ChainID:       "1",
		ChainSelector: "11155111", // Provide explicit selector
		CourierURL:    "http://example.com",
		Service:       &testService,
		ApiKeySecret:  "courier",
		DetectEventTriggerConfig: DetectEventTriggerConfig{
			ContractName: "TestConsumer",
		},
	}

	// Build a pre-consensus verifiable event
	params := SanitiseJSON(map[string]any{
		"Sender": common.HexToAddress("0x1111111111111111111111111111111111111111").Bytes(),
	}).(map[string]any)
	pre, err := BuildAndHashEventEnvelope(
		&testService,
		"Sender",
		"0xABCDEF",
		testABIForCommon,
		"1",
		100,
		2,
		"0xaaaa",
		4321,
		params,
		map[string]any{},
	)
	require.NoError(t, err)
	require.NotEmpty(t, pre.Base64Event)

	// Post to courier
	out, err := PostSignedEvent(cfg, rt, "Sender", "0xABCDEF", pre)
	require.NoError(t, err)
	require.Equal(t, pre.Base64Event, out)

	// sanity: verify the embedded verifiable_event decodes
	decoded, err := base64.StdEncoding.DecodeString(out)
	require.NoError(t, err)
	var obj VerifiableEvent
	require.NoError(t, json.Unmarshal(decoded, &obj))
	ev := obj.Event
	require.Equal(t, &testService, obj.Domain, "domain should be test")
	require.Equal(t, "Sender", ev.EventName, "event name should be Sender")
	require.Equal(t, "0xABCDEF", ev.ContractAddress, "contract address should be 0xABCDEF")
	require.Nil(t, obj.ReferenceData, "reference data should be nil")
}

func TestPostSignedEvent_ChainSelectorEnsuredString(t *testing.T) {
	// This test verifies that even if ChainSelector looks like a number in the struct
	// (it is string in Config, but we simulate potential regression or data oddities)
	// it is serialized as a string in the JSON payload sent to Courier.

	rt := testutils.NewRuntime(t, testutils.Secrets{
		"": map[testutils.ID]string{"courier": "API-KEY"},
	})
	httpCap, err := httpmock.NewClientCapability(t)
	require.NoError(t, err)
	httpCap.SendRequest = func(_ context.Context, req *httpcap.Request) (*httpcap.Response, error) {
		var body map[string]any
		require.NoError(t, json.Unmarshal(req.Body, &body))
		// Strictly check type is string
		_, ok := body["chain_selector"].(string)
		require.True(t, ok, "chain_selector in JSON body must be a string")
		require.Equal(t, "16015286601757825753", body["chain_selector"])
		return &httpcap.Response{StatusCode: 200}, nil
	}

	testService := "test"
	cfg := &Config{
		Network: "evm",
		ChainID: "1",
		// Large number as string.
		// Even if the struct field was uint64 (simulated regression), the fix in PostSignedEvent ensures string.
		ChainSelector: "16015286601757825753",
		CourierURL:    "http://example.com",
		Service:       &testService,
		ApiKeySecret:  "courier",
	}

	pre, _ := BuildAndHashEventEnvelope(&testService, "Sender", "0xABC", testABIForCommon, "1", 1, 1, "0x", 123, nil, nil)
	_, err = PostSignedEvent(cfg, rt, "Sender", "0xABC", pre)
	require.NoError(t, err)
}
