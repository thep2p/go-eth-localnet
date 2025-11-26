# Prysm Package

Package prysm provides in-process Prysm beacon node and validator management.

## Overview

The prysm package implements a launcher for running Prysm consensus clients in-process alongside Geth execution layer nodes. It follows the component lifecycle pattern with Start, Stop, Wait, and Ready methods for clean integration with the go-eth-localnet orchestration framework.

## Architecture

The package is organized into three main components:

1. **Client** - Manages the lifecycle of a Prysm beacon node and validator
2. **Launcher** - Factory for creating and configuring Prysm clients
3. **Genesis** - Helpers for generating genesis configurations

## Usage

Basic usage involves creating a launcher, configuring a client, and starting it:

```go
logger := zerolog.New(os.Stdout)
launcher := prysm.NewLauncher(logger)

cfg := consensus.Config{
    DataDir:        "/tmp/prysm",
    ChainID:        1337,
    GenesisTime:    time.Now(),
    BeaconPort:     4000,
    P2PPort:        9000,
    RPCPort:        5000,
    EngineEndpoint: "http://127.0.0.1:8551",
    JWTSecret:      jwtSecret,
    ValidatorKeys:  []string{"validator-key"},
}

client, err := launcher.Launch(cfg)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
if err := client.Start(ctx); err != nil {
    log.Fatal(err)
}

// Wait for client to be ready
<-client.Ready()

// Use client...

// Graceful shutdown
client.Stop()
client.Wait()
```

## Engine API Integration

Prysm clients connect to Geth execution layer nodes via the Engine API, which requires JWT authentication. The EngineEndpoint and JWTSecret fields in the configuration must match the Geth node's Engine API settings:

```go
// Get JWT secret from Geth node
jwtSecret, err := gethManager.GetJWTSecret(0)
if err != nil {
    log.Fatal(err)
}

// Get Engine API port from Geth node
enginePort := gethManager.GetEnginePort(0)

// Configure Prysm with matching credentials
cfg.EngineEndpoint = fmt.Sprintf("http://127.0.0.1:%d", enginePort)
cfg.JWTSecret = jwtSecret
```

## Lifecycle Management

The Client implements a standard component lifecycle:

1. Create client with NewClient or launcher.Launch
2. Start client with Start(ctx) - launches background goroutines
3. Wait for Ready() channel to close - client is operational
4. Stop client with Stop() - initiates graceful shutdown
5. Wait for Wait() to return - cleanup is complete

The lifecycle ensures proper resource cleanup and graceful shutdown:

```go
client, _ := launcher.Launch(cfg)
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

if err := client.Start(ctx); err != nil {
    log.Fatal(err)
}

// Register cleanup
defer func() {
    client.Stop()
    client.Wait()
}()

// Client is ready when channel closes
<-client.Ready()
```

## Configuration Options

The launcher supports functional options for flexible configuration:

```go
client, err := launcher.LaunchWithOptions(cfg,
    prysm.WithValidatorKeys(keys),
    prysm.WithBootnodes(bootnodes),
    prysm.WithStaticPeers(peers),
)
```

Available options:

- **WithValidatorKeys** - Configure validator keys for block production
- **WithBootnodes** - Set bootstrap nodes for peer discovery
- **WithStaticPeers** - Configure persistent peer connections
- **WithCheckpointSync** - Enable checkpoint sync for faster syncing
- **WithGenesisState** - Provide custom genesis state

## Genesis Configuration

For local development, genesis states can be generated programmatically using consensus.Config:

```go
// Generate test validator keys
validatorKeys, err := prysm.GenerateTestValidators(4)
if err != nil {
    log.Fatal(err)
}

// Create consensus config
cfg := consensus.Config{
    ChainID:       1337,
    GenesisTime:   prysm.DefaultGenesisTime(),
    ValidatorKeys: validatorKeys,
    FeeRecipient:  common.HexToAddress("0x1234567890123456789012345678901234567890"),
    // ... other config fields ...
}

// Generate genesis state from config and Geth genesis block info
withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
genesisState, err := prysm.GenerateGenesisState(
    cfg,
    withdrawalAddr,
    genesisBlockHash,
    genesisBlockNumber,
    genesisBlockTimestamp,
)
if err != nil {
    log.Fatal(err)
}
```

## Multi-Node Networks

Multiple Prysm clients can be connected to form a consensus network:

```go
// Start first node as bootnode
client1, _ := launcher.Launch(cfg1)
client1.Start(ctx)
<-client1.Ready()

// Get bootnode address
bootnode := client1.P2PAddress()

// Configure subsequent nodes to connect to bootnode
cfg2.Bootnodes = []string{bootnode}
client2, _ := launcher.Launch(cfg2)
client2.Start(ctx)
```

## Testing

The package includes comprehensive tests demonstrating all features:

- `client_test.go` - Client lifecycle and validation tests
- `launcher_test.go` - Launcher and configuration option tests
- `genesis_test.go` - Genesis generation and validation tests
- `integration_test.go` - Full Prysm-Geth integration tests

Integration tests demonstrate the complete workflow from Geth node startup through Prysm client launch and Engine API communication.

## Implementation Status

The package provides a complete API and lifecycle management framework. The actual Prysm v5 integration is planned for implementation in the following issues:

- **[Issue #45](https://github.com/thep2p/go-eth-localnet/issues/45)**: Beacon node initialization, startup, and shutdown
- **[Issue #46](https://github.com/thep2p/go-eth-localnet/issues/46)**: Validator client integration
- **[Issue #47](https://github.com/thep2p/go-eth-localnet/issues/47)**: Genesis state generation and BLS key handling
- **[Issue #48](https://github.com/thep2p/go-eth-localnet/issues/48)**: Beacon API health checks and readiness probes
- **[Issue #49](https://github.com/thep2p/go-eth-localnet/issues/49)**: Prysm-Geth integration tests

This design allows:
- Testing of the lifecycle and configuration patterns
- Integration into the broader orchestration framework
- Incremental implementation of Prysm-specific features
- Clear separation between framework and implementation

## Future Work

Planned enhancements include:
- Complete Prysm v5 beacon node integration
- Validator client implementation with key management
- Engine API forkchoiceUpdated and newPayload calls
- Beacon API health checks and monitoring
- Multi-node consensus validation
- Checkpoint sync implementation
- Custom genesis state generation

## See Also

- `internal/consensus/config.go` - Configuration data structures
- `internal/node` - Geth execution layer node management
- [github.com/prysmaticlabs/prysm/v5](https://github.com/prysmaticlabs/prysm) - Prysm consensus client
