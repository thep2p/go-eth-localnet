# Step 2b: Prysm Consensus Client In-Process Launcher

## Overview

This issue implements the Prysm consensus client launcher using the CL abstraction layer from Step 2a. Prysm will be embedded in-process using its Go libraries, similar to how we embed Geth. This provides a production-grade consensus client for testing real Ethereum consensus mechanisms without external processes or containers.

Prysm is chosen as our first CL implementation because it's written in Go and can be embedded directly, maintaining our philosophy of pure Go orchestration without external dependencies.

## Why This Matters

- **Real Consensus**: Replace SimulatedBeacon with actual consensus implementation
- **In-Process Control**: Direct control over Prysm lifecycle without process management overhead
- **Production Fidelity**: Test against real consensus client behavior
- **Developer Experience**: No need to install/manage separate Prysm binaries

## Acceptance Criteria

- [ ] Prysm beacon node can be launched in-process
- [ ] Prysm validator client can be launched with test keys
- [ ] Proper Engine API communication with paired Geth node
- [ ] Beacon API endpoints are accessible
- [ ] Multi-node Prysm networks can form consensus
- [ ] Graceful shutdown of Prysm components
- [ ] Tests verify Prysm-Geth integration

## Implementation Tasks

### 1. Prysm Dependencies
```go
// go.mod additions
require (
    github.com/prysmaticlabs/prysm/v5 v5.1.2
    github.com/prysmaticlabs/prysm/v5/config/params v5.1.2
    github.com/prysmaticlabs/prysm/v5/runtime v5.1.2
    github.com/prysmaticlabs/prysm/v5/beacon-chain v5.1.2
    github.com/prysmaticlabs/prysm/v5/validator v5.1.2
)
```

