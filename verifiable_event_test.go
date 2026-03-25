package workflows_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/smartcontractkit/crec-api-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workflows "github.com/smartcontractkit/crec-workflow-utils"
)

func ptr(s string) *string {
	return &s
}

func TestVerifiableEvent_EncodeVerifiableEvent(t *testing.T) {
	testCases := []struct {
		name    string
		ve      *models.VerifiableEvent
		wantErr bool
	}{
		{
			name: "encodes basic verifiable event",
			ve: &models.VerifiableEvent{
				ChainFamily:   ptr("evm"),
				ChainSelector: ptr("11155111"),
				Name:          "TestEvent",
				Service:       ptr("test-service"),
				Timestamp:     time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
			},
			wantErr: false,
		},
		{
			name: "encodes event with data",
			ve: &models.VerifiableEvent{
				ChainFamily:   ptr("evm"),
				ChainSelector: ptr("1"),
				Name:          "Transfer",
				Service:       ptr("dta"),
				Timestamp:     time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
				Data: &map[string]interface{}{
					"amount": "1000000",
					"sender": "0x1234",
				},
			},
			wantErr: false,
		},
		{
			name: "encodes event with nil service",
			ve: &models.VerifiableEvent{
				ChainFamily:   ptr("evm"),
				ChainSelector: ptr("11155111"),
				Name:          "GenericEvent",
				Service:       nil,
				Timestamp:     time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
			},
			wantErr: false,
		},
		{
			name:    "encodes nil event",
			ve:      nil,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				encoded, err := workflows.EncodeVerifiableEvent(tc.ve)
				if tc.wantErr {
					require.Error(t, err)
					return
				}

				require.NoError(t, err)
				require.NotEmpty(t, encoded)

				decoded, err := base64.StdEncoding.DecodeString(encoded)
				require.NoError(t, err)

				var result models.VerifiableEvent
				err = json.Unmarshal(decoded, &result)
				require.NoError(t, err)
				assert.Equal(t, tc.ve.ChainFamily, result.ChainFamily)
				assert.Equal(t, tc.ve.ChainSelector, result.ChainSelector)
				assert.Equal(t, tc.ve.Name, result.Name)
				if tc.ve.Service == nil {
					assert.Nil(t, result.Service)
				} else {
					require.NotNil(t, result.Service)
					assert.Equal(t, *tc.ve.Service, *result.Service)
				}
			},
		)
	}
}

func TestVerifiableEvent_DecodeVerifiableEvent(t *testing.T) {
	testCases := []struct {
		name         string
		setupEncoded func() string
		wantErr      bool
		validate     func(t *testing.T, ve *models.VerifiableEvent)
	}{
		{
			name: "decodes valid base64 encoded event",
			setupEncoded: func() string {
				ve := &models.VerifiableEvent{
					ChainFamily:   ptr("evm"),
					ChainSelector: ptr("11155111"),
					Name:          "TestEvent",
					Service:       ptr("operations"),
					Timestamp:     time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
				}
				b, _ := json.Marshal(ve)
				return base64.StdEncoding.EncodeToString(b)
			},
			wantErr: false,
			validate: func(t *testing.T, ve *models.VerifiableEvent) {
				require.NotNil(t, ve.ChainFamily)
				require.NotNil(t, ve.ChainSelector)
				assert.Equal(t, "evm", *ve.ChainFamily)
				assert.Equal(t, "11155111", *ve.ChainSelector)
				assert.Equal(t, "TestEvent", ve.Name)
				require.NotNil(t, ve.Service)
				assert.Equal(t, "operations", *ve.Service)
			},
		},
		{
			name: "decodes event with nil service",
			setupEncoded: func() string {
				ve := &models.VerifiableEvent{
					ChainFamily:   ptr("evm"),
					ChainSelector: ptr("11155111"),
					Name:          "GenericEvent",
					Service:       nil,
					Timestamp:     time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
				}
				b, _ := json.Marshal(ve)
				return base64.StdEncoding.EncodeToString(b)
			},
			wantErr: false,
			validate: func(t *testing.T, ve *models.VerifiableEvent) {
				require.NotNil(t, ve.ChainFamily)
				require.NotNil(t, ve.ChainSelector)
				assert.Equal(t, "evm", *ve.ChainFamily)
				assert.Equal(t, "11155111", *ve.ChainSelector)
				assert.Equal(t, "GenericEvent", ve.Name)
				assert.Nil(t, ve.Service)
			},
		},
		{
			name: "fails on invalid base64",
			setupEncoded: func() string {
				return "not-valid-base64!!!"
			},
			wantErr: true,
		},
		{
			name: "fails on invalid JSON",
			setupEncoded: func() string {
				return base64.StdEncoding.EncodeToString([]byte("not valid json"))
			},
			wantErr: true,
		},
		{
			name: "decodes empty object",
			setupEncoded: func() string {
				return base64.StdEncoding.EncodeToString([]byte("{}"))
			},
			wantErr: false,
			validate: func(t *testing.T, ve *models.VerifiableEvent) {
				assert.Nil(t, ve.ChainFamily)
				assert.Empty(t, ve.Name)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				encoded := tc.setupEncoded()
				result, err := workflows.DecodeVerifiableEvent(encoded)

				if tc.wantErr {
					require.Error(t, err)
					return
				}

				require.NoError(t, err)
				require.NotNil(t, result)
				if tc.validate != nil {
					tc.validate(t, result)
				}
			},
		)
	}
}

