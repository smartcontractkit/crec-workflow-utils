package workflows

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"reflect"
	"strings"
	"time"

	gethAbi "github.com/ethereum/go-ethereum/accounts/abi"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	httpcap "github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http"
	"github.com/smartcontractkit/cre-sdk-go/cre"
)

// PreConsensusEventResults holds the encoded event and hash used for consensus.
type PreConsensusEventResults struct {
	Base64Event    string
	Type           string
	EventHash      gethCommon.Hash
	BlockTimestamp uint64
	Metadata       map[string]any
}

// toSnakeCase converts "CamelCase" -> "camel_case".
func toSnakeCase(in string) string {
	var b strings.Builder
	for i, r := range in {
		if 'A' <= r && r <= 'Z' {
			if i != 0 {
				b.WriteByte('_')
			}
			r += 32
		}
		b.WriteRune(r)
	}
	return b.String()
}

// toHexIfB64 converts 20 or 32-byte base64 into 0x-hex.
func toHexIfB64(s string) (string, bool) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", false
	}
	switch len(b) {
	case 20, 32:
		return "0x" + hex.EncodeToString(b), true
	default:
		return "", false
	}
}

// tryByteArrayHex attempts to detect if arr is a []interface{} representing a byte-array
// (sequence of numeric 0..255 values). If so, it returns a 0x-hex string. Otherwise ok=false.
func tryByteArrayHex(arr []any) (string, bool) {
	n := len(arr)
	if n != 20 && n != 32 {
		return "", false
	}
	buf := make([]byte, n)
	for i, e := range arr {
		switch v := e.(type) {
		case float64:
			if v < 0 || v > 255 {
				return "", false
			}
			buf[i] = byte(uint8(v))
		case int:
			if v < 0 || v > 255 {
				return "", false
			}
			buf[i] = byte(uint8(v))
		case int64:
			if v < 0 || v > 255 {
				return "", false
			}
			buf[i] = byte(uint8(v))
		case uint64:
			if v > 255 {
				return "", false
			}
			buf[i] = byte(uint8(v))
		case uint8:
			buf[i] = byte(v)
		default:
			return "", false
		}
	}
	return "0x" + hex.EncodeToString(buf), true
}

// SanitiseJSON transforms keys to snake_case and encodes []byte or base64-like strings as 0x-hex.
// big.Int is rendered as string to avoid 64-bit precision loss.
// Additionally, fixed-size byte arrays (e.g. [32]byte) are encoded as 0x-hex.
// For JSON-derived []interface{} representing byte arrays (length 20 or 32), encode as 0x-hex as well.
func SanitiseJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v2 := range x {
			out[toSnakeCase(k)] = SanitiseJSON(v2)
		}
		return out
	case []any:
		// If this is a JSON-decoded byte-array (20/32 numeric elements), collapse to 0x-hex.
		if hx, ok := tryByteArrayHex(x); ok {
			return hx
		}
		arr := make([]any, len(x))
		for i := range x {
			arr[i] = SanitiseJSON(x[i])
		}
		return arr
	case []byte:
		return "0x" + hex.EncodeToString(x)
	case string:
		if hx, ok := toHexIfB64(x); ok {
			return hx
		}
		return x
	case *big.Int:
		if x == nil {
			return nil
		}
		return x.String()
	case gethCommon.Address:
		return x.Hex()
	case uint8, uint16, uint32, int8, int16, int32, int:
		return x
	case int64, uint64, float32, float64, bool:
		return x
	default:
		// Handle fixed-size byte arrays, e.g. [32]byte, [20]byte
		rv := reflect.ValueOf(v)
		if rv.IsValid() && rv.Kind() == reflect.Array && rv.Type().Elem().Kind() == reflect.Uint8 {
			n := rv.Len()
			b := make([]byte, n)
			for i := 0; i < n; i++ {
				b[i] = byte(rv.Index(i).Uint())
			}
			return "0x" + hex.EncodeToString(b)
		}
		return v
	}
}

// CursorInfo contains parsed info from a "block-logIndex-txHash" string.
type CursorInfo struct {
	BlockNumber uint64
	LogIndex    uint64
	TxHash      string
}

// ParseCursor splits "block-logIndex-txHash".
func ParseCursor(cursor string) (CursorInfo, error) {
	parts := strings.Split(cursor, "-")
	if len(parts) != 3 {
		return CursorInfo{}, fmt.Errorf("invalid cursor %s", cursor)
	}
	var ci CursorInfo
	var err error
	if _, err = fmt.Sscan(parts[0], &ci.BlockNumber); err != nil {
		return CursorInfo{}, fmt.Errorf("invalid block number in cursor: %w", err)
	}
	if _, err = fmt.Sscan(parts[1], &ci.LogIndex); err != nil {
		return CursorInfo{}, fmt.Errorf("invalid log index in cursor: %w", err)
	}
	ci.TxHash = parts[2]
	return ci, nil
}