### 2. Prysm Client Implementation
```go
// internal/consensus/prysm/client.go
package prysm

import (
    "context"
    "fmt"
    "path/filepath"
    "time"

    "github.com/prysmaticlabs/prysm/v5/beacon-chain/node"
    "github.com/prysmaticlabs/prysm/v5/cmd/beacon-chain/flags"
    "github.com/prysmaticlabs/prysm/v5/config/params"
    "github.com/prysmaticlabs/prysm/v5/runtime/tos"
    "github.com/prysmaticlabs/prysm/v5/validator/node"
    "github.com/rs/zerolog"
    "github.com/thep2p/go-eth-localnet/internal/consensus"
    "github.com/urfave/cli/v2"
)

// Client wraps an in-process Prysm beacon node and validator.
type Client struct {
    *consensus.BaseClient

    beaconNode    *node.BeaconNode
    validatorNode *node.ValidatorClient
    ctx           context.Context
    cancel        context.CancelFunc
}

// NewClient creates a new Prysm client instance.
func NewClient(logger zerolog.Logger, cfg consensus.Config) (*Client, error) {
    return &Client{
        BaseClient: consensus.NewBaseClient(logger, cfg),
    }, nil
}

// Start launches the Prysm beacon node and validator.
func (c *Client) Start(ctx context.Context) error {
    if err := c.BaseClient.Start(ctx); err != nil {
        return err
    }

    c.ctx, c.cancel = context.WithCancel(ctx)

    // Accept Terms of Service programmatically for testing
    tos.MarkAccepted()

    // Start beacon node
    if err := c.startBeaconNode(); err != nil {
        return fmt.Errorf("start beacon node: %w", err)
    }

    // Start validator if keys are configured
    if len(c.config.ValidatorKeys) > 0 {
        if err := c.startValidator(); err != nil {
            return fmt.Errorf("start validator: %w", err)
        }
    }

    return nil
}

// startBeaconNode launches the Prysm beacon node.
func (c *Client) startBeaconNode() error {
    app := cli.NewApp()
    set := cli.NewContext(app, nil, nil)

    // Configure beacon node flags
    beaconFlags := []cli.Flag{
        &cli.StringFlag{
            Name:  "datadir",
            Value: filepath.Join(c.config.DataDir, "beacon"),
        },
        &cli.StringFlag{
            Name:  "execution-endpoint",
            Value: c.config.EngineEndpoint,
        },
        &cli.StringFlag{
            Name:  "jwt-secret",
            Value: string(c.config.JWTSecret),
        },
        &cli.IntFlag{
            Name:  "grpc-gateway-port",
            Value: c.config.BeaconPort,
        },
        &cli.IntFlag{
            Name:  "rpc-port",
            Value: c.config.RPCPort,
        },
        &cli.IntFlag{
            Name:  "p2p-tcp-port",
            Value: c.config.P2PPort,
        },
        &cli.IntFlag{
            Name:  "p2p-udp-port",
            Value: c.config.P2PPort,
        },
        &cli.BoolFlag{
            Name:  "disable-monitoring",
            Value: true,
        },
        &cli.BoolFlag{
            Name:  "accept-terms-of-use",
            Value: true,
        },
        &cli.StringFlag{
            Name:  "chain-id",
            Value: fmt.Sprintf("%d", c.config.ChainID),
        },
        &cli.StringFlag{
            Name:  "network-id",
            Value: fmt.Sprintf("%d", c.config.ChainID),
        },
    }

    // Add bootstrap nodes if configured
    if len(c.config.Bootnodes) > 0 {
        beaconFlags = append(beaconFlags, &cli.StringSliceFlag{
            Name:  "bootstrap-node",
            Value: cli.NewStringSlice(c.config.Bootnodes...),
        })
    }

    // Apply flags to context
    for _, flag := range beaconFlags {
        if err := flag.Apply(set); err != nil {
            return fmt.Errorf("apply flag %s: %w", flag.Names()[0], err)
        }
    }

    // Create beacon node configuration
    beaconCfg := &node.Config{
        BeaconChainCfg: params.BeaconChainConfig(),
        P2P:            c.buildP2PConfig(),
        ExecutionCfg:   c.buildExecutionConfig(),
    }

    // Initialize beacon node
    var err error
    c.beaconNode, err = node.New(c.ctx, beaconCfg)
    if err != nil {
        return fmt.Errorf("create beacon node: %w", err)
    }

    // Start beacon node in background
    go func() {
        if err := c.beaconNode.Start(); err != nil {
            c.logger.Error().Err(err).Msg("beacon node failed")
        }
    }()

    return nil
}

// startValidator launches the Prysm validator client.
func (c *Client) startValidator() error {
    // Import validator keys
    keystorePath := filepath.Join(c.config.DataDir, "validator", "keystores")
    if err := c.importValidatorKeys(keystorePath); err != nil {
        return fmt.Errorf("import validator keys: %w", err)
    }

    app := cli.NewApp()
    set := cli.NewContext(app, nil, nil)

    // Configure validator flags
    validatorFlags := []cli.Flag{
        &cli.StringFlag{
            Name:  "datadir",
            Value: filepath.Join(c.config.DataDir, "validator"),
        },
        &cli.StringFlag{
            Name:  "beacon-rpc-provider",
            Value: fmt.Sprintf("127.0.0.1:%d", c.config.RPCPort),
        },
        &cli.StringFlag{
            Name:  "wallet-dir",
            Value: filepath.Join(c.config.DataDir, "validator", "wallet"),
        },
        &cli.StringFlag{
            Name:  "wallet-password-file",
            Value: c.createPasswordFile(),
        },
        &cli.BoolFlag{
            Name:  "disable-accounts-v2",
            Value: false,
        },
        &cli.BoolFlag{
            Name:  "accept-terms-of-use",
            Value: true,
        },
        &cli.StringFlag{
            Name:  "suggested-fee-recipient",
            Value: c.config.FeeRecipient.Hex(),
        },
    }

    // Apply flags
    for _, flag := range validatorFlags {
        if err := flag.Apply(set); err != nil {
            return fmt.Errorf("apply validator flag %s: %w", flag.Names()[0], err)
        }
    }

    // Create validator configuration
    validatorCfg := &validator.Config{
        WalletDir:          filepath.Join(c.config.DataDir, "validator", "wallet"),
        KeystoresDir:       keystorePath,
        BeaconRPCProvider:  fmt.Sprintf("127.0.0.1:%d", c.config.RPCPort),
        FeeRecipientConfig: c.config.FeeRecipient,
    }

    // Initialize validator
    var err error
    c.validatorNode, err = validator.New(c.ctx, validatorCfg)
    if err != nil {
        return fmt.Errorf("create validator: %w", err)
    }

    // Start validator in background
    go func() {
        if err := c.validatorNode.Start(); err != nil {
            c.logger.Error().Err(err).Msg("validator failed")
        }
    }()

    return nil
}

// Stop gracefully shuts down Prysm components.
func (c *Client) Stop() error {
    if c.cancel != nil {
        c.cancel()
    }

    // Stop validator first
    if c.validatorNode != nil {
        c.validatorNode.Close()
    }

    // Then stop beacon node
    if c.beaconNode != nil {
        c.beaconNode.Close()
    }

    return c.BaseClient.Stop()
}

// ValidatorKeys returns the configured validator public keys.
func (c *Client) ValidatorKeys() []string {
    // Convert private keys to public keys
    pubKeys := make([]string, len(c.config.ValidatorKeys))
    for i, privKey := range c.config.ValidatorKeys {
        // Parse and convert to public key
        pubKeys[i] = c.derivePublicKey(privKey)
    }
    return pubKeys
}

// Metrics queries Prysm for operational metrics.
func (c *Client) Metrics() (*consensus.Metrics, error) {
    // Query beacon node API for metrics
    // Implementation would use Prysm's beacon API client
    return &consensus.Metrics{
        CurrentSlot:    c.getCurrentSlot(),
        HeadSlot:       c.getHeadSlot(),
        FinalizedSlot:  c.getFinalizedSlot(),
        PeerCount:      c.getPeerCount(),
        IsSyncing:      c.isSyncing(),
        ValidatorCount: len(c.config.ValidatorKeys),
    }, nil
}
```

