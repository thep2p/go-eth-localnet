# Step 3: Multi-Node Consensus - Enable Proper Consensus Across Multiple EL+CL Node Pairs

## Overview

With Engine API foundation (Step 1) and Prysm launcher (Step 2) complete, this issue implements true multi-node consensus where multiple EL+CL pairs participate in a coordinated blockchain network. This involves proper validator distribution, network topology setup, and ensuring all nodes maintain consensus on the canonical chain.

This step transforms our single-node development setup into a true distributed system where multiple consensus clients coordinate block production, attestations, and finalization - just like mainnet Ethereum.

## Why This Matters

- **Distributed Testing**: Test scenarios requiring multiple validators and network partitions
- **Consensus Testing**: Verify fork choice rules, reorganizations, and finality
- **Production Simulation**: Mirror mainnet's distributed validator setup
- **Fault Tolerance**: Test network resilience with node failures

## Acceptance Criteria

- [ ] Multiple EL+CL pairs form a single network with shared consensus
- [ ] Validators distributed across nodes participate in block production
- [ ] Proper peer discovery and connection between all nodes
- [ ] Fork choice and finalization work across the network
- [ ] Network handles node additions/removals gracefully
- [ ] Tests verify multi-node consensus scenarios

## Implementation Tasks

### 1. Multi-Node Configuration Builder
```go
// internal/consensus/network.go
package consensus

import (
    "fmt"
    "time"

    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
)

// NetworkConfig defines configuration for a multi-node consensus network.
type NetworkConfig struct {
    // Number of nodes in the network
    NodeCount int

    // Validator distribution across nodes
    ValidatorDistribution []int // e.g., [4, 4, 4, 4] for 16 validators across 4 nodes

    // Network topology
    Topology NetworkTopology

    // Genesis configuration
    GenesisTime    time.Time
    ChainID        uint64
    DepositAmount  uint64 // Amount to deposit per validator (32 ETH default)

    // Network parameters
    SlotDuration   time.Duration
    SlotsPerEpoch  uint64
    EpochsPerSync  uint64
}

// NetworkTopology defines how nodes connect to each other.
type NetworkTopology string

const (
    // FullMesh means every node connects to every other node.
    TopologyFullMesh NetworkTopology = "full-mesh"

    // Star means all nodes connect to a central bootstrap node.
    TopologyStar NetworkTopology = "star"

    // Ring means nodes connect in a circular pattern.
    TopologyRing NetworkTopology = "ring"

    // Random means random connections up to max peers.
    TopologyRandom NetworkTopology = "random"
)

// BuildNodeConfigs generates individual node configurations for the network.
func BuildNodeConfigs(netCfg NetworkConfig, baseDataDir string, portAllocator func() int) ([]Config, error) {
    if netCfg.NodeCount <= 0 {
        return nil, fmt.Errorf("node count must be positive")
    }

    // Generate shared genesis data
    genesisStateRoot := generateGenesisStateRoot(netCfg)

    configs := make([]Config, netCfg.NodeCount)

    // First pass: Create base configurations
    for i := 0; i < netCfg.NodeCount; i++ {
        priv, _ := crypto.GenerateKey()

        configs[i] = Config{
            Client:      "prysm",
            DataDir:     fmt.Sprintf("%s/node%d", baseDataDir, i),
            ChainID:     netCfg.ChainID,
            GenesisTime: netCfg.GenesisTime,
            GenesisRoot: genesisStateRoot,

            // Allocate ports
            BeaconPort: portAllocator(),
            P2PPort:    portAllocator(),
            RPCPort:    portAllocator(),

            // P2P identity
            PrivateKey: priv,

            // Will be set based on validator distribution
            ValidatorKeys: []string{},
        }
    }

    // Distribute validators
    if err := distributeValidators(configs, netCfg.ValidatorDistribution); err != nil {
        return nil, fmt.Errorf("distribute validators: %w", err)
    }

    // Configure network topology
    if err := configureTopology(configs, netCfg.Topology); err != nil {
        return nil, fmt.Errorf("configure topology: %w", err)
    }

    return configs, nil
}

// distributeValidators assigns validator keys to nodes based on distribution.
func distributeValidators(configs []Config, distribution []int) error {
    if len(distribution) == 0 {
        // Default: all validators on first node
        configs[0].ValidatorKeys = generateValidatorKeys(32)
        return nil
    }

    if len(distribution) != len(configs) {
        return fmt.Errorf("distribution length %d doesn't match node count %d",
            len(distribution), len(configs))
    }

    for i, count := range distribution {
        if count > 0 {
            configs[i].ValidatorKeys = generateValidatorKeys(count)
        }
    }

    return nil
}

// configureTopology sets up peer connections based on topology.
func configureTopology(configs []Config, topology NetworkTopology) error {
    switch topology {
    case TopologyFullMesh:
        return configureFullMesh(configs)
    case TopologyStar:
        return configureStar(configs)
    case TopologyRing:
        return configureRing(configs)
    case TopologyRandom:
        return configureRandom(configs)
    default:
        return fmt.Errorf("unknown topology: %s", topology)
    }
}

// configureFullMesh connects every node to every other node.
func configureFullMesh(configs []Config) error {
    enrs := make([]string, len(configs))

    // Generate ENRs for all nodes
    for i, cfg := range configs {
        enrs[i] = generateENR(cfg)
    }

    // Connect each node to all others
    for i := range configs {
        configs[i].Bootnodes = make([]string, 0, len(enrs)-1)
        configs[i].StaticPeers = make([]string, 0, len(enrs)-1)

        for j, enr := range enrs {
            if i != j {
                configs[i].Bootnodes = append(configs[i].Bootnodes, enr)
                configs[i].StaticPeers = append(configs[i].StaticPeers, enr)
            }
        }
    }

    return nil
}

// configureStar connects all nodes to the first node (bootstrap).
func configureStar(configs []Config) error {
    if len(configs) < 2 {
        return nil // No connections needed for single node
    }

    bootstrapENR := generateENR(configs[0])

    for i := 1; i < len(configs); i++ {
        configs[i].Bootnodes = []string{bootstrapENR}
        configs[i].StaticPeers = []string{bootstrapENR}
    }

    return nil
}
```

