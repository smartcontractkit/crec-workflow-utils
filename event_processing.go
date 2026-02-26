package workflows

import (
	"bytes"
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

	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	httpcap "github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/crec-api-go/models"
)

// GetEventNameFromLog identifies the event name matching the log's topic hash
// by checking against the list of configured ContractEventNames and the ABI.
func GetEventNameFromLog(cfg *Config, payload *evm.Log, abiJSON string) (string, error) {
	if len(payload.Topics) == 0 {
		return "", fmt.Errorf("log has no topics")
	}
	topic0 := payload.Topics[0]

	parsedABI, err := gethAbi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return "", fmt.Errorf("failed to parse ABI: %w", err)
	}

	for _, name := range cfg.DetectEventTriggerConfig.ContractEventNames {
		eventDef, ok := parsedABI.Events[name]
		if ok && bytes.Equal(eventDef.ID.Bytes(), topic0) {
			return name, nil
		}
	}

	return "", fmt.Errorf("event not found for topic %x", topic0)
}

// BuildEVMEventFromLog constructs an EVMEvent from the given evm.Log payload,
// decoding parameters using the contract ABI specified in cfg.
func BuildEVMEventFromLog(rt cre.Runtime, cfg *Config, payload *evm.Log) (*models.EVMEvent, error) {
	blockTimestamp := GetBlockTimestamp(rt, EnsureChainSelector(cfg, cfg.ChainSelector), payload.BlockNumber)
	abi, err := GetContractABI(cfg, cfg.DetectEventTriggerConfig.ContractName)
	if err != nil {
		return nil, err
	}

	eventName, err := GetEventNameFromLog(cfg, payload, abi)
	if err != nil {
		return nil, err
	}

	params, err := DecodeEventParams(abi, eventName, payload)
	if err != nil {
		return nil, err
	}
	return &models.EVMEvent{
		Address:        gethCommon.BytesToAddress(payload.Address).Hex(),
		BlockNumber:    PBToUint64(payload.BlockNumber),
		BlockTimestamp: blockTimestamp,
		ChainId:        cfg.ChainID,
		EventSignature: GetEventSignature(cfg, eventName),
		LogIndex:       payload.Index,
		Params:         &params,
		TopicHash:      "0x" + hex.EncodeToString(payload.Topics[0]),
		TxHash:         "0x" + hex.EncodeToString(payload.TxHash),
	}, nil
}

// BuildVerifiableEventForEVMEvent constructs a VerifiableEvent for the given EVMEvent,
// with the specified service (optional, can be nil for workflows not scoped to a service),
// name, and additional data.
func BuildVerifiableEventForEVMEvent(
	cfg *Config, ev *models.EVMEvent, service *string, name string, data *map[string]interface{},
) (*models.VerifiableEvent, error) {
	chainFamily := "evm"
	chainSelector := cfg.ChainSelector
	ve := models.VerifiableEvent{
		ChainFamily:   &chainFamily,
		ChainSelector: &chainSelector,
		Data:          data,
		Name:          name,
		Service:       service,
		Timestamp:     time.Unix(int64(ev.BlockTimestamp), 0).UTC(),
		ChainEvent:    &models.VerifiableEvent_ChainEvent{},
	}
	err := ve.ChainEvent.FromEVMEvent(*ev)
	if err != nil {
		return nil, fmt.Errorf("failed to set EVM event in VerifiableEvent: %w", err)
	}
	return &ve, nil
}

// SignAndPostVerifiableEvent performs identical-consensus report generation and posts the signed event
// to the Courier /onchain-watcher-events endpoint. It returns the base64 verifiable event.
func SignAndPostVerifiableEvent(cfg *Config, rt cre.Runtime, ve *models.VerifiableEvent) (string, error) {
	encodedVerifiableEvent, err := EncodeVerifiableEvent(ve)
	if err != nil {
		return "", err
	}

	eventHash, err := ComputeEventHash(encodedVerifiableEvent)
	if err != nil {
		return "", err
	}

	report, err := rt.GenerateReport(
		&cre.ReportRequest{
			EncodedPayload: eventHash.Bytes(),
			EncoderName:    "evm",
			SigningAlgo:    "ecdsa",
			HashingAlgo:    "keccak256",
		},
	).Await()
	if err != nil {
		return "", err
	}
	rpb := report.X_GeneratedCodeOnly_Unwrap()

	// Compose HTTP body
	bodyMap := map[string]any{
		"watcher_id":       cfg.WatcherID,
		"ocr_report":       "0x" + hex.EncodeToString(rpb.RawReport),
		"ocr_context":      "0x" + hex.EncodeToString(rpb.ReportContext),
		"verifiable_event": encodedVerifiableEvent,
		"signatures": func() []string {
			out := make([]string, 0, len(rpb.Sigs))
			for _, s := range rpb.Sigs {
				out = append(out, "0x"+hex.EncodeToString(s.Signature))
			}
			return out
		}(),
	}
	body, _ := json.Marshal(bodyMap)

	// HTTP POST with identical consensus
	// We aggregate only the integer StatusCode to ensure compatibility with Identical consensus.
	client := &httpcap.Client{}

	// Retry only the network request, not the entire event processing/signing.
	_, err = Retry(slog.Default(), "post_verifiable_event", func() (int, error) {
		return httpcap.SendRequest(
			cfg,
			rt,
			client,
			func(_ *Config, _ *slog.Logger, sr *httpcap.SendRequester) (int, error) {
				headers := map[string]string{
					"Content-Type": "application/json",
				}
				req := &httpcap.Request{
					Url:     strings.TrimRight(cfg.CourierURL, "/") + "/system/v1/onchain-watcher-events",
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
				// Treat 4xx as deterministic errors (except 408/429) and stop retry.
				// Treat 5xx as retriable errors.
				if resp.StatusCode >= 400 {
					err := fmt.Errorf("courier API responded with status %d", resp.StatusCode)
					if resp.StatusCode < 500 && resp.StatusCode != 408 && resp.StatusCode != 429 {
						return 0, StopRetry(err)
					}
					return 0, err
				}
				return int(resp.StatusCode), nil
			},
			cre.ConsensusIdenticalAggregation[int](),
		).Await()
	})
	if err != nil {
		return "", err
	}

	return eventHash.String(), nil
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

// tryByteArrayHex Attempts to detect if arr is a []interface{} representing a byte-array
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
