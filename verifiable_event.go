package workflows

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/smartcontractkit/crec-api-go/models"
)

// ErrNilVerifiableEvent is returned when a nil VerifiableEvent is passed to EncodeVerifiableEvent.
var ErrNilVerifiableEvent = errors.New("verifiable event cannot be nil")

// EncodeVerifiableEvent marshals the VerifiableEvent to JSON and encodes it as base64.
// It returns ErrNilVerifiableEvent if ve is nil.
func EncodeVerifiableEvent(ve *models.VerifiableEvent) (string, error) {
	if ve == nil {
		return "", ErrNilVerifiableEvent
	}
	jsonString, err := json.Marshal(ve)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(jsonString), nil
}

// DecodeVerifiableEvent decodes a base64-encoded JSON string into a VerifiableEvent.
// It returns an error if the input is not valid base64 or if JSON unmarshaling fails.
func DecodeVerifiableEvent(encoded string) (*models.VerifiableEvent, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	var ve models.VerifiableEvent
	err = json.Unmarshal(decodedBytes, &ve)
	if err != nil {
		return nil, err
	}
	return &ve, nil
}

// ComputeEventHash returns the Keccak256 hash of the encoded string as a common.Hash.
// The encoded string is typically the base64 output from EncodeVerifiableEvent.
func ComputeEventHash(encoded string) (common.Hash, error) {
	return crypto.Keccak256Hash([]byte(encoded)), nil
}