### 2. Enhanced Manager with CL Support
```go
// internal/node/consensus_manager.go
package node

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/rs/zerolog"
    "github.com/thep2p/go-eth-localnet/internal/consensus"
)

// ConsensusManager extends Manager to handle both EL and CL nodes.
type ConsensusManager struct {
    *Manager

    clClients []consensus.Client
    clConfigs []consensus.Config

    mu sync.RWMutex
}

// NewConsensusManager creates a manager for EL+CL node pairs.
func NewConsensusManager(
    logger zerolog.Logger,
    launcher *Launcher,
    clLauncher consensus.Launcher,
    baseDataDir string,
    assignNewPort func() int,
) *ConsensusManager {
    return &ConsensusManager{
        Manager: NewNodeManager(logger, launcher, baseDataDir, assignNewPort),
        clClients: make([]consensus.Client, 0),
        clConfigs: make([]consensus.Config, 0),
    }
}

// StartConsensusNetwork launches a multi-node network with consensus.
func (cm *ConsensusManager) StartConsensusNetwork(
    ctx context.Context,
    netCfg consensus.NetworkConfig,
    elOpts ...LaunchOption,
) error {
    cm.logger.Info().
        Int("node_count", netCfg.NodeCount).
        Str("topology", string(netCfg.Topology)).
        Msg("Starting consensus network")

    // Build CL configurations
    clConfigs, err := consensus.BuildNodeConfigs(netCfg, cm.baseDataDir, cm.assignNewPort)
    if err != nil {
        return fmt.Errorf("build CL configs: %w", err)
    }

    // Start EL nodes with Engine API
    elOpts = append(elOpts, WithEngineAPI())
    if err := cm.Start(ctx, netCfg.NodeCount, elOpts...); err != nil {
        return fmt.Errorf("start EL nodes: %w", err)
    }

    // Wait for all EL nodes to be ready
    for i := 0; i < netCfg.NodeCount; i++ {
        if err := cm.waitForELReady(ctx, i); err != nil {
            return fmt.Errorf("EL node %d not ready: %w", i, err)
        }
    }

    // Configure and start CL clients
    var wg sync.WaitGroup
    errCh := make(chan error, netCfg.NodeCount)

    for i := 0; i < netCfg.NodeCount; i++ {
        wg.Add(1)
        go func(nodeIndex int) {
            defer wg.Done()

            // Update CL config with Engine API details
            clConfigs[nodeIndex].EngineEndpoint = fmt.Sprintf("http://127.0.0.1:%d", cm.GetEnginePort(nodeIndex))
            clConfigs[nodeIndex].JWTSecret = cm.GetJWTSecretBytes(nodeIndex)

            // Launch CL client
            client, err := cm.clLauncher.Launch(clConfigs[nodeIndex])
            if err != nil {
                errCh <- fmt.Errorf("launch CL node %d: %w", nodeIndex, err)
                return
            }

            // Start CL client
            if err := client.Start(ctx); err != nil {
                errCh <- fmt.Errorf("start CL node %d: %w", nodeIndex, err)
                return
            }

            cm.mu.Lock()
            cm.clClients = append(cm.clClients, client)
            cm.clConfigs = append(cm.clConfigs, clConfigs[nodeIndex])
            cm.mu.Unlock()

            cm.logger.Info().Int("node", nodeIndex).Msg("CL client started")
        }(i)
    }

    // Wait for all CL clients to start
    wg.Wait()
    close(errCh)

    // Check for errors
    for err := range errCh {
        if err != nil {
            return err
        }
    }

    // Wait for consensus formation
    if err := cm.waitForConsensus(ctx, 30*time.Second); err != nil {
        return fmt.Errorf("consensus formation failed: %w", err)
    }

    cm.logger.Info().Msg("Consensus network started successfully")
    return nil
}

// waitForConsensus waits for all nodes to agree on the chain head.
func (cm *ConsensusManager) waitForConsensus(ctx context.Context, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)

    for time.Now().Before(deadline) {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        // Check if all CL clients are ready
        allReady := true
        for _, client := range cm.clClients {
            if !client.Ready() {
                allReady = false
                break
            }
        }

        if !allReady {
            time.Sleep(1 * time.Second)
            continue
        }

        // Check if all nodes agree on head slot
        var headSlot uint64
        consensus := true

        for i, client := range cm.clClients {
            metrics, err := client.Metrics()
            if err != nil {
                continue
            }

            if i == 0 {
                headSlot = metrics.HeadSlot
            } else if metrics.HeadSlot != headSlot {
                consensus = false
                break
            }
        }

        if consensus && headSlot > 0 {
            return nil
        }

        time.Sleep(1 * time.Second)
    }

    return fmt.Errorf("timeout waiting for consensus")
}

// GetCLClient returns the CL client at the specified index.
func (cm *ConsensusManager) GetCLClient(index int) consensus.Client {
    cm.mu.RLock()
    defer cm.mu.RUnlock()

    if index < 0 || index >= len(cm.clClients) {
        return nil
    }
    return cm.clClients[index]
}

// GetValidatorDistribution returns how validators are distributed across nodes.
func (cm *ConsensusManager) GetValidatorDistribution() map[int]int {
    cm.mu.RLock()
    defer cm.mu.RUnlock()

    distribution := make(map[int]int)
    for i, client := range cm.clClients {
        distribution[i] = len(client.ValidatorKeys())
    }
    return distribution
}
```

