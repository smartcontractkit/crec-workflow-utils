# CREC Workflow Utils

Common utilities for building CRE (Chainlink Runtime Environment) event watcher workflows. Used as part of the CREC (CRE Connect) ecosystem.

## Overview

This package provides shared functionality for CRE workflow extensions that implement event watcher workflows:

- **Configuration parsing** — Parse YAML/JSON workflow configs
- **Event processing** — Decode EVM log events, build verifiable events, post to Courier
- **Workflow initialization** — Wire up EVM log triggers with handlers
- **Testing utilities** — Mock runtime for unit testing handlers

## Installation

```bash
go get github.com/smartcontractkit/crec-workflow-utils
```

## Usage

### Workflow Entry Point

```go
//go:build wasip1

package main

import (
    "log/slog"
    "github.com/smartcontractkit/cre-sdk-go/cre"
    "github.com/smartcontractkit/cre-sdk-go/cre/wasm"
    workflows "github.com/smartcontractkit/crec-workflow-utils"
)

func main() {
    r := wasm.NewRunner(workflows.ParseWorkflowConfig)
    r.Run(func(cfg *workflows.Config, _ *slog.Logger, _ cre.SecretsProvider) (cre.Workflow[*workflows.Config], error) {
        return workflows.InitEventListenerWorkflow(cfg, OnLog)
    })
}
```

### Log Handler

```go
package handler

import (
    "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
    "github.com/smartcontractkit/cre-sdk-go/cre"
    workflows "github.com/smartcontractkit/crec-workflow-utils"
)

func OnLog(cfg *workflows.Config, rt cre.Runtime, payload *evm.Log, confidenceLevel string) (string, error) {
    evmEvent, err := workflows.BuildEVMEventFromLog(rt, cfg, payload)
    if err != nil {
        return "", err
    }

    abiJSON, err := workflows.GetContractABI(cfg, cfg.DetectEventTriggerConfig.ContractName)
    if err != nil {
        return "", err
    }
    eventName, err := workflows.GetEventNameFromLog(cfg, payload, abiJSON)
    if err != nil {
        return "", err
    }

    verifiableEvent, err := workflows.BuildVerifiableEventForEVMEvent(
        cfg, evmEvent, cfg.Service, eventName, nil,
    )
    if err != nil {
        return "", err
    }

    return workflows.SignAndPostVerifiableEvent(cfg, rt, verifiableEvent)
}
```

### Configuration

Workflow config accepts YAML or JSON (YAML is tried first; JSON is used if YAML parsing fails). Required fields:

| Field                                                     | Description                                              |
| --------------------------------------------------------- | -------------------------------------------------------- |
| `chainSelector`                                           | Chain selector (uint64 string) for the target EVM chain  |
| `courierURL`                                              | Base URL of the event ingestion service                  |
| `watcherID`                                               | Watcher identifier                                       |
| `detectEventTriggerConfig.contractName`                   | Contract name in `contractReaderConfig.contracts`        |
| `detectEventTriggerConfig.contractAddress`                | Contract address to watch                                |
| `detectEventTriggerConfig.contractEventNames`             | List of event names to listen for                        |
| `detectEventTriggerConfig.contractReaderConfig.contracts` | Map of contract name → ABI                               |

Example config structure:

```yaml
network: "sepolia"
chainSelector: "16015286601757825753"
courierURL: "https://courier.example.com"
watcherID: "my-watcher-id"
detectEventTriggerConfig:
  contractName: "MyContract"
  contractAddress: "0x..."
  contractEventNames: ["Transfer", "Approval"]
  contractReaderConfig:
    contracts:
      MyContract:
        contractABI: '[{"type":"event","name":"Transfer",...}]'
```

## Testing

```bash
go test ./...
```

## License

[BUSL](LICENSE)