func TestVerifiableEvent_ComputeEventHash(t *testing.T) {
	testCases := []struct {
		name         string
		setupEncoded func() string
		wantErr      bool
		validate     func(t *testing.T, hash string)
	}{
		{
			name: "computes hash for valid encoded event",
			setupEncoded: func() string {
				ve := &models.VerifiableEvent{
					ChainFamily:   ptr("evm"),
					ChainSelector: ptr("11155111"),
					Name:          "TestEvent",
					Service:       ptr("operations"),
					Timestamp:     time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
				}
				b, _ := json.Marshal(ve)
				return base64.StdEncoding.EncodeToString(b)
			},
			wantErr: false,
			validate: func(t *testing.T, hash string) {
				assert.Regexp(t, `^0x[0-9a-fA-F]{64}$`, hash)
			},
		},
		{
			name: "hash is deterministic",
			setupEncoded: func() string {
				ve := &models.VerifiableEvent{
					ChainFamily:   ptr("evm"),
					ChainSelector: ptr("1"),
					Name:          "DeterministicTest",
					Service:       ptr("test"),
				}
				b, _ := json.Marshal(ve)
				return base64.StdEncoding.EncodeToString(b)
			},
			wantErr: false,
			validate: func(t *testing.T, hash string) {
				ve := &models.VerifiableEvent{
					ChainFamily:   ptr("evm"),
					ChainSelector: ptr("1"),
					Name:          "DeterministicTest",
					Service:       ptr("test"),
				}
				b, _ := json.Marshal(ve)
				encoded := base64.StdEncoding.EncodeToString(b)
				hash2, err := workflows.ComputeEventHash(encoded)
				require.NoError(t, err)
				assert.Equal(t, hash, hash2.String())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				encoded := tc.setupEncoded()
				result, err := workflows.ComputeEventHash(encoded)

				if tc.wantErr {
					require.Error(t, err)
					return
				}

				require.NoError(t, err)
				if tc.validate != nil {
					tc.validate(t, result.String())
				}
			},
		)
	}
}

func TestVerifiableEvent_RoundTrip(t *testing.T) {
	original := &models.VerifiableEvent{
		ChainFamily:   ptr("evm"),
		ChainSelector: ptr("11155111"),
		Name:          "RequestCreated",
		Service:       ptr("dta"),
		Timestamp:     time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		Data: &map[string]interface{}{
			"requestId": "0x1234567890abcdef",
			"amount":    "1000000000000000000",
		},
	}

	encoded, err := workflows.EncodeVerifiableEvent(original)
	require.NoError(t, err)

	decoded, err := workflows.DecodeVerifiableEvent(encoded)
	require.NoError(t, err)

	assert.Equal(t, original.ChainFamily, decoded.ChainFamily)
	assert.Equal(t, original.ChainSelector, decoded.ChainSelector)
	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.Service, decoded.Service)

	hash1, err := workflows.ComputeEventHash(encoded)
	require.NoError(t, err)

	encoded2, err := workflows.EncodeVerifiableEvent(decoded)
	require.NoError(t, err)

	hash2, err := workflows.ComputeEventHash(encoded2)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}

