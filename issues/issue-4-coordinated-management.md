# Step 4: Coordinated Node Management - Unified Control Over EL+CL Pairs

## Overview

With multi-node consensus working (Step 3), this issue implements sophisticated orchestration for EL+CL node pairs. This includes synchronized lifecycle management, health monitoring, dynamic network modifications, and unified APIs for controlling the entire network as a cohesive unit.

The goal is to make managing complex multi-node Ethereum networks as simple as managing a single node, with all the complexity abstracted away behind clean, intuitive APIs.

## Why This Matters

- **Simplified Testing**: Complex test scenarios become easy to set up and tear down
- **Reliability**: Coordinated health checks and automatic recovery
- **Flexibility**: Dynamic network modifications without full restarts
- **Observability**: Unified metrics and logging across all components
- **Developer Experience**: Clean APIs hide complexity of distributed systems

## Acceptance Criteria

- [ ] Unified lifecycle management for EL+CL pairs
- [ ] Health monitoring with automatic issue detection
- [ ] Dynamic node addition/removal from running networks
- [ ] Coordinated configuration updates across nodes
- [ ] Unified API for network-wide operations
- [ ] Comprehensive observability and debugging tools
- [ ] Tests verify all coordination scenarios

## Implementation Tasks

### 1. Node Pair Abstraction
```go
// internal/node/pair.go
package node

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/ethereum/go-ethereum/common"
    gethnode "github.com/ethereum/go-ethereum/node"
    "github.com/rs/zerolog"
    "github.com/thep2p/go-eth-localnet/internal/consensus"
)

// Pair represents a coordinated EL+CL node pair.
type Pair struct {
    ID       string
    ELNode   *gethnode.Node
    CLClient consensus.Client
    Config   PairConfig

    logger   zerolog.Logger
    health   *HealthMonitor
    mu       sync.RWMutex
    state    PairState
}

// PairConfig holds configuration for a node pair.
type PairConfig struct {
    ELConfig model.Config
    CLConfig consensus.Config
    Role     NodeRole
}

// NodeRole defines the role of a node pair in the network.
type NodeRole string

const (
    RoleValidator  NodeRole = "validator"  // Participates in consensus
    RoleFullNode   NodeRole = "full"       // Syncs but doesn't validate
    RoleBootstrap  NodeRole = "bootstrap"  // Helps with peer discovery
    RoleArchive    NodeRole = "archive"    // Stores full history
)

// PairState represents the current state of a node pair.
type PairState string

const (
    StateInitializing PairState = "initializing"
    StateStarting     PairState = "starting"
    StateRunning      PairState = "running"
    StateSyncing      PairState = "syncing"
    StateStopping     PairState = "stopping"
    StateStopped      PairState = "stopped"
    StateError        PairState = "error"
)

// NewPair creates a new coordinated node pair.
func NewPair(id string, cfg PairConfig, logger zerolog.Logger) *Pair {
    return &Pair{
        ID:     id,
        Config: cfg,
        logger: logger.With().Str("pair", id).Logger(),
        state:  StateInitializing,
        health: NewHealthMonitor(logger),
    }
}

// Start launches both EL and CL nodes in coordinated fashion.
func (p *Pair) Start(ctx context.Context, launcher *Launcher, clLauncher consensus.Launcher) error {
    p.mu.Lock()
    p.state = StateStarting
    p.mu.Unlock()

    // Start EL node first
    p.logger.Info().Msg("Starting execution layer")
    elNode, err := launcher.Launch(p.Config.ELConfig)
    if err != nil {
        p.setError(fmt.Errorf("launch EL: %w", err))
        return err
    }
    p.ELNode = elNode

    // Wait for EL to be ready
    if err := p.waitForELReady(ctx); err != nil {
        p.setError(fmt.Errorf("EL not ready: %w", err))
        return err
    }

    // Update CL config with EL details
    p.Config.CLConfig.EngineEndpoint = fmt.Sprintf("http://127.0.0.1:%d", p.Config.ELConfig.EnginePort)
    p.Config.CLConfig.JWTSecret = p.Config.ELConfig.JWTSecretBytes

    // Start CL client
    p.logger.Info().Msg("Starting consensus layer")
    clClient, err := clLauncher.Launch(p.Config.CLConfig)
    if err != nil {
        p.setError(fmt.Errorf("launch CL: %w", err))
        return err
    }
    p.CLClient = clClient

    if err := clClient.Start(ctx); err != nil {
        p.setError(fmt.Errorf("start CL: %w", err))
        return err
    }

    // Start health monitoring
    p.health.Start(ctx, p)

    p.mu.Lock()
    p.state = StateRunning
    p.mu.Unlock()

    p.logger.Info().Msg("Node pair started successfully")
    return nil
}

// Stop gracefully shuts down both nodes.
func (p *Pair) Stop() error {
    p.mu.Lock()
    if p.state == StateStopped || p.state == StateStopping {
        p.mu.Unlock()
        return nil
    }
    p.state = StateStopping
    p.mu.Unlock()

    p.logger.Info().Msg("Stopping node pair")

    // Stop health monitoring
    p.health.Stop()

    var errs []error

    // Stop CL first (it depends on EL)
    if p.CLClient != nil {
        if err := p.CLClient.Stop(); err != nil {
            errs = append(errs, fmt.Errorf("stop CL: %w", err))
        }
    }

    // Then stop EL
    if p.ELNode != nil {
        if err := p.ELNode.Close(); err != nil {
            errs = append(errs, fmt.Errorf("stop EL: %w", err))
        }
    }

    p.mu.Lock()
    p.state = StateStopped
    p.mu.Unlock()

    if len(errs) > 0 {
        return fmt.Errorf("stop errors: %v", errs)
    }
    return nil
}

// GetState returns the current state of the pair.
func (p *Pair) GetState() PairState {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return p.state
}

// IsHealthy returns true if both nodes are healthy.
func (p *Pair) IsHealthy() bool {
    return p.health.IsHealthy()
}

// GetMetrics returns combined metrics from both nodes.
func (p *Pair) GetMetrics() (*PairMetrics, error) {
    clMetrics, err := p.CLClient.Metrics()
    if err != nil {
        return nil, fmt.Errorf("get CL metrics: %w", err)
    }

    // Get EL metrics via RPC
    elMetrics, err := p.getELMetrics()
    if err != nil {
        return nil, fmt.Errorf("get EL metrics: %w", err)
    }

    return &PairMetrics{
        ID:           p.ID,
        State:        p.GetState(),
        Role:         p.Config.Role,
        CLMetrics:    clMetrics,
        ELMetrics:    elMetrics,
        HealthStatus: p.health.GetStatus(),
    }, nil
}

// setError sets the pair state to error.
func (p *Pair) setError(err error) {
    p.mu.Lock()
    p.state = StateError
    p.mu.Unlock()
    p.logger.Error().Err(err).Msg("Node pair error")
}
```