// MustEvent returns the ABI event by name (panics on error).
func MustEvent(abiJSON, eventName string) gethAbi.Event {
	parsedABI, err := gethAbi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		panic(err)
	}
	eventDef, ok := parsedABI.Events[eventName]
	if !ok {
		panic("event " + eventName + " not found in ABI")
	}
	return eventDef
}

// BuildAndHashEventEnvelope builds the base64 verifiable-event JSON and computes keccak256(type+"."+b64).
// Note: parameters and metadata are treated as read-only; they are not mutated.
func BuildAndHashEventEnvelope(
	service *string,
	eventName string,
	contractAddress string,
	eventABIJSON string,
	chainID string,
	blockNumber uint64,
	logIndex uint64,
	txHash string,
	timestamp uint64,
	parameters map[string]any,
	metadata map[string]any,
) (PreConsensusEventResults, error) {
	eventABI := MustEvent(eventABIJSON, eventName)

	if parameters == nil {
		parameters = map[string]any{}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}

	event := map[string]any{
		"name":         eventName,
		"address":      contractAddress,
		"topic_hash":   eventABI.ID.Hex(),
		"log_index":    logIndex,
		"block_number": blockNumber,
		"parameters":   parameters,
	}
	if service != nil {
		event["service"] = *service
	}

	transaction := map[string]any{
		"timestamp":    timestamp,
		"chain_id":     chainID,
		"hash":         txHash,
		"block_number": blockNumber,
	}

	createdAt := time.Unix(int64(timestamp), 0).UTC().Format(time.RFC3339Nano)
	verifiableEventBody := map[string]any{
		"created_at":  createdAt,
		"event":       event,
		"parameters":  parameters,
		"transaction": transaction,
		"metadata":    metadata,
	}

	marshalledVerifiableEvent, _ := json.Marshal(verifiableEventBody)
	base64VerifiableEvent := base64.StdEncoding.EncodeToString(marshalledVerifiableEvent)
	// Build typeName: if service is nil, use just eventName, otherwise use service.eventName
	typeName := eventName
	if service != nil {
		typeName = *service + "." + eventName
	}
	payloadToSign := typeName + "." + base64VerifiableEvent
	eventHash := crypto.Keccak256Hash([]byte(payloadToSign))

	return PreConsensusEventResults{
		Base64Event:    base64VerifiableEvent,
		Type:           typeName,
		EventHash:      eventHash,
		BlockTimestamp: timestamp,
		Metadata:       metadata,
	}, nil
}

// ResolveAPIKey returns the API key to use for Courier requests.
// Only the secret-based approach is supported:
//   - apiKeySecret MUST be set to the secret ID.
//   - The secret MUST resolve via rt.GetSecret.
//
// If resolution fails, an empty string is returned and callers should error.
func ResolveAPIKey(rt cre.Runtime, apiKeySecret string) string {
	secretID := strings.TrimSpace(apiKeySecret)
	if secretID == "" {
		return ""
	}

	// WARN: the docs currently state that namespacing is optional, but it is actually currently hardcoded to 'main' and should not be included in fetching the secret
	s, err := rt.GetSecret(&cre.SecretRequest{Id: secretID}).Await()

	if err != nil {
		rt.Logger().Warn("ResolveAPIKey failed to get secret", "error", err)
		return ""
	}

	return s.Value
}