### 3. Prysm Launcher
```go
// internal/consensus/prysm/launcher.go
package prysm

import (
    "fmt"

    "github.com/rs/zerolog"
    "github.com/thep2p/go-eth-localnet/internal/consensus"
)

// Launcher creates Prysm client instances.
type Launcher struct {
    logger zerolog.Logger
}

// NewLauncher creates a new Prysm launcher.
func NewLauncher(logger zerolog.Logger) *Launcher {
    return &Launcher{
        logger: logger.With().Str("component", "prysm-launcher").Logger(),
    }
}

// Launch creates and starts a new Prysm client.
func (l *Launcher) Launch(cfg consensus.Config) (consensus.Client, error) {
    // Validate configuration
    if err := l.ValidateConfig(cfg); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }

    // Create client
    client, err := NewClient(l.logger, cfg)
    if err != nil {
        return nil, fmt.Errorf("create client: %w", err)
    }

    return client, nil
}

// Name returns the launcher name.
func (l *Launcher) Name() string {
    return "prysm"
}

// ValidateConfig ensures the configuration is valid for Prysm.
func (l *Launcher) ValidateConfig(cfg consensus.Config) error {
    if cfg.EngineEndpoint == "" {
        return fmt.Errorf("engine endpoint required")
    }
    if len(cfg.JWTSecret) != 32 {
        return fmt.Errorf("JWT secret must be 32 bytes")
    }
    if cfg.BeaconPort == 0 {
        return fmt.Errorf("beacon port required")
    }
    if cfg.P2PPort == 0 {
        return fmt.Errorf("P2P port required")
    }
    if cfg.RPCPort == 0 {
        return fmt.Errorf("RPC port required")
    }
    return nil
}
```