### 3. Validator Management and Distribution
```go
// internal/consensus/validators.go
package consensus

import (
    "crypto/rand"
    "fmt"
    "math/big"

    "github.com/ethereum/go-ethereum/common"
    "github.com/prysmaticlabs/prysm/v5/crypto/bls"
)

// ValidatorSet represents a set of validators for the network.
type ValidatorSet struct {
    Keys       []*bls.SecretKey
    PublicKeys [][]byte
    Indices    []uint64
}

// GenerateValidatorSet creates a new validator set with the specified count.
func GenerateValidatorSet(count int) (*ValidatorSet, error) {
    set := &ValidatorSet{
        Keys:       make([]*bls.SecretKey, count),
        PublicKeys: make([][]byte, count),
        Indices:    make([]uint64, count),
    }

    for i := 0; i < count; i++ {
        // Generate BLS key
        key, err := bls.RandKey()
        if err != nil {
            return nil, fmt.Errorf("generate key %d: %w", i, err)
        }

        set.Keys[i] = key
        set.PublicKeys[i] = key.PublicKey().Marshal()
        set.Indices[i] = uint64(i)
    }

    return set, nil
}

// DistributeValidators splits validators across nodes according to strategy.
type DistributionStrategy string

const (
    // Even distributes validators evenly across all nodes.
    DistributeEven DistributionStrategy = "even"

    // Weighted gives more validators to early nodes.
    DistributeWeighted DistributionStrategy = "weighted"

    // Single puts all validators on one node.
    DistributeSingle DistributionStrategy = "single"

    // Random distributes validators randomly.
    DistributeRandom DistributionStrategy = "random"
)

// DistributeValidatorSet assigns validators from a set to nodes.
func DistributeValidatorSet(
    validatorSet *ValidatorSet,
    nodeCount int,
    strategy DistributionStrategy,
) ([][]string, error) {
    if nodeCount <= 0 {
        return nil, fmt.Errorf("node count must be positive")
    }

    distribution := make([][]string, nodeCount)

    switch strategy {
    case DistributeEven:
        validatorsPerNode := len(validatorSet.Keys) / nodeCount
        remainder := len(validatorSet.Keys) % nodeCount

        idx := 0
        for i := 0; i < nodeCount; i++ {
            count := validatorsPerNode
            if i < remainder {
                count++
            }

            distribution[i] = make([]string, count)
            for j := 0; j < count; j++ {
                distribution[i][j] = encodeValidatorKey(validatorSet.Keys[idx])
                idx++
            }
        }

    case DistributeWeighted:
        // Give more validators to earlier nodes (2:1 ratio)
        total := len(validatorSet.Keys)
        for i := 0; i < nodeCount && total > 0; i++ {
            weight := nodeCount - i
            count := (total * weight) / ((nodeCount * (nodeCount + 1)) / 2)
            if i == nodeCount-1 {
                count = total // Give remaining to last node
            }

            distribution[i] = make([]string, count)
            for j := 0; j < count; j++ {
                distribution[i][j] = encodeValidatorKey(validatorSet.Keys[total-count+j])
            }
            total -= count
        }

    case DistributeSingle:
        // All validators on first node
        distribution[0] = make([]string, len(validatorSet.Keys))
        for i, key := range validatorSet.Keys {
            distribution[0][i] = encodeValidatorKey(key)
        }

    case DistributeRandom:
        // Randomly assign each validator
        for i, key := range validatorSet.Keys {
            nodeIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(nodeCount)))
            node := int(nodeIdx.Int64())
            distribution[node] = append(distribution[node], encodeValidatorKey(key))
        }

    default:
        return nil, fmt.Errorf("unknown distribution strategy: %s", strategy)
    }

    return distribution, nil
}
```