// PostSignedEvent performs identical-consensus report generation and posts the signed event
// to the Courier /system/onchain-watcher-events endpoint. It returns the base64 verifiable event.
func PostSignedEvent(cfg *Config, rt cre.Runtime, eventName, address string, pre PreConsensusEventResults) (string, error) {
	// Generate CRE report with ECDSA signatures over the event hash
	report, err := rt.GenerateReport(&cre.ReportRequest{
		EncodedPayload: pre.EventHash.Bytes(),
		EncoderName:    "evm",
		SigningAlgo:    "ecdsa",
		HashingAlgo:    "keccak256",
	}).Await()
	if err != nil {
		return "", err
	}
	rpb := report.X_GeneratedCodeOnly_Unwrap()

	// Compose HTTP body
	bodyMap := map[string]any{
		"created_at":       int64(pre.BlockTimestamp) * 1000, // Convert seconds to milliseconds for server
		"watcher_id":       cfg.WatcherID,
		"name":             eventName,
		"chain_selector":   cfg.ChainSelector,
		"address":          address,
		"ocr_report":       "0x" + hex.EncodeToString(rpb.RawReport),
		"ocr_context":      "0x" + hex.EncodeToString(rpb.ReportContext),
		"verifiable_event": pre.Base64Event,
		"event_hash":       pre.EventHash.Hex(),
		"signatures": func() []string {
			out := make([]string, 0, len(rpb.Sigs))
			for _, s := range rpb.Sigs {
				out = append(out, "0x"+hex.EncodeToString(s.Signature))
			}
			return out
		}(),
	}
	if cfg.Service != nil {
		bodyMap["domain"] = *cfg.Service
	}
	body, _ := json.Marshal(bodyMap)

	// HTTP POST with identical consensus
	// We aggregate only the integer StatusCode to ensure compatibility with Identical consensus.
	client := &httpcap.Client{}
	key := ResolveAPIKey(rt, cfg.ApiKeySecret)

	_, err = httpcap.SendRequest(
		cfg,
		rt,
		client,
		func(_ *Config, _ *slog.Logger, sr *httpcap.SendRequester) (int, error) {
			if key == "" {
				return 0, fmt.Errorf("courier API key is required but not configured")
			}
			headers := map[string]string{
				"Content-Type": "application/json",
				"Api-Key":      key,
			}
			req := &httpcap.Request{
				Url:     strings.TrimRight(cfg.CourierURL, "/") + "/system/onchain-watcher-events",
				Method:  "POST",
				Headers: headers,
				Body:    body,
			}
			resp, err := sr.SendRequest(req).Await()
			if err != nil {
				return 0, err
			}
			if resp == nil {
				return 0, fmt.Errorf("nil response")
			}
			// Treat any 4xx/5xx as an error (caller may retry).
			if resp.StatusCode >= 400 {
				return 0, fmt.Errorf("courier API responded with status %d", resp.StatusCode)
			}
			return int(resp.StatusCode), nil
		},
		cre.ConsensusIdenticalAggregation[int](),
	).Await()
	if err != nil {
		return "", err
	}

	return pre.Base64Event, nil
}

// CheckResponse validates the httpcap response and returns it unchanged if acceptable.
func CheckResponse(resp *httpcap.Response) (*httpcap.Response, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil response")
	}
	// Treat any 4xx/5xx as an error (caller may retry).
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("courier API responded with status %d", resp.StatusCode)
	}
	return resp, nil
}

// DecodeEventParams decodes an EVM log's topics/data into a named parameter map, using the provided ABI JSON and event-name.
// It returns parameters with snake_case keys and values sanitised via SanitiseJSON.
// It always returns a params map (possibly empty) even when an error occurs parsing ABI or event.
func DecodeEventParams(abiJSON, eventName string, log *evm.Log) (map[string]any, error) {
	params := make(map[string]any)

	parsedABI, err := gethAbi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return params, err
	}
	eventDefinition, ok := parsedABI.Events[eventName]
	if !ok {
		return params, fmt.Errorf("event %q not found in ABI", eventName)
	}

	// Non-indexed (in data)
	nonIndexed := eventDefinition.Inputs.NonIndexed()
	if len(log.Data) > 0 && len(nonIndexed) > 0 {
		vals, err := nonIndexed.Unpack(log.Data)
		if err == nil && len(vals) == len(nonIndexed) {
			for i, arg := range nonIndexed {
				k := toSnakeCase(arg.Name)
				params[k] = SanitiseJSON(vals[i])
			}
		}
	}

	// Indexed (in topics[1:])
	topicIdx := 1 // topics[0] is the event signature
	for _, arg := range eventDefinition.Inputs {
		if !arg.Indexed {
			continue
		}
		if topicIdx >= len(log.Topics) {
			break
		}
		raw := log.Topics[topicIdx]
		topicIdx++

		var decoded any
		switch arg.Type.T {
		case gethAbi.AddressTy:
			if len(raw) == 32 {
				decoded = gethCommon.BytesToAddress(raw[12:])
			} else {
				decoded = gethCommon.BytesToAddress(raw)
			}
		case gethAbi.UintTy, gethAbi.IntTy:
			decoded = new(big.Int).SetBytes(raw)
		case gethAbi.BoolTy:
			if len(raw) == 32 {
				decoded = raw[31] != 0
			} else if len(raw) > 0 {
				decoded = raw[len(raw)-1] != 0
			} else {
				decoded = false
			}
		case gethAbi.FixedBytesTy, gethAbi.BytesTy, gethAbi.StringTy:
			// For dynamic types indexed, the topic is keccak256(value); keep hex
			decoded = raw
		default:
			decoded = raw
		}
		params[toSnakeCase(arg.Name)] = SanitiseJSON(decoded)
	}

	return params, nil
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
