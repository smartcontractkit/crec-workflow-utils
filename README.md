# CREC Workflow Utils

Common utilities for building CRE (Chainlink Runtime Environment) event watcher workflows.

## Overview

This package provides shared functionality for CRE workflow extensions that implement event watcher workflows:

- **Configuration parsing** - Parse YAML/JSON workflow configs
- **Event processing** - Decode EVM log events, build consensus reports
- **Workflow initialization** - Wire up EVM log triggers with handlers
- **Testing utilities** - Mock runtime for unit testing handlers

## Installation

```bash
go get github.com/smartcontractkit/crec-workflow-utils
```

## Usage

### Workflow Handler

```go
package handler

import (
    "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
    "github.com/smartcontractkit/cre-sdk-go/cre"
    workflows "github.com/smartcontractkit/crec-workflow-utils"
)

func OnLog(cfg *workflows.Config, rt cre.Runtime, payload *evm.Log) (string, error) {
    // Get block timestamp
    ts := workflows.GetBlockTimestamp(rt, workflows.EnsureChainSelector(cfg, cfg.ChainSelector), payload.BlockNumber)
    
    // Decode event parameters
    abiJSON, _ := workflows.GetContractABI(cfg, cfg.DetectEventTriggerConfig.ContractName)
    params, _ := workflows.DecodeEventParams(abiJSON, cfg.DetectEventTriggerConfig.ContractEventName, payload)
    
    // Build and post signed event
    pre, err := workflows.BuildAndHashEventEnvelope(
        cfg.Service,
        cfg.DetectEventTriggerConfig.ContractEventName,
        cfg.DetectEventTriggerConfig.ContractAddress,
        abiJSON,
        cfg.ChainID,
        workflows.PBToUint64(payload.BlockNumber),
        uint64(payload.Index),
        workflows.TxHashFromLog(payload),
        ts,
        params,
        nil,
    )
    if err != nil {
        return "", err
    }
    
    return workflows.PostSignedEvent(cfg, rt, cfg.DetectEventTriggerConfig.ContractEventName, cfg.DetectEventTriggerConfig.ContractAddress, pre)
}
```