### 4. Network Testing Utilities
```go
// internal/testutils/consensus.go
package testutils

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/require"
    "github.com/thep2p/go-eth-localnet/internal/consensus"
    "github.com/thep2p/go-eth-localnet/internal/node"
)

// StartConsensusNetwork is a test helper for starting EL+CL networks.
func StartConsensusNetwork(
    t *testing.T,
    nodeCount int,
    validatorCount int,
    topology consensus.NetworkTopology,
) (context.Context, context.CancelFunc, *node.ConsensusManager) {
    t.Helper()

    tmp := NewTempDir(t)
    launcher := node.NewLauncher(Logger(t))
    clLauncher := consensus.DefaultRegistry.Get("prysm")

    manager := node.NewConsensusManager(
        Logger(t),
        launcher,
        clLauncher,
        tmp.Path(),
        func() int { return NewPort(t) },
    )

    ctx, cancel := context.WithCancel(context.Background())
    t.Cleanup(tmp.Remove)
    t.Cleanup(func() {
        RequireCallMustReturnWithinTimeout(
            t, manager.Done, node.ShutdownTimeout, "network shutdown failed",
        )
    })

    // Build network configuration
    netCfg := consensus.NetworkConfig{
        NodeCount:     nodeCount,
        Topology:      topology,
        GenesisTime:   time.Now(),
        ChainID:       1337,
        SlotDuration:  2 * time.Second,
        SlotsPerEpoch: 6,
    }

    // Distribute validators evenly
    if validatorCount > 0 {
        netCfg.ValidatorDistribution = make([]int, nodeCount)
        for i := 0; i < validatorCount; i++ {
            netCfg.ValidatorDistribution[i%nodeCount]++
        }
    }

    require.NoError(t, manager.StartConsensusNetwork(ctx, netCfg))

    return ctx, cancel, manager
}

// WaitForSlots waits for the specified number of slots to pass.
func WaitForSlots(t *testing.T, manager *node.ConsensusManager, slots uint64) {
    t.Helper()

    initialSlot := GetCurrentSlot(t, manager, 0)
    targetSlot := initialSlot + slots

    require.Eventually(t, func() bool {
        currentSlot := GetCurrentSlot(t, manager, 0)
        return currentSlot >= targetSlot
    }, time.Duration(slots*3)*time.Second, 500*time.Millisecond)
}

// GetCurrentSlot returns the current slot from a CL client.
func GetCurrentSlot(t *testing.T, manager *node.ConsensusManager, nodeIndex int) uint64 {
    t.Helper()

    client := manager.GetCLClient(nodeIndex)
    require.NotNil(t, client)

    metrics, err := client.Metrics()
    require.NoError(t, err)

    return metrics.CurrentSlot
}

// AssertConsensus verifies all nodes agree on the chain head.
func AssertConsensus(t *testing.T, manager *node.ConsensusManager) {
    t.Helper()

    nodeCount := manager.NodeCount()
    require.Greater(t, nodeCount, 1, "need at least 2 nodes for consensus check")

    var headSlot uint64
    for i := 0; i < nodeCount; i++ {
        metrics, err := manager.GetCLClient(i).Metrics()
        require.NoError(t, err)

        if i == 0 {
            headSlot = metrics.HeadSlot
        } else {
            require.Equal(t, headSlot, metrics.HeadSlot,
                "node %d has different head slot", i)
        }
    }
}
```