### 2. Health Monitoring System
```go
// internal/node/health.go
package node

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/rs/zerolog"
)

// HealthMonitor tracks the health of a node pair.
type HealthMonitor struct {
    logger zerolog.Logger

    mu           sync.RWMutex
    status       HealthStatus
    lastCheck    time.Time
    checkResults map[string]CheckResult

    stopCh chan struct{}
    done   chan struct{}
}

// HealthStatus represents overall health.
type HealthStatus string

const (
    HealthHealthy   HealthStatus = "healthy"
    HealthDegraded  HealthStatus = "degraded"
    HealthUnhealthy HealthStatus = "unhealthy"
    HealthUnknown   HealthStatus = "unknown"
)

// CheckResult represents a single health check result.
type CheckResult struct {
    Name      string
    Healthy   bool
    Message   string
    Timestamp time.Time
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor(logger zerolog.Logger) *HealthMonitor {
    return &HealthMonitor{
        logger:       logger.With().Str("component", "health-monitor").Logger(),
        status:       HealthUnknown,
        checkResults: make(map[string]CheckResult),
        stopCh:       make(chan struct{}),
        done:         make(chan struct{}),
    }
}

// Start begins health monitoring for a node pair.
func (h *HealthMonitor) Start(ctx context.Context, pair *Pair) {
    go h.monitorLoop(ctx, pair)
}

// Stop halts health monitoring.
func (h *HealthMonitor) Stop() {
    close(h.stopCh)
    <-h.done
}

// monitorLoop continuously checks node health.
func (h *HealthMonitor) monitorLoop(ctx context.Context, pair *Pair) {
    defer close(h.done)

    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    // Immediate first check
    h.runHealthChecks(ctx, pair)

    for {
        select {
        case <-ctx.Done():
            return
        case <-h.stopCh:
            return
        case <-ticker.C:
            h.runHealthChecks(ctx, pair)
        }
    }
}

// runHealthChecks performs all health checks.
func (h *HealthMonitor) runHealthChecks(ctx context.Context, pair *Pair) {
    checks := []struct {
        name string
        fn   func(context.Context, *Pair) (bool, string)
    }{
        {"el_running", checkELRunning},
        {"el_peers", checkELPeers},
        {"el_syncing", checkELSyncing},
        {"cl_running", checkCLRunning},
        {"cl_peers", checkCLPeers},
        {"cl_syncing", checkCLSyncing},
        {"engine_api", checkEngineAPI},
    }

    results := make(map[string]CheckResult)
    allHealthy := true
    anyUnhealthy := false

    for _, check := range checks {
        healthy, message := check.fn(ctx, pair)
        results[check.name] = CheckResult{
            Name:      check.name,
            Healthy:   healthy,
            Message:   message,
            Timestamp: time.Now(),
        }

        if !healthy {
            allHealthy = false
            if check.name == "el_running" || check.name == "cl_running" {
                anyUnhealthy = true
            }
        }
    }

    h.mu.Lock()
    h.checkResults = results
    h.lastCheck = time.Now()

    if anyUnhealthy {
        h.status = HealthUnhealthy
    } else if allHealthy {
        h.status = HealthHealthy
    } else {
        h.status = HealthDegraded
    }
    h.mu.Unlock()

    if h.status != HealthHealthy {
        h.logger.Warn().Str("status", string(h.status)).Msg("Health check failed")
    }
}

// IsHealthy returns true if the monitored pair is healthy.
func (h *HealthMonitor) IsHealthy() bool {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.status == HealthHealthy
}

// GetStatus returns the current health status.
func (h *HealthMonitor) GetStatus() HealthStatus {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.status
}

// Health check implementations
func checkELRunning(ctx context.Context, pair *Pair) (bool, string) {
    if pair.ELNode == nil {
        return false, "EL node not initialized"
    }
    // Check if node is still running
    return true, "EL node running"
}

func checkCLRunning(ctx context.Context, pair *Pair) (bool, string) {
    if pair.CLClient == nil {
        return false, "CL client not initialized"
    }
    return pair.CLClient.Ready(), "CL client ready"
}

func checkELPeers(ctx context.Context, pair *Pair) (bool, string) {
    // Check EL peer count
    peers := pair.ELNode.Server().PeerCount()
    if pair.Config.Role != RoleBootstrap && peers == 0 {
        return false, fmt.Sprintf("No EL peers connected")
    }
    return true, fmt.Sprintf("%d EL peers", peers)
}

func checkCLPeers(ctx context.Context, pair *Pair) (bool, string) {
    metrics, err := pair.CLClient.Metrics()
    if err != nil {
        return false, fmt.Sprintf("Failed to get CL metrics: %v", err)
    }
    if pair.Config.Role != RoleBootstrap && metrics.PeerCount == 0 {
        return false, "No CL peers connected"
    }
    return true, fmt.Sprintf("%d CL peers", metrics.PeerCount)
}

func checkELSyncing(ctx context.Context, pair *Pair) (bool, string) {
    // Check if EL is synced
    // In local dev, should sync quickly
    return true, "EL synced"
}

func checkCLSyncing(ctx context.Context, pair *Pair) (bool, string) {
    metrics, err := pair.CLClient.Metrics()
    if err != nil {
        return false, fmt.Sprintf("Failed to get CL metrics: %v", err)
    }
    if metrics.IsSyncing && time.Since(pair.health.lastCheck) > 30*time.Second {
        return false, "CL still syncing after 30s"
    }
    return true, fmt.Sprintf("CL at slot %d", metrics.CurrentSlot)
}

func checkEngineAPI(ctx context.Context, pair *Pair) (bool, string) {
    // Verify Engine API communication
    // This would make a test Engine API call
    return true, "Engine API responsive"
}
```