### 4. Genesis Configuration Helper
```go
// internal/consensus/prysm/genesis.go
package prysm

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/ethereum/go-ethereum/common"
    "github.com/prysmaticlabs/prysm/v5/config/params"
)

// GenesisConfig represents the beacon chain genesis configuration.
type GenesisConfig struct {
    GenesisTime     uint64         `json:"genesis_time"`
    GenesisStateRoot string        `json:"genesis_state_root"`
    DepositContract common.Address `json:"deposit_contract_address"`
    ChainID         uint64         `json:"chain_id"`
}

// CreateGenesisConfig generates a genesis configuration for local testing.
func CreateGenesisConfig(dataDir string, chainID uint64, genesisTime time.Time) error {
    // Use minimal config for faster testing
    params.UseMinimalConfig()

    genesis := &GenesisConfig{
        GenesisTime:      uint64(genesisTime.Unix()),
        GenesisStateRoot: "0x0000000000000000000000000000000000000000000000000000000000000000",
        DepositContract:  common.HexToAddress("0x1234567890123456789012345678901234567890"),
        ChainID:          chainID,
    }

    // Write genesis config
    genesisPath := filepath.Join(dataDir, "genesis.json")
    data, err := json.MarshalIndent(genesis, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal genesis: %w", err)
    }

    if err := os.WriteFile(genesisPath, data, 0644); err != nil {
        return fmt.Errorf("write genesis: %w", err)
    }

    return nil
}

// ConfigureTestnet sets up Prysm for local testnet operation.
func ConfigureTestnet() {
    config := params.BeaconChainConfig()

    // Adjust for fast local testing
    config.SecondsPerSlot = 2                    // 2 second slots
    config.SlotsPerEpoch = 6                     // 6 slots per epoch
    config.MinGenesisActiveValidatorCount = 1    // Allow single validator
    config.MinGenesisTime = 0                    // Start immediately
    config.TargetAggregatorsPerCommittee = 1
    config.MinPerEpochChurnLimit = 1

    // Reduce deposit requirements for testing
    config.EjectionBalance = 1e9                 // 1 Gwei
    config.MaxEffectiveBalance = 32e9            // 32 Ether

    params.OverrideBeaconChainConfig(config)
}
```

### 5. Integration with Node Manager
```go
// internal/node/manager_prysm.go
package node

import (
    "context"
    "fmt"

    "github.com/thep2p/go-eth-localnet/internal/consensus"
    "github.com/thep2p/go-eth-localnet/internal/consensus/prysm"
)

// StartWithPrysm launches nodes with Prysm consensus clients.
func (m *Manager) StartWithPrysm(ctx context.Context, nodeCount int, opts ...LaunchOption) error {
    // First start EL nodes with Engine API enabled
    if err := m.Start(ctx, nodeCount, append(opts, WithEngineAPI())...); err != nil {
        return fmt.Errorf("start EL nodes: %w", err)
    }

    // Configure Prysm for testing
    prysm.ConfigureTestnet()

    // Create CL clients for each EL node
    for i := 0; i < nodeCount; i++ {
        cfg := consensus.Config{
            Client:         "prysm",
            DataDir:        filepath.Join(m.baseDataDir, fmt.Sprintf("cl-node%d", i)),
            ChainID:        uint64(m.chainID.Int64()),
            GenesisTime:    time.Now(),
            BeaconPort:     m.assignNewPort(),
            P2PPort:        m.assignNewPort(),
            RPCPort:        m.assignNewPort(),
            EngineEndpoint: fmt.Sprintf("http://127.0.0.1:%d", m.GetEnginePort(i)),
            JWTSecret:      m.GetJWTSecretBytes(i),
        }

        // First node should have a validator
        if i == 0 {
            cfg.ValidatorKeys = m.generateValidatorKeys(1)
        }

        // Other nodes connect to first as bootnode
        if i > 0 && m.clClients[0] != nil {
            cfg.Bootnodes = []string{m.clClients[0].GetENR()}
        }

        launcher := prysm.NewLauncher(m.logger)
        client, err := launcher.Launch(cfg)
        if err != nil {
            return fmt.Errorf("launch CL node %d: %w", i, err)
        }

        if err := client.Start(ctx); err != nil {
            return fmt.Errorf("start CL client %d: %w", i, err)
        }

        m.clClients = append(m.clClients, client)
    }

    return nil
}
```

## Testing Requirements