## Testing Requirements

### 1. Test Basic Multi-Node Consensus
```go
// internal/node/consensus_test.go
func TestMultiNodeConsensusFormation(t *testing.T) {
    ctx, cancel, manager := testutils.StartConsensusNetwork(t, 3, 12,
        consensus.TopologyFullMesh)
    defer cancel()

    // Wait for some slots to pass
    testutils.WaitForSlots(t, manager, 10)

    // Verify all nodes agree on head
    testutils.AssertConsensus(t, manager)

    // Verify validator distribution
    dist := manager.GetValidatorDistribution()
    require.Equal(t, 3, len(dist))

    totalValidators := 0
    for _, count := range dist {
        totalValidators += count
    }
    require.Equal(t, 12, totalValidators)
}
```

### 2. Test Different Network Topologies
```go
func TestNetworkTopologies(t *testing.T) {
    topologies := []consensus.NetworkTopology{
        consensus.TopologyFullMesh,
        consensus.TopologyStar,
        consensus.TopologyRing,
    }

    for _, topology := range topologies {
        t.Run(string(topology), func(t *testing.T) {
            ctx, cancel, manager := testutils.StartConsensusNetwork(t, 4, 16, topology)
            defer cancel()

            // Verify peer connections based on topology
            for i := 0; i < 4; i++ {
                metrics, err := manager.GetCLClient(i).Metrics()
                require.NoError(t, err)

                switch topology {
                case consensus.TopologyFullMesh:
                    require.GreaterOrEqual(t, metrics.PeerCount, 3)
                case consensus.TopologyStar:
                    if i == 0 {
                        require.GreaterOrEqual(t, metrics.PeerCount, 3)
                    } else {
                        require.GreaterOrEqual(t, metrics.PeerCount, 1)
                    }
                case consensus.TopologyRing:
                    require.GreaterOrEqual(t, metrics.PeerCount, 2)
                }
            }

            // All topologies should achieve consensus
            testutils.WaitForSlots(t, manager, 5)
            testutils.AssertConsensus(t, manager)
        })
    }
}
```