### 3. Network Orchestrator
```go
// internal/node/orchestrator.go
package node

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/rs/zerolog"
    "github.com/thep2p/go-eth-localnet/internal/consensus"
)

// Orchestrator manages a network of node pairs.
type Orchestrator struct {
    logger        zerolog.Logger
    launcher      *Launcher
    clLauncher    consensus.Launcher
    baseDataDir   string
    assignNewPort func() int

    mu       sync.RWMutex
    pairs    map[string]*Pair
    network  *NetworkState
    shutdown chan struct{}
}

// NetworkState represents the current state of the network.
type NetworkState struct {
    GenesisTime    time.Time
    ChainID        uint64
    ValidatorCount int
    NodeCount      int
    Topology       consensus.NetworkTopology
    StartTime      time.Time
}

// NewOrchestrator creates a new network orchestrator.
func NewOrchestrator(
    logger zerolog.Logger,
    launcher *Launcher,
    clLauncher consensus.Launcher,
    baseDataDir string,
    assignNewPort func() int,
) *Orchestrator {
    return &Orchestrator{
        logger:        logger.With().Str("component", "orchestrator").Logger(),
        launcher:      launcher,
        clLauncher:    clLauncher,
        baseDataDir:   baseDataDir,
        assignNewPort: assignNewPort,
        pairs:         make(map[string]*Pair),
        shutdown:      make(chan struct{}),
    }
}

// StartNetwork launches a complete network with the given configuration.
func (o *Orchestrator) StartNetwork(ctx context.Context, cfg NetworkConfig) error {
    o.logger.Info().
        Int("nodes", cfg.NodeCount).
        Str("topology", string(cfg.Topology)).
        Msg("Starting network")

    o.network = &NetworkState{
        GenesisTime:    cfg.GenesisTime,
        ChainID:        cfg.ChainID,
        ValidatorCount: sum(cfg.ValidatorDistribution),
        NodeCount:      cfg.NodeCount,
        Topology:       cfg.Topology,
        StartTime:      time.Now(),
    }

    // Build node configurations
    pairConfigs, err := o.buildPairConfigs(cfg)
    if err != nil {
        return fmt.Errorf("build configs: %w", err)
    }

    // Launch all pairs in parallel with proper ordering
    if err := o.launchPairs(ctx, pairConfigs); err != nil {
        return fmt.Errorf("launch pairs: %w", err)
    }

    // Wait for network to stabilize
    if err := o.waitForNetworkReady(ctx, 30*time.Second); err != nil {
        return fmt.Errorf("network not ready: %w", err)
    }

    o.logger.Info().Msg("Network started successfully")
    return nil
}

// AddNode dynamically adds a new node to the running network.
func (o *Orchestrator) AddNode(ctx context.Context, role NodeRole) (*Pair, error) {
    o.mu.Lock()
    defer o.mu.Unlock()

    if o.network == nil {
        return nil, fmt.Errorf("no network running")
    }

    // Get bootstrap nodes from existing network
    bootstrapNodes := o.getBootstrapNodes()

    // Create configuration for new node
    pairID := fmt.Sprintf("node-%d", len(o.pairs))
    pairCfg := o.createPairConfig(pairID, role, bootstrapNodes)

    // Create and start the pair
    pair := NewPair(pairID, pairCfg, o.logger)
    if err := pair.Start(ctx, o.launcher, o.clLauncher); err != nil {
        return nil, fmt.Errorf("start pair: %w", err)
    }

    o.pairs[pairID] = pair
    o.network.NodeCount++

    o.logger.Info().Str("node", pairID).Str("role", string(role)).Msg("Added node to network")
    return pair, nil
}

// RemoveNode gracefully removes a node from the network.
func (o *Orchestrator) RemoveNode(pairID string) error {
    o.mu.Lock()
    defer o.mu.Unlock()

    pair, exists := o.pairs[pairID]
    if !exists {
        return fmt.Errorf("node %s not found", pairID)
    }

    // Stop the pair
    if err := pair.Stop(); err != nil {
        return fmt.Errorf("stop pair: %w", err)
    }

    delete(o.pairs, pairID)
    o.network.NodeCount--

    o.logger.Info().Str("node", pairID).Msg("Removed node from network")
    return nil
}

// UpdateConfiguration applies configuration changes to running nodes.
func (o *Orchestrator) UpdateConfiguration(pairID string, updates ConfigUpdate) error {
    o.mu.RLock()
    pair, exists := o.pairs[pairID]
    o.mu.RUnlock()

    if !exists {
        return fmt.Errorf("node %s not found", pairID)
    }

    // Apply configuration updates
    // This would involve updating node configurations and potentially restarting services
    o.logger.Info().Str("node", pairID).Msg("Updated configuration")
    return nil
}

// GetNetworkStatus returns the current network status.
func (o *Orchestrator) GetNetworkStatus() *NetworkStatus {
    o.mu.RLock()
    defer o.mu.RUnlock()

    status := &NetworkStatus{
        State:          o.network,
        NodeStatuses:   make(map[string]*PairMetrics),
        HealthySummary: HealthSummary{},
        Uptime:         time.Since(o.network.StartTime),
    }

    healthyCount := 0
    for id, pair := range o.pairs {
        metrics, err := pair.GetMetrics()
        if err == nil {
            status.NodeStatuses[id] = metrics
            if pair.IsHealthy() {
                healthyCount++
            }
        }
    }

    status.HealthySummary.Healthy = healthyCount
    status.HealthySummary.Total = len(o.pairs)
    status.HealthySummary.Percentage = float64(healthyCount) / float64(len(o.pairs)) * 100

    return status
}

// StopNetwork gracefully shuts down the entire network.
func (o *Orchestrator) StopNetwork() error {
    o.logger.Info().Msg("Stopping network")

    var wg sync.WaitGroup
    errCh := make(chan error, len(o.pairs))

    o.mu.RLock()
    pairs := make([]*Pair, 0, len(o.pairs))
    for _, pair := range o.pairs {
        pairs = append(pairs, pair)
    }
    o.mu.RUnlock()

    // Stop all pairs in parallel
    for _, pair := range pairs {
        wg.Add(1)
        go func(p *Pair) {
            defer wg.Done()
            if err := p.Stop(); err != nil {
                errCh <- fmt.Errorf("stop %s: %w", p.ID, err)
            }
        }(pair)
    }

    wg.Wait()
    close(errCh)

    // Collect errors
    var errs []error
    for err := range errCh {
        errs = append(errs, err)
    }

    o.mu.Lock()
    o.pairs = make(map[string]*Pair)
    o.network = nil
    o.mu.Unlock()

    if len(errs) > 0 {
        return fmt.Errorf("stop errors: %v", errs)
    }

    close(o.shutdown)
    o.logger.Info().Msg("Network stopped")
    return nil
}

// Wait blocks until the network is shut down.
func (o *Orchestrator) Wait() {
    <-o.shutdown
}
```