func TestEventProcessing_BuildVerifiableEventForEVMEvent(t *testing.T) {
	testCases := []struct {
		name        string
		cfg         *workflows.Config
		evmEvent    *models.EVMEvent
		service     *string
		eventName   string
		data        *map[string]interface{}
		wantErr     bool
		errContains string
		validate    func(t *testing.T, ve *models.VerifiableEvent)
	}{
		{
			name: "builds verifiable event with all fields",
			cfg: &workflows.Config{
				ChainSelector: "11155111",
			},
			evmEvent: &models.EVMEvent{
				Address:        "0x1234567890123456789012345678901234567890",
				BlockNumber:    12345,
				BlockTimestamp: 1700000000,
				EventSignature: "Transfer(address,address,uint256)",
				LogIndex:       5,
				TopicHash:      "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
				TxHash:         "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
			service:   ptr("dta"),
			eventName: "Transfer",
			data: &map[string]interface{}{
				"from":  "0x1111111111111111111111111111111111111111",
				"to":    "0x2222222222222222222222222222222222222222",
				"value": "1000000000000000000",
			},
			wantErr: false,
			validate: func(t *testing.T, ve *models.VerifiableEvent) {
				require.NotNil(t, ve.ChainFamily)
				require.NotNil(t, ve.ChainSelector)
				assert.Equal(t, "evm", *ve.ChainFamily)
				assert.Equal(t, "11155111", *ve.ChainSelector)
				require.NotNil(t, ve.Service)
				assert.Equal(t, "dta", *ve.Service)
				assert.Equal(t, "Transfer", ve.Name)
				assert.NotNil(t, ve.Data)
				assert.Equal(t, time.Unix(1700000000, 0).UTC(), ve.Timestamp)
			},
		},
		{
			name: "builds event without optional data",
			cfg: &workflows.Config{
				ChainSelector: "1",
			},
			evmEvent: &models.EVMEvent{
				Address:        "0xabcdef1234567890abcdef1234567890abcdef12",
				BlockNumber:    100,
				BlockTimestamp: 1600000000,
				EventSignature: "Approval(address,address,uint256)",
				LogIndex:       0,
				TopicHash:      "0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925",
				TxHash:         "0x0000000000000000000000000000000000000000000000000000000000000001",
			},
			service:   ptr("operations"),
			eventName: "Approval",
			data:      nil,
			wantErr:   false,
			validate: func(t *testing.T, ve *models.VerifiableEvent) {
				require.NotNil(t, ve.ChainFamily)
				require.NotNil(t, ve.ChainSelector)
				assert.Equal(t, "evm", *ve.ChainFamily)
				assert.Equal(t, "1", *ve.ChainSelector)
				require.NotNil(t, ve.Service)
				assert.Equal(t, "operations", *ve.Service)
				assert.Equal(t, "Approval", ve.Name)
				assert.Nil(t, ve.Data)
			},
		},
		{
			name: "uses chain selector from config",
			cfg: &workflows.Config{
				ChainSelector: "16015286601757825753",
			},
			evmEvent: &models.EVMEvent{
				Address:        "0x1234567890123456789012345678901234567890",
				BlockNumber:    999,
				BlockTimestamp: 1650000000,
				EventSignature: "TestEvent()",
				LogIndex:       1,
				TopicHash:      "0x1234",
				TxHash:         "0x5678",
			},
			service:   ptr("test"),
			eventName: "TestEvent",
			data:      nil,
			wantErr:   false,
			validate: func(t *testing.T, ve *models.VerifiableEvent) {
				require.NotNil(t, ve.ChainSelector)
				assert.Equal(t, "16015286601757825753", *ve.ChainSelector)
			},
		},
		{
			name: "builds event with nil service for workflows not scoped to a service",
			cfg: &workflows.Config{
				ChainSelector: "11155111",
			},
			evmEvent: &models.EVMEvent{
				Address:        "0x1234567890123456789012345678901234567890",
				BlockNumber:    12345,
				BlockTimestamp: 1700000000,
				EventSignature: "GenericEvent()",
				LogIndex:       5,
				TopicHash:      "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
				TxHash:         "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			},
			service:   nil,
			eventName: "GenericEvent",
			data:      nil,
			wantErr:   false,
			validate: func(t *testing.T, ve *models.VerifiableEvent) {
				require.NotNil(t, ve.ChainFamily)
				require.NotNil(t, ve.ChainSelector)
				assert.Equal(t, "evm", *ve.ChainFamily)
				assert.Equal(t, "11155111", *ve.ChainSelector)
				assert.Nil(t, ve.Service)
				assert.Equal(t, "GenericEvent", ve.Name)
				assert.Nil(t, ve.Data)
				assert.Equal(t, time.Unix(1700000000, 0).UTC(), ve.Timestamp)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				result, err := workflows.BuildVerifiableEventForEVMEvent(
					tc.cfg, tc.evmEvent, tc.service, tc.eventName, tc.data,
				)

				if tc.wantErr {
					require.Error(t, err)
					if tc.errContains != "" {
						assert.Contains(t, err.Error(), tc.errContains)
					}
					return
				}

				require.NoError(t, err)
				require.NotNil(t, result)

				if tc.validate != nil {
					tc.validate(t, result)
				}
			},
		)
	}
}