### 3. Test Validator Distribution Strategies
```go
func TestValidatorDistribution(t *testing.T) {
    strategies := []consensus.DistributionStrategy{
        consensus.DistributeEven,
        consensus.DistributeWeighted,
        consensus.DistributeSingle,
    }

    for _, strategy := range strategies {
        t.Run(string(strategy), func(t *testing.T) {
            // Create validator set
            validatorSet, err := consensus.GenerateValidatorSet(20)
            require.NoError(t, err)

            // Distribute across 4 nodes
            distribution, err := consensus.DistributeValidatorSet(
                validatorSet, 4, strategy)
            require.NoError(t, err)

            // Verify distribution
            total := 0
            for i, keys := range distribution {
                t.Logf("Node %d: %d validators", i, len(keys))
                total += len(keys)
            }
            require.Equal(t, 20, total)

            // Test expected distribution patterns
            switch strategy {
            case consensus.DistributeEven:
                for _, keys := range distribution {
                    require.InDelta(t, 5, len(keys), 1)
                }
            case consensus.DistributeSingle:
                require.Equal(t, 20, len(distribution[0]))
                for i := 1; i < 4; i++ {
                    require.Empty(t, distribution[i])
                }
            }
        })
    }
}
```

### 4. Test Network Resilience
```go
func TestNodeFailureRecovery(t *testing.T) {
    ctx, cancel, manager := testutils.StartConsensusNetwork(t, 5, 20,
        consensus.TopologyFullMesh)
    defer cancel()

    // Wait for initial consensus
    testutils.WaitForSlots(t, manager, 5)
    testutils.AssertConsensus(t, manager)

    // Stop one node (non-validator)
    nodeToStop := 3
    require.NoError(t, manager.GetCLClient(nodeToStop).Stop())
    require.NoError(t, manager.GetNode(nodeToStop).Close())

    // Network should continue producing blocks
    initialSlot := testutils.GetCurrentSlot(t, manager, 0)
    testutils.WaitForSlots(t, manager, 5)
    newSlot := testutils.GetCurrentSlot(t, manager, 0)
    require.Greater(t, newSlot, initialSlot)

    // Remaining nodes should maintain consensus
    for i := 0; i < 5; i++ {
        if i == nodeToStop {
            continue
        }
        metrics, err := manager.GetCLClient(i).Metrics()
        if err == nil {
            require.Equal(t, newSlot, metrics.CurrentSlot)
        }
    }
}
```

## Technical Considerations

### Important Notes
- **Genesis Coordination**: All nodes must use identical genesis configuration
- **Time Synchronization**: Nodes must have synchronized clocks for slot timing
- **Validator Key Management**: Keys must be unique across the network
- **Resource Requirements**: Each node pair needs ~2.5GB RAM

### Gotchas
1. **Peer Discovery**: Initial peer connections can take 10-30 seconds
2. **Slot Timing**: First few slots may be skipped during startup
3. **Finalization**: Requires 2/3 of validators for finalization
4. **Port Exhaustion**: Large networks need many ports (3 per CL, 3 per EL)
5. **Genesis Delay**: All nodes must start before genesis time

### Performance Considerations
- Use minimal preset for faster slot times in testing
- Limit validator count for resource-constrained environments
- Consider topology impact on consensus speed
- Monitor memory usage with many nodes

## Success Metrics
- Multi-node networks achieve consensus within 30 seconds
- All configured validators participate in attestations
- Different topologies maintain consensus
- Network handles single node failures gracefully
- Tests cover >85% of consensus code paths
- Documentation includes topology selection guide