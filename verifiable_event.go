package workflows

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/smartcontractkit/crec-api-go/models"
)

var ErrNilVerifiableEvent = errors.New("verifiable event cannot be nil")

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

func ComputeEventHash(encoded string) (common.Hash, error) {
	return crypto.Keccak256Hash([]byte(encoded)), nil
}
