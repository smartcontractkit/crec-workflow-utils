package workflows_test

import (
	"testing"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/crec-api-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workflows "github.com/smartcontractkit/crec-workflow-utils"
)

const testSenderABI = `[{"type":"event","name":"Sender","inputs":[{"name":"sender","type":"address","indexed":true}],"anonymous":false}]`

func TestInitEventListenerWorkflow_HandlerCountMatchesConfidenceLevels(t *testing.T) {
	levels := []string{"latest", "safe", "finalized"}
	cfg := &workflows.Config{
		ChainSelector:    "16015286601757825753",
		ConfidenceLevels: &levels,
		DetectEventTriggerConfig: workflows.DetectEventTriggerConfig{
			ContractName:       "TestConsumer",
			ContractAddress:      "0x1234567890123456789012345678901234567890",
			ContractEventNames:   []string{"Sender"},
			ContractReaderConfig: workflows.ContractReaderConfig{
				Contracts: map[string]workflows.ContractDef{
					"TestConsumer": {ContractABI: testSenderABI},
				},
			},
		},
	}

	handler := func(*workflows.Config, cre.Runtime, *evm.Log, models.ConfidenceLevel) (string, error) {
		return "", nil
	}

	wf, err := workflows.InitEventListenerWorkflow(cfg, handler)
	require.NoError(t, err)
	assert.Len(t, wf, len(levels))
}