### 4. Unified Control API
```go
// internal/node/api.go
package node

import (
    "context"
    "fmt"
    "time"

    "github.com/ethereum/go-ethereum/common"
)

// NetworkAPI provides high-level operations for the network.
type NetworkAPI struct {
    orchestrator *Orchestrator
}

// NewNetworkAPI creates a new network API.
func NewNetworkAPI(orchestrator *Orchestrator) *NetworkAPI {
    return &NetworkAPI{orchestrator: orchestrator}
}

// ExecuteTransaction sends a transaction across the network.
func (api *NetworkAPI) ExecuteTransaction(from, to common.Address, amount *big.Int) (common.Hash, error) {
    // Get a healthy node to send transaction through
    pair := api.getHealthyPair()
    if pair == nil {
        return common.Hash{}, fmt.Errorf("no healthy nodes available")
    }

    // Send transaction via EL RPC
    client, err := ethclient.Dial(fmt.Sprintf("http://127.0.0.1:%d", pair.Config.ELConfig.RPCPort))
    if err != nil {
        return common.Hash{}, err
    }
    defer client.Close()

    // Build and send transaction
    // ... transaction logic ...

    return txHash, nil
}

// WaitForFinalization waits for a block to be finalized.
func (api *NetworkAPI) WaitForFinalization(blockNumber uint64, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return fmt.Errorf("timeout waiting for finalization")
        case <-ticker.C:
            finalized, err := api.getFinalizedBlock()
            if err != nil {
                continue
            }
            if finalized >= blockNumber {
                return nil
            }
        }
    }
}

// SimulateNetworkPartition creates a network partition for testing.
func (api *NetworkAPI) SimulateNetworkPartition(duration time.Duration, partitionGroups [][]string) error {
    // Disconnect nodes in different partition groups
    for _, group1 := range partitionGroups {
        for _, group2 := range partitionGroups {
            if group1 == group2 {
                continue
            }
            for _, node1 := range group1 {
                for _, node2 := range group2 {
                    api.disconnectNodes(node1, node2)
                }
            }
        }
    }

    // Reconnect after duration
    time.AfterFunc(duration, func() {
        api.healPartition(partitionGroups)
    })

    return nil
}

// ScaleNetwork adds or removes nodes dynamically.
func (api *NetworkAPI) ScaleNetwork(targetSize int) error {
    currentSize := api.orchestrator.network.NodeCount

    if targetSize > currentSize {
        // Add nodes
        for i := currentSize; i < targetSize; i++ {
            role := NodeRole(RoleFullNode)
            if i < 4 { // First 4 nodes are validators
                role = RoleValidator
            }
            _, err := api.orchestrator.AddNode(context.Background(), role)
            if err != nil {
                return fmt.Errorf("add node %d: %w", i, err)
            }
        }
    } else if targetSize < currentSize {
        // Remove nodes (starting from the end)
        for i := currentSize - 1; i >= targetSize; i-- {
            pairID := fmt.Sprintf("node-%d", i)
            if err := api.orchestrator.RemoveNode(pairID); err != nil {
                return fmt.Errorf("remove node %s: %w", pairID, err)
            }
        }
    }

    return nil
}

// GetNetworkMetrics returns comprehensive network metrics.
func (api *NetworkAPI) GetNetworkMetrics() (*NetworkMetrics, error) {
    status := api.orchestrator.GetNetworkStatus()

    metrics := &NetworkMetrics{
        NodeCount:       status.State.NodeCount,
        ValidatorCount:  status.State.ValidatorCount,
        HealthyNodes:    status.HealthySummary.Healthy,
        Uptime:          status.Uptime,
        CurrentSlot:     api.getCurrentSlot(),
        FinalizedSlot:   api.getFinalizedSlot(),
        TotalPeers:      api.getTotalPeerCount(),
        BlocksProduced:  api.getBlocksProduced(),
        Attestations:    api.getAttestationCount(),
    }

    return metrics, nil
}
```

## Testing Requirements

### 1. Test Coordinated Lifecycle
```go
// internal/node/orchestrator_test.go
func TestOrchestratorLifecycle(t *testing.T) {
    orchestrator := setupOrchestrator(t)

    ctx := context.Background()
    cfg := NetworkConfig{
        NodeCount:             3,
        ValidatorDistribution: []int{4, 4, 4},
        Topology:              consensus.TopologyFullMesh,
        ChainID:               1337,
        GenesisTime:           time.Now(),
    }

    // Start network
    require.NoError(t, orchestrator.StartNetwork(ctx, cfg))

    // Verify all nodes are healthy
    status := orchestrator.GetNetworkStatus()
    require.Equal(t, 3, status.HealthySummary.Total)
    require.Equal(t, 3, status.HealthySummary.Healthy)

    // Stop network
    require.NoError(t, orchestrator.StopNetwork())

    // Verify all nodes stopped
    status = orchestrator.GetNetworkStatus()
    require.Equal(t, 0, len(status.NodeStatuses))
}
```

### 2. Test Dynamic Node Management
```go
func TestDynamicNodeManagement(t *testing.T) {
    orchestrator := setupOrchestrator(t)
    ctx := context.Background()

    // Start with 2 nodes
    cfg := NetworkConfig{
        NodeCount: 2,
        Topology:  consensus.TopologyFullMesh,
    }
    require.NoError(t, orchestrator.StartNetwork(ctx, cfg))

    // Add a new node
    newPair, err := orchestrator.AddNode(ctx, RoleFullNode)
    require.NoError(t, err)
    require.NotNil(t, newPair)

    // Wait for new node to sync
    require.Eventually(t, func() bool {
        return newPair.IsHealthy()
    }, 30*time.Second, 1*time.Second)

    // Verify network has 3 nodes
    status := orchestrator.GetNetworkStatus()
    require.Equal(t, 3, status.State.NodeCount)

    // Remove the new node
    require.NoError(t, orchestrator.RemoveNode(newPair.ID))

    // Verify network has 2 nodes again
    status = orchestrator.GetNetworkStatus()
    require.Equal(t, 2, status.State.NodeCount)
}
```

### 3. Test Health Monitoring
```go
func TestHealthMonitoring(t *testing.T) {
    orchestrator := setupOrchestrator(t)
    ctx := context.Background()

    cfg := NetworkConfig{NodeCount: 3}
    require.NoError(t, orchestrator.StartNetwork(ctx, cfg))

    // All nodes should be healthy initially
    status := orchestrator.GetNetworkStatus()
    require.Equal(t, 100.0, status.HealthySummary.Percentage)

    // Simulate a node failure
    pair := orchestrator.pairs["node-1"]
    require.NoError(t, pair.CLClient.Stop())

    // Health should degrade
    require.Eventually(t, func() bool {
        status := orchestrator.GetNetworkStatus()
        return status.HealthySummary.Healthy < 3
    }, 10*time.Second, 1*time.Second)

    // Verify specific node is unhealthy
    metrics := status.NodeStatuses["node-1"]
    require.Equal(t, HealthUnhealthy, metrics.HealthStatus)
}
```

### 4. Test Network API
```go
func TestNetworkAPI(t *testing.T) {
    orchestrator := setupOrchestrator(t)
    api := NewNetworkAPI(orchestrator)

    ctx := context.Background()
    cfg := NetworkConfig{
        NodeCount:             4,
        ValidatorDistribution: []int{8, 8, 8, 8},
    }
    require.NoError(t, orchestrator.StartNetwork(ctx, cfg))

    t.Run("GetMetrics", func(t *testing.T) {
        metrics, err := api.GetNetworkMetrics()
        require.NoError(t, err)
        require.Equal(t, 4, metrics.NodeCount)
        require.Equal(t, 32, metrics.ValidatorCount)
        require.Equal(t, 4, metrics.HealthyNodes)
    })

    t.Run("ScaleNetwork", func(t *testing.T) {
        // Scale up
        require.NoError(t, api.ScaleNetwork(5))
        metrics, err := api.GetNetworkMetrics()
        require.NoError(t, err)
        require.Equal(t, 5, metrics.NodeCount)

        // Scale down
        require.NoError(t, api.ScaleNetwork(3))
        metrics, err = api.GetNetworkMetrics()
        require.NoError(t, err)
        require.Equal(t, 3, metrics.NodeCount)
    })

    t.Run("NetworkPartition", func(t *testing.T) {
        // Create partition
        groups := [][]string{
            {"node-0", "node-1"},
            {"node-2"},
        }
        require.NoError(t, api.SimulateNetworkPartition(10*time.Second, groups))

        // Verify partition (nodes in different groups shouldn't see each other)
        // ... verification logic ...

        // Wait for partition to heal
        time.Sleep(11 * time.Second)

        // Verify network is whole again
        // ... verification logic ...
    })
}
```

## Technical Considerations

### Important Notes
- **Resource Management**: Each node pair needs ~3GB RAM total
- **Port Allocation**: Need 6 ports per node pair (3 EL, 3 CL)
- **Synchronization**: Careful ordering when starting/stopping nodes
- **State Consistency**: Health checks must be consistent across queries

### Gotchas
1. **Startup Order**: Bootstrap nodes must start before others
2. **Shutdown Order**: CL must stop before EL (dependency)
3. **Health Check Timing**: Too frequent checks can impact performance
4. **Dynamic Scaling**: Adding validators requires careful coordination
5. **Network Partitions**: May cause consensus issues if not carefully managed

### Performance Optimizations
- Parallel node startup where possible
- Batch configuration updates
- Cache health check results
- Use connection pooling for RPC clients
- Implement circuit breakers for failing nodes

## Success Metrics
- Network startup time under 1 minute for 10 nodes
- Health monitoring detects issues within 10 seconds
- Dynamic node addition completes within 30 seconds
- API operations have <100ms latency
- Zero data loss during graceful shutdown
- Test coverage >85% for orchestration code