# Consensus Layer Client Abstraction

This package provides abstractions for Consensus Layer (CL) client implementations, enabling support for multiple CL clients (Prysm, Lighthouse, Nimbus, etc.) without duplicating code.

## Purpose

BaseClient serves as the foundation for all CL client implementations, providing:
- Common lifecycle management via `component.Manager` pattern
- Standardized configuration access
- Consistent logging
- Beacon API endpoint management

## Component Lifecycle Pattern

BaseClient uses the `component.Manager` pattern from [skipgraph-go](https://github.com/thep2p/skipgraph-go/tree/main/modules/component):

### Key Concepts

1. **Manager handles lifecycle**: Start/Ready/Done signaling is automatic
2. **Startup logic**: Provided via `component.WithStartupLogic()`
3. **Shutdown logic**: Provided via `component.WithShutdownLogic()`
4. **Child components**: Can be added via `component.WithComponent()`
5. **Hierarchical coordination**: Parents wait for children before signaling ready/done
6. **Single start**: Calling Start() twice triggers a panic (intentional error detection)

## Usage Example

### Creating a CL Client Implementation

```go
package prysm

import (
    "github.com/thep2p/go-eth-localnet/internal/consensus"
    "github.com/thep2p/skipgraph-go/modules"
    "github.com/thep2p/skipgraph-go/modules/component"
)

type PrysmClient struct {
    *consensus.BaseClient
    process *exec.Cmd
    apiClient *prysm.BeaconClient
}

func NewPrysmClient(logger zerolog.Logger, cfg consensus.Config) *PrysmClient {
    p := &PrysmClient{}

    // BaseClient encapsulates the component.Manager
    p.BaseClient = consensus.NewBaseClient(logger, cfg,
        component.WithStartupLogic(p.startPrysm),
        component.WithShutdownLogic(p.stopPrysm),
    )

    return p
}

// startPrysm is called when Start() is invoked
func (p *PrysmClient) startPrysm(ctx modules.ThrowableContext) {
    p.Logger().Info().Msg("starting prysm client")

    // Launch prysm process
    cmd := exec.CommandContext(ctx.Context(), "prysm",
        "--datadir", p.Config().DataDir,
        "--http-port", fmt.Sprintf("%d", p.Config().BeaconPort),
    )

    if err := cmd.Start(); err != nil {
        ctx.ThrowIrrecoverable(err) // Fatal error during startup
    }

    p.process = cmd

    // Wait for beacon API to be ready
    p.waitForBeaconAPI(ctx)
}

// stopPrysm is called during shutdown
func (p *PrysmClient) stopPrysm() {
    p.Logger().Info().Msg("stopping prysm client")

    if p.process != nil {
        p.process.Process.Signal(syscall.SIGTERM)
        p.process.Wait()
    }
}

// Implement Client interface methods
func (p *PrysmClient) ValidatorKeys() []string {
    return p.Config().ValidatorKeys
}

func (p *PrysmClient) Metrics() (*consensus.Metrics, error) {
    // Query beacon API for metrics
    return p.apiClient.GetMetrics()
}
```

### Using the Client

```go
func main() {
    cfg := consensus.Config{
        Client:     "prysm",
        DataDir:    "/tmp/prysm",
        BeaconPort: 4000,
        P2PPort:    9000,
    }

    client := NewPrysmClient(logger, cfg)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    tctx := throwable.NewContext(ctx)

    // Start the client (runs startup logic)
    client.Start(tctx)

    // Wait for ready (blocks until startup logic completes)
    <-client.Ready()

    // Client is now operational
    endpoint := client.BeaconEndpoint()
    metrics, _ := client.Metrics()

    // Trigger shutdown
    cancel()

    // Wait for complete shutdown
    <-client.Done()
}
```

## Testing

The package includes comprehensive tests demonstrating:
- Basic lifecycle (start/ready/done)
- Startup and shutdown logic execution
- Child component management
- Configuration access
- Error handling (duplicate starts)

See `base_client_test.go` for examples.

## Current Status

**Implementation Status**: ✅ **Complete** (Step 2a of issue #32)

This abstraction layer is ready for concrete CL implementations:
- ⏳ **Step 2b**: Prysm in-process launcher (issue #33)
- ⏳ **Step 3**: Multi-node consensus network (issue #34)

## Architecture Notes

### Why BaseClient?

BaseClient provides a common foundation that:
1. **Reduces duplication**: All CL clients share configuration, logging, endpoint management
2. **Enforces patterns**: Ensures all implementations follow component lifecycle
3. **Simplifies testing**: Mock implementations can be created easily
4. **Enables composition**: Clients can manage child components (validators, attesters, etc.)

### What NOT to Put in BaseClient

- Client-specific process management
- Protocol-specific API calls
- Validator keystores or signing logic
- Network-specific genesis handling

These belong in concrete implementations (PrysmClient, LighthouseClient, etc.).

## Related Documentation

- [component.Manager](https://github.com/thep2p/skipgraph-go/tree/main/modules/component) - Lifecycle pattern
- [Issue #32](https://github.com/thep2p/go-eth-localnet/issues/32) - CL abstraction design
- [Issue #33](https://github.com/thep2p/go-eth-localnet/issues/33) - Prysm implementation (next)