### 1. Test Prysm-Geth Integration
```go
// internal/consensus/prysm/integration_test.go
func TestPrysmGethIntegration(t *testing.T) {
    ctx, cancel, manager := startNodesWithPrysm(t, 1)
    defer cancel()

    // Wait for Prysm to be ready
    clClient := manager.GetCLClient(0)
    require.Eventually(t, clClient.Ready, 30*time.Second, 1*time.Second)

    // Verify Engine API communication
    metrics, err := clClient.Metrics()
    require.NoError(t, err)
    require.Greater(t, metrics.CurrentSlot, uint64(0))

    // Verify block production
    initialBlock := getLatestBlock(t, manager.GetRPCPort(0))
    time.Sleep(5 * time.Second)
    newBlock := getLatestBlock(t, manager.GetRPCPort(0))
    require.Greater(t, newBlock, initialBlock)
}
```

### 2. Test Multi-Node Consensus
```go
func TestMultiNodePrysmConsensus(t *testing.T) {
    ctx, cancel, manager := startNodesWithPrysm(t, 3)
    defer cancel()

    // Wait for all CL clients to be ready
    for i := 0; i < 3; i++ {
        clClient := manager.GetCLClient(i)
        require.Eventually(t, clClient.Ready, 30*time.Second, 1*time.Second)
    }

    // Verify peer connections
    for i := 0; i < 3; i++ {
        metrics, err := manager.GetCLClient(i).Metrics()
        require.NoError(t, err)
        require.GreaterOrEqual(t, metrics.PeerCount, 2)
    }

    // Verify consensus (all nodes on same head)
    time.Sleep(10 * time.Second) // Allow some slots to pass
    var headSlot uint64
    for i := 0; i < 3; i++ {
        metrics, err := manager.GetCLClient(i).Metrics()
        require.NoError(t, err)
        if i == 0 {
            headSlot = metrics.HeadSlot
        } else {
            require.Equal(t, headSlot, metrics.HeadSlot)
        }
    }
}
```

### 3. Test Validator Functionality
```go
func TestPrysmValidator(t *testing.T) {
    ctx, cancel, manager := startNodesWithPrysm(t, 1,
        consensus.WithValidatorKeys([]string{"test-key"}))
    defer cancel()

    clClient := manager.GetCLClient(0)
    require.Eventually(t, clClient.Ready, 30*time.Second, 1*time.Second)

    // Verify validator is active
    validators := clClient.ValidatorKeys()
    require.Len(t, validators, 1)

    // Verify blocks are being proposed
    metrics, err := clClient.Metrics()
    require.NoError(t, err)
    require.Equal(t, 1, metrics.ValidatorCount)
}
```

## Technical Considerations

### Important Notes
- **Memory Usage**: Prysm uses significant memory (~2GB per node)
- **Startup Time**: Prysm takes 10-30 seconds to fully initialize
- **Port Requirements**: Each Prysm instance needs 3 ports (beacon, P2P, RPC)
- **Data Persistence**: Prysm creates substantial data in datadir

### Gotchas
1. **Terms of Service**: Prysm requires ToS acceptance (handled programmatically)
2. **Config Overrides**: Must use minimal config for fast local testing
3. **Genesis Timing**: Genesis time must be coordinated between EL and CL
4. **Validator Keys**: Test keys must be properly formatted for Prysm
5. **Resource Limits**: Running multiple Prysm nodes requires significant resources

### Performance Optimizations
- Use minimal preset for faster slot times
- Disable unnecessary Prysm features (metrics, monitoring)
- Share genesis state between nodes when possible
- Use in-memory databases for testing

## References
- [Prysm Documentation](https://docs.prylabs.network/)
- [Prysm In-Process Usage](https://github.com/prysmaticlabs/prysm/tree/develop/testing)
- [Engine API Specification](https://github.com/ethereum/execution-apis/blob/main/src/engine/common.md)

## Success Metrics
- Prysm beacon node starts within 30 seconds
- Engine API communication established with Geth
- Multi-node networks achieve consensus
- Validator produces blocks when configured
- All tests pass with >80% coverage
- Memory usage under 2GB per node pair