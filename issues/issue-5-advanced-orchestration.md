# Step 5: Advanced Orchestration - Production-Like Features

## Overview

With coordinated node management complete (Step 4), this final step adds advanced orchestration features that enable testing of complex, production-like scenarios. This includes validator rotation, network fault injection, fork testing, performance profiling, and comprehensive observability.

These features transform the local Ethereum network into a powerful testing and development platform that can simulate real-world conditions and edge cases.

## Why This Matters

- **Production Readiness**: Test applications against realistic network conditions
- **Edge Case Testing**: Simulate failures, attacks, and extreme conditions
- **Performance Analysis**: Profile and optimize blockchain applications
- **Protocol Testing**: Verify behavior across forks and upgrades
- **Debugging Tools**: Deep visibility into network operations

## Acceptance Criteria

- [ ] Validator rotation and slashing simulation
- [ ] Network fault injection (latency, packet loss, partitions)
- [ ] Fork testing with configurable chain rules
- [ ] Performance profiling and bottleneck detection
- [ ] Time manipulation for testing time-dependent logic
- [ ] Comprehensive event streaming and observability
- [ ] Snapshot and restore functionality
- [ ] Tests verify all advanced features

## Implementation Tasks

### 1. Validator Rotation and Management
```go
// internal/orchestration/validators.go
package orchestration

import (
    "context"
    "fmt"
    "math/big"
    "time"

    "github.com/ethereum/go-ethereum/common"
    "github.com/prysmaticlabs/prysm/v5/crypto/bls"
    "github.com/thep2p/go-eth-localnet/internal/consensus"
)

// ValidatorManager handles advanced validator operations.
type ValidatorManager struct {
    orchestrator *node.Orchestrator
    validators   map[string]*ValidatorInfo
    rotations    []*RotationSchedule
}

// ValidatorInfo tracks validator state and history.
type ValidatorInfo struct {
    PublicKey      string
    Index          uint64
    Balance        *big.Int
    Status         ValidatorStatus
    AssignedNode   string
    ActivationSlot uint64
    ExitSlot       uint64
    SlashedAt      uint64
    Performance    *ValidatorPerformance
}

// ValidatorStatus represents validator state.
type ValidatorStatus string

const (
    StatusPending    ValidatorStatus = "pending"
    StatusActive     ValidatorStatus = "active"
    StatusExiting    ValidatorStatus = "exiting"
    StatusExited     ValidatorStatus = "exited"
    StatusSlashed    ValidatorStatus = "slashed"
)

// ValidatorPerformance tracks validator effectiveness.
type ValidatorPerformance struct {
    ProposedBlocks      uint64
    MissedBlocks        uint64
    Attestations        uint64
    MissedAttestations  uint64
    InclusionDistance   float64
    RewardsEarned       *big.Int
    PenaltiesIncurred   *big.Int
}

// RotationSchedule defines when validators rotate between nodes.
type RotationSchedule struct {
    ValidatorIndex uint64
    FromNode       string
    ToNode         string
    AtSlot         uint64
    Reason         string
}

// NewValidatorManager creates a validator manager.
func NewValidatorManager(orchestrator *node.Orchestrator) *ValidatorManager {
    return &ValidatorManager{
        orchestrator: orchestrator,
        validators:   make(map[string]*ValidatorInfo),
        rotations:    make([]*RotationSchedule, 0),
    }
}

// RotateValidator moves a validator from one node to another.
func (vm *ValidatorManager) RotateValidator(
    ctx context.Context,
    validatorPubKey string,
    targetNodeID string,
) error {
    validator, exists := vm.validators[validatorPubKey]
    if !exists {
        return fmt.Errorf("validator %s not found", validatorPubKey)
    }

    if validator.Status != StatusActive {
        return fmt.Errorf("validator not active: %s", validator.Status)
    }

    currentNode := validator.AssignedNode
    if currentNode == targetNodeID {
        return fmt.Errorf("validator already on node %s", targetNodeID)
    }

    // Stop validator on current node
    if err := vm.stopValidatorOnNode(ctx, validatorPubKey, currentNode); err != nil {
        return fmt.Errorf("stop validator on %s: %w", currentNode, err)
    }

    // Wait for safe rotation point (end of epoch)
    if err := vm.waitForEpochBoundary(ctx); err != nil {
        return fmt.Errorf("wait for epoch: %w", err)
    }

    // Start validator on target node
    if err := vm.startValidatorOnNode(ctx, validatorPubKey, targetNodeID); err != nil {
        return fmt.Errorf("start validator on %s: %w", targetNodeID, err)
    }

    // Update tracking
    validator.AssignedNode = targetNodeID
    vm.recordRotation(validator.Index, currentNode, targetNodeID, "manual rotation")

    return nil
}

// SimulateSlashing marks a validator as slashed for testing.
func (vm *ValidatorManager) SimulateSlashing(validatorPubKey string) error {
    validator, exists := vm.validators[validatorPubKey]
    if !exists {
        return fmt.Errorf("validator not found")
    }

    if validator.Status != StatusActive {
        return fmt.Errorf("can only slash active validators")
    }

    // Mark as slashed
    validator.Status = StatusSlashed
    validator.SlashedAt = vm.getCurrentSlot()

    // Apply slashing penalty
    slashingPenalty := new(big.Int).Div(validator.Balance, big.NewInt(32))
    validator.Balance.Sub(validator.Balance, slashingPenalty)
    validator.Performance.PenaltiesIncurred.Add(
        validator.Performance.PenaltiesIncurred,
        slashingPenalty,
    )

    // Remove from active validator set
    return vm.stopValidatorOnNode(context.Background(),
        validatorPubKey, validator.AssignedNode)
}

// RebalanceValidators redistributes validators across nodes for load balancing.
func (vm *ValidatorManager) RebalanceValidators(ctx context.Context) error {
    // Get current distribution
    distribution := vm.getValidatorDistribution()

    // Calculate optimal distribution
    totalValidators := len(vm.validators)
    nodeCount := vm.orchestrator.GetNetworkStatus().State.NodeCount
    targetPerNode := totalValidators / nodeCount

    // Identify over/under loaded nodes
    overloaded := make([]string, 0)
    underloaded := make([]string, 0)

    for nodeID, count := range distribution {
        if count > targetPerNode+1 {
            overloaded = append(overloaded, nodeID)
        } else if count < targetPerNode {
            underloaded = append(underloaded, nodeID)
        }
    }

    // Move validators from overloaded to underloaded nodes
    for _, fromNode := range overloaded {
        for _, toNode := range underloaded {
            validators := vm.getValidatorsOnNode(fromNode)
            if len(validators) > targetPerNode {
                // Move one validator
                if err := vm.RotateValidator(ctx, validators[0], toNode); err != nil {
                    return fmt.Errorf("rotate validator: %w", err)
                }
            }
        }
    }

    return nil
}

// MonitorPerformance tracks validator performance metrics.
func (vm *ValidatorManager) MonitorPerformance(ctx context.Context) {
    ticker := time.NewTicker(12 * time.Second) // Every epoch
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            vm.updatePerformanceMetrics()
        }
    }
}

// updatePerformanceMetrics updates validator performance data.
func (vm *ValidatorManager) updatePerformanceMetrics() {
    for pubKey, validator := range vm.validators {
        if validator.Status != StatusActive {
            continue
        }

        // Query beacon node for validator performance
        perf := vm.queryValidatorPerformance(pubKey)
        if perf != nil {
            validator.Performance = perf
        }

        // Check for poor performance
        if vm.isPoorPerformer(validator.Performance) {
            vm.handlePoorPerformance(validator)
        }
    }
}

// isPoorPerformer checks if a validator is underperforming.
func (vm *ValidatorManager) isPoorPerformer(perf *ValidatorPerformance) bool {
    if perf == nil {
        return false
    }

    missRate := float64(perf.MissedAttestations) / float64(perf.Attestations)
    return missRate > 0.1 // More than 10% missed attestations
}
```

### 2. Network Fault Injection
```go
// internal/orchestration/faults.go
package orchestration

import (
    "context"
    "fmt"
    "math/rand"
    "time"
)

// FaultInjector simulates network faults and failures.
type FaultInjector struct {
    orchestrator *node.Orchestrator
    activeFaults []*Fault
}

// Fault represents an active network fault.
type Fault struct {
    ID        string
    Type      FaultType
    Target    FaultTarget
    Severity  FaultSeverity
    Duration  time.Duration
    StartTime time.Time
    Config    map[string]interface{}
}

// FaultType defines the type of fault.
type FaultType string

const (
    FaultLatency       FaultType = "latency"
    FaultPacketLoss    FaultType = "packet_loss"
    FaultPartition     FaultType = "partition"
    FaultBandwidth     FaultType = "bandwidth"
    FaultNodeCrash     FaultType = "node_crash"
    FaultClockSkew     FaultType = "clock_skew"
    FaultResourceLimit FaultType = "resource_limit"
)

// FaultTarget specifies what the fault affects.
type FaultTarget struct {
    Nodes       []string // Specific nodes
    Percentage  float64  // Percentage of nodes
    Layer       string   // "execution" or "consensus"
    Direction   string   // "inbound", "outbound", or "both"
}

// FaultSeverity indicates fault impact level.
type FaultSeverity string

const (
    SeverityLow    FaultSeverity = "low"
    SeverityMedium FaultSeverity = "medium"
    SeverityHigh   FaultSeverity = "high"
    SeverityChaos  FaultSeverity = "chaos"
)

// InjectLatency adds network latency between nodes.
func (fi *FaultInjector) InjectLatency(
    ctx context.Context,
    target FaultTarget,
    latencyMs int,
    jitterMs int,
    duration time.Duration,
) error {
    fault := &Fault{
        ID:        generateFaultID(),
        Type:      FaultLatency,
        Target:    target,
        Severity:  fi.calculateSeverity(latencyMs),
        Duration:  duration,
        StartTime: time.Now(),
        Config: map[string]interface{}{
            "latency_ms": latencyMs,
            "jitter_ms":  jitterMs,
        },
    }

    // Apply latency to target nodes
    for _, nodeID := range fi.selectTargetNodes(target) {
        if err := fi.applyLatencyToNode(nodeID, latencyMs, jitterMs); err != nil {
            return fmt.Errorf("apply latency to %s: %w", nodeID, err)
        }
    }

    fi.activeFaults = append(fi.activeFaults, fault)

    // Schedule removal
    time.AfterFunc(duration, func() {
        fi.removeFault(fault.ID)
    })

    return nil
}

// InjectPacketLoss simulates packet loss in the network.
func (fi *FaultInjector) InjectPacketLoss(
    ctx context.Context,
    target FaultTarget,
    lossPercentage float64,
    duration time.Duration,
) error {
    if lossPercentage < 0 || lossPercentage > 100 {
        return fmt.Errorf("loss percentage must be 0-100")
    }

    fault := &Fault{
        ID:        generateFaultID(),
        Type:      FaultPacketLoss,
        Target:    target,
        Severity:  fi.calculateSeverityFromLoss(lossPercentage),
        Duration:  duration,
        StartTime: time.Now(),
        Config: map[string]interface{}{
            "loss_percentage": lossPercentage,
        },
    }

    // Apply packet loss
    for _, nodeID := range fi.selectTargetNodes(target) {
        if err := fi.applyPacketLossToNode(nodeID, lossPercentage); err != nil {
            return fmt.Errorf("apply packet loss to %s: %w", nodeID, err)
        }
    }

    fi.activeFaults = append(fi.activeFaults, fault)

    time.AfterFunc(duration, func() {
        fi.removeFault(fault.ID)
    })

    return nil
}

// CreateNetworkPartition splits the network into isolated groups.
func (fi *FaultInjector) CreateNetworkPartition(
    ctx context.Context,
    partitionGroups [][]string,
    duration time.Duration,
) error {
    fault := &Fault{
        ID:        generateFaultID(),
        Type:      FaultPartition,
        Target:    FaultTarget{},
        Severity:  SeverityHigh,
        Duration:  duration,
        StartTime: time.Now(),
        Config: map[string]interface{}{
            "groups": partitionGroups,
        },
    }

    // Disconnect nodes in different partitions
    for i, group1 := range partitionGroups {
        for j, group2 := range partitionGroups {
            if i >= j {
                continue
            }
            for _, node1 := range group1 {
                for _, node2 := range group2 {
                    fi.disconnectNodes(node1, node2)
                }
            }
        }
    }

    fi.activeFaults = append(fi.activeFaults, fault)

    // Schedule healing
    time.AfterFunc(duration, func() {
        fi.healPartition(partitionGroups)
        fi.removeFault(fault.ID)
    })

    return nil
}

// SimulateCascadingFailure creates a series of related failures.
func (fi *FaultInjector) SimulateCascadingFailure(
    ctx context.Context,
    initialNode string,
    spreadProbability float64,
    maxNodes int,
) error {
    affected := make(map[string]bool)
    queue := []string{initialNode}
    affected[initialNode] = true

    for len(queue) > 0 && len(affected) < maxNodes {
        current := queue[0]
        queue = queue[1:]

        // Crash the current node
        if err := fi.crashNode(current); err != nil {
            continue
        }

        // Potentially spread to connected nodes
        peers := fi.getNodePeers(current)
        for _, peer := range peers {
            if !affected[peer] && rand.Float64() < spreadProbability {
                affected[peer] = true
                queue = append(queue, peer)

                // Add delay for cascade effect
                time.Sleep(500 * time.Millisecond)
            }
        }
    }

    return nil
}

// ChaosMode enables random fault injection for chaos testing.
func (fi *FaultInjector) ChaosMode(
    ctx context.Context,
    intensity ChaosIntensity,
    duration time.Duration,
) {
    ticker := time.NewTicker(fi.getChaosInterval(intensity))
    defer ticker.Stop()

    endTime := time.Now().Add(duration)

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if time.Now().After(endTime) {
                return
            }
            fi.injectRandomFault(intensity)
        }
    }
}

// injectRandomFault injects a random fault based on intensity.
func (fi *FaultInjector) injectRandomFault(intensity ChaosIntensity) {
    faultTypes := fi.getFaultTypesForIntensity(intensity)
    faultType := faultTypes[rand.Intn(len(faultTypes))]

    switch faultType {
    case FaultLatency:
        latency := rand.Intn(500) + 100 // 100-600ms
        fi.InjectLatency(context.Background(),
            FaultTarget{Percentage: 0.3},
            latency, latency/10,
            30*time.Second)

    case FaultPacketLoss:
        loss := rand.Float64() * 20 // 0-20% loss
        fi.InjectPacketLoss(context.Background(),
            FaultTarget{Percentage: 0.2},
            loss,
            20*time.Second)

    case FaultNodeCrash:
        nodes := fi.orchestrator.GetNetworkStatus().NodeStatuses
        if len(nodes) > 2 {
            // Crash a random non-critical node
            fi.crashRandomNode()
        }
    }
}
```

### 3. Fork Testing Framework
```go
// internal/orchestration/forks.go
package orchestration

import (
    "context"
    "fmt"
    "time"

    "github.com/ethereum/go-ethereum/core"
    "github.com/ethereum/go-ethereum/params"
)

// ForkManager handles testing of protocol upgrades and forks.
type ForkManager struct {
    orchestrator *node.Orchestrator
    forkHistory  []*ForkEvent
}

// ForkEvent represents a fork in the chain.
type ForkEvent struct {
    Name        string
    BlockNumber uint64
    Timestamp   time.Time
    Changes     []ProtocolChange
    TestCases   []ForkTestCase
}

// ProtocolChange describes a change in protocol rules.
type ProtocolChange struct {
    Type        string
    Description string
    Parameters  map[string]interface{}
}

// ForkTestCase defines a test to run at a fork.
type ForkTestCase struct {
    Name     string
    PreFork  func() error
    AtFork   func() error
    PostFork func() error
}

// ScheduleFork schedules a protocol fork at a specific block.
func (fm *ForkManager) ScheduleFork(
    name string,
    atBlock uint64,
    changes []ProtocolChange,
) error {
    fork := &ForkEvent{
        Name:        name,
        BlockNumber: atBlock,
        Changes:     changes,
        TestCases:   make([]ForkTestCase, 0),
    }

    // Apply fork configuration to all nodes
    for _, pair := range fm.orchestrator.GetPairs() {
        if err := fm.configureForkForNode(pair, fork); err != nil {
            return fmt.Errorf("configure fork for %s: %w", pair.ID, err)
        }
    }

    fm.forkHistory = append(fm.forkHistory, fork)

    // Monitor for fork activation
    go fm.monitorForkActivation(fork)

    return nil
}

// configureForkForNode applies fork configuration to a node.
func (fm *ForkManager) configureForkForNode(pair *node.Pair, fork *ForkEvent) error {
    // Modify chain configuration
    chainConfig := params.ChainConfig{
        ChainID: big.NewInt(1337),
        // Apply fork-specific changes
    }

    for _, change := range fork.Changes {
        switch change.Type {
        case "eip1559":
            chainConfig.LondonBlock = big.NewInt(int64(fork.BlockNumber))
        case "merge":
            chainConfig.MergeBlock = big.NewInt(int64(fork.BlockNumber))
        case "shanghai":
            chainConfig.ShanghaiTime = &fork.Timestamp
        case "cancun":
            chainConfig.CancunTime = &fork.Timestamp
        }
    }

    // Apply to node (would require node restart in practice)
    return nil
}

// TestForkTransition tests behavior across a fork boundary.
func (fm *ForkManager) TestForkTransition(
    ctx context.Context,
    forkName string,
    testCase ForkTestCase,
) error {
    fork := fm.getFork(forkName)
    if fork == nil {
        return fmt.Errorf("fork %s not found", forkName)
    }

    // Run pre-fork test
    if testCase.PreFork != nil {
        currentBlock := fm.getCurrentBlock()
        if currentBlock < fork.BlockNumber-10 {
            if err := testCase.PreFork(); err != nil {
                return fmt.Errorf("pre-fork test failed: %w", err)
            }
        }
    }

    // Wait for fork block
    if err := fm.waitForBlock(ctx, fork.BlockNumber); err != nil {
        return fmt.Errorf("wait for fork block: %w", err)
    }

    // Run at-fork test
    if testCase.AtFork != nil {
        if err := testCase.AtFork(); err != nil {
            return fmt.Errorf("at-fork test failed: %w", err)
        }
    }

    // Wait a few blocks post-fork
    if err := fm.waitForBlock(ctx, fork.BlockNumber+5); err != nil {
        return fmt.Errorf("wait for post-fork blocks: %w", err)
    }

    // Run post-fork test
    if testCase.PostFork != nil {
        if err := testCase.PostFork(); err != nil {
            return fmt.Errorf("post-fork test failed: %w", err)
        }
    }

    return nil
}

// SimulateContentions creates competing forks to test fork choice.
func (fm *ForkManager) SimulateContentions(
    ctx context.Context,
    forkPoint uint64,
    branchCount int,
) error {
    // Partition validators to create competing chains
    validators := fm.orchestrator.GetValidatorDistribution()
    groups := fm.splitValidatorsIntoGroups(validators, branchCount)

    // Create network partitions
    partitions := make([][]string, branchCount)
    for i, group := range groups {
        partitions[i] = group.Nodes
    }

    // Wait for fork point
    if err := fm.waitForBlock(ctx, forkPoint); err != nil {
        return err
    }

    // Create partition
    faultInjector := NewFaultInjector(fm.orchestrator)
    if err := faultInjector.CreateNetworkPartition(ctx, partitions, 30*time.Second); err != nil {
        return fmt.Errorf("create partition: %w", err)
    }

    // Let chains diverge
    time.Sleep(20 * time.Second)

    // Heal partition and observe fork choice
    // The chain with more validators should win

    return nil
}
```

### 4. Performance Profiling
```go
// internal/orchestration/profiling.go
package orchestration

import (
    "context"
    "fmt"
    "runtime"
    "runtime/pprof"
    "time"
)

// Profiler provides performance profiling for the network.
type Profiler struct {
    orchestrator *node.Orchestrator
    metrics      *MetricsCollector
    traces       []*PerformanceTrace
}

// PerformanceTrace captures performance data over time.
type PerformanceTrace struct {
    ID         string
    StartTime  time.Time
    EndTime    time.Time
    Metrics    []MetricSnapshot
    Bottleneck *BottleneckAnalysis
}

// MetricSnapshot captures metrics at a point in time.
type MetricSnapshot struct {
    Timestamp       time.Time
    BlockHeight     uint64
    TPS             float64
    BlockTime       time.Duration
    GasUsed         uint64
    StateSize       uint64
    PeerCount       int
    MemoryUsage     uint64
    CPUUsage        float64
    DiskIO          IOStats
    NetworkIO       IOStats
    ValidatorMetrics map[string]*ValidatorPerformance
}

// BottleneckAnalysis identifies performance bottlenecks.
type BottleneckAnalysis struct {
    Type        BottleneckType
    Severity    float64 // 0-1
    Location    string
    Description string
    Suggestions []string
}

// BottleneckType categorizes performance issues.
type BottleneckType string

const (
    BottleneckCPU         BottleneckType = "cpu"
    BottleneckMemory      BottleneckType = "memory"
    BottleneckDisk        BottleneckType = "disk"
    BottleneckNetwork     BottleneckType = "network"
    BottleneckConsensus   BottleneckType = "consensus"
    BottleneckStateGrowth BottleneckType = "state_growth"
)

// StartProfiling begins collecting performance metrics.
func (p *Profiler) StartProfiling(ctx context.Context, duration time.Duration) (*PerformanceTrace, error) {
    trace := &PerformanceTrace{
        ID:        generateTraceID(),
        StartTime: time.Now(),
        Metrics:   make([]MetricSnapshot, 0),
    }

    // Start CPU profiling
    cpuFile, err := os.Create(fmt.Sprintf("cpu_%s.prof", trace.ID))
    if err != nil {
        return nil, err
    }
    defer cpuFile.Close()

    if err := pprof.StartCPUProfile(cpuFile); err != nil {
        return nil, err
    }
    defer pprof.StopCPUProfile()

    // Collect metrics periodically
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    endTime := time.Now().Add(duration)

    for {
        select {
        case <-ctx.Done():
            return trace, ctx.Err()
        case <-ticker.C:
            if time.Now().After(endTime) {
                trace.EndTime = time.Now()
                trace.Bottleneck = p.analyzeBottlenecks(trace)
                return trace, nil
            }

            snapshot := p.collectSnapshot()
            trace.Metrics = append(trace.Metrics, snapshot)
        }
    }
}

// collectSnapshot gathers current performance metrics.
func (p *Profiler) collectSnapshot() MetricSnapshot {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    snapshot := MetricSnapshot{
        Timestamp:   time.Now(),
        BlockHeight: p.getCurrentBlockHeight(),
        TPS:         p.calculateTPS(),
        BlockTime:   p.getAverageBlockTime(),
        GasUsed:     p.getGasUsed(),
        StateSize:   p.getStateSize(),
        PeerCount:   p.getTotalPeerCount(),
        MemoryUsage: m.Alloc,
        CPUUsage:    p.getCPUUsage(),
        DiskIO:      p.getDiskIO(),
        NetworkIO:   p.getNetworkIO(),
    }

    // Collect per-validator metrics
    snapshot.ValidatorMetrics = p.getValidatorMetrics()

    return snapshot
}

// analyzeBottlenecks identifies performance bottlenecks.
func (p *Profiler) analyzeBottlenecks(trace *PerformanceTrace) *BottleneckAnalysis {
    if len(trace.Metrics) < 2 {
        return nil
    }

    // Analyze CPU usage
    avgCPU := p.calculateAverageCPU(trace.Metrics)
    if avgCPU > 80 {
        return &BottleneckAnalysis{
            Type:        BottleneckCPU,
            Severity:    avgCPU / 100,
            Location:    "System",
            Description: fmt.Sprintf("High CPU usage: %.1f%%", avgCPU),
            Suggestions: []string{
                "Optimize computation-heavy operations",
                "Enable parallel processing where possible",
                "Consider hardware upgrade",
            },
        }
    }

    // Analyze memory usage
    memGrowth := p.calculateMemoryGrowthRate(trace.Metrics)
    if memGrowth > 100*1024*1024 { // 100MB/s growth
        return &BottleneckAnalysis{
            Type:        BottleneckMemory,
            Severity:    0.8,
            Location:    "Memory Management",
            Description: "Rapid memory growth detected",
            Suggestions: []string{
                "Check for memory leaks",
                "Optimize data structures",
                "Implement caching strategies",
            },
        }
    }

    // Analyze TPS degradation
    tpsDegradation := p.calculateTPSDegradation(trace.Metrics)
    if tpsDegradation > 0.3 { // 30% degradation
        return &BottleneckAnalysis{
            Type:        BottleneckConsensus,
            Severity:    tpsDegradation,
            Location:    "Transaction Processing",
            Description: fmt.Sprintf("TPS degraded by %.1f%%", tpsDegradation*100),
            Suggestions: []string{
                "Optimize transaction validation",
                "Increase block gas limit",
                "Review consensus parameters",
            },
        }
    }

    return nil
}

// BenchmarkScenario runs a specific benchmark scenario.
func (p *Profiler) BenchmarkScenario(
    ctx context.Context,
    scenario BenchmarkScenario,
) (*BenchmarkResult, error) {
    result := &BenchmarkResult{
        Scenario:  scenario,
        StartTime: time.Now(),
    }

    // Prepare network for benchmark
    if err := p.prepareForBenchmark(scenario); err != nil {
        return nil, fmt.Errorf("prepare benchmark: %w", err)
    }

    // Run warm-up phase
    if scenario.WarmupDuration > 0 {
        p.runWarmup(ctx, scenario)
    }

    // Start profiling
    trace, err := p.StartProfiling(ctx, scenario.Duration)
    if err != nil {
        return nil, fmt.Errorf("profiling failed: %w", err)
    }

    // Generate load based on scenario
    loadGen := NewLoadGenerator(p.orchestrator)
    loadStats, err := loadGen.GenerateLoad(ctx, scenario.LoadProfile)
    if err != nil {
        return nil, fmt.Errorf("load generation failed: %w", err)
    }

    result.EndTime = time.Now()
    result.Trace = trace
    result.LoadStats = loadStats
    result.Summary = p.generateBenchmarkSummary(trace, loadStats)

    return result, nil
}
```

### 5. Time Manipulation
```go
// internal/orchestration/time.go
package orchestration

import (
    "context"
    "fmt"
    "time"
)

// TimeController manages time manipulation for testing.
type TimeController struct {
    orchestrator *node.Orchestrator
    offset       time.Duration
    speedFactor  float64
    frozen       bool
    frozenAt     time.Time
}

// NewTimeController creates a time controller.
func NewTimeController(orchestrator *node.Orchestrator) *TimeController {
    return &TimeController{
        orchestrator: orchestrator,
        speedFactor:  1.0,
    }
}

// FastForward advances time by the specified duration.
func (tc *TimeController) FastForward(duration time.Duration) error {
    tc.offset += duration

    // Update all nodes with new time offset
    for _, pair := range tc.orchestrator.GetPairs() {
        if err := tc.updateNodeTime(pair, tc.offset); err != nil {
            return fmt.Errorf("update node %s time: %w", pair.ID, err)
        }
    }

    // Trigger time-dependent events
    tc.triggerTimeEvents(duration)

    return nil
}

// SetSpeed changes the speed of time progression.
func (tc *TimeController) SetSpeed(factor float64) error {
    if factor <= 0 {
        return fmt.Errorf("speed factor must be positive")
    }

    tc.speedFactor = factor

    // Adjust slot duration on all nodes
    newSlotDuration := time.Duration(float64(12*time.Second) / factor)

    for _, pair := range tc.orchestrator.GetPairs() {
        if err := tc.updateSlotDuration(pair, newSlotDuration); err != nil {
            return fmt.Errorf("update slot duration: %w", err)
        }
    }

    return nil
}

// Freeze stops time progression.
func (tc *TimeController) Freeze() error {
    tc.frozen = true
    tc.frozenAt = time.Now()

    // Pause block production
    for _, pair := range tc.orchestrator.GetPairs() {
        if err := tc.pauseBlockProduction(pair); err != nil {
            return fmt.Errorf("pause block production: %w", err)
        }
    }

    return nil
}

// Unfreeze resumes time progression.
func (tc *TimeController) Unfreeze() error {
    if !tc.frozen {
        return nil
    }

    tc.frozen = false

    // Resume block production
    for _, pair := range tc.orchestrator.GetPairs() {
        if err := tc.resumeBlockProduction(pair); err != nil {
            return fmt.Errorf("resume block production: %w", err)
        }
    }

    return nil
}

// WarpToSlot jumps directly to a specific slot.
func (tc *TimeController) WarpToSlot(targetSlot uint64) error {
    currentSlot := tc.getCurrentSlot()
    if targetSlot <= currentSlot {
        return fmt.Errorf("can only warp forward")
    }

    slotDiff := targetSlot - currentSlot
    timeDiff := time.Duration(slotDiff) * 12 * time.Second

    return tc.FastForward(timeDiff)
}
```

## Testing Requirements

### 1. Test Validator Rotation
```go
// internal/orchestration/validators_test.go
func TestValidatorRotation(t *testing.T) {
    orchestrator, vm := setupValidatorTest(t)

    // Start network with validators
    cfg := NetworkConfig{
        NodeCount:             3,
        ValidatorDistribution: []int{4, 4, 4},
    }
    require.NoError(t, orchestrator.StartNetwork(context.Background(), cfg))

    // Get a validator from node 0
    validator := vm.getValidatorsOnNode("node-0")[0]

    // Rotate to node 1
    require.NoError(t, vm.RotateValidator(context.Background(), validator, "node-1"))

    // Verify validator moved
    require.NotContains(t, vm.getValidatorsOnNode("node-0"), validator)
    require.Contains(t, vm.getValidatorsOnNode("node-1"), validator)

    // Verify network still producing blocks
    time.Sleep(5 * time.Second)
    // ... verify block production ...
}
```

### 2. Test Fault Injection
```go
func TestFaultInjection(t *testing.T) {
    orchestrator, fi := setupFaultTest(t)

    t.Run("Latency", func(t *testing.T) {
        err := fi.InjectLatency(
            context.Background(),
            FaultTarget{Nodes: []string{"node-0", "node-1"}},
            200, 50, 10*time.Second,
        )
        require.NoError(t, err)

        // Verify latency applied
        latency := measureLatencyBetweenNodes("node-0", "node-1")
        require.InDelta(t, 200, latency, 50)
    })

    t.Run("NetworkPartition", func(t *testing.T) {
        groups := [][]string{
            {"node-0", "node-1"},
            {"node-2", "node-3"},
        }

        err := fi.CreateNetworkPartition(
            context.Background(),
            groups,
            20*time.Second,
        )
        require.NoError(t, err)

        // Verify partition
        require.False(t, canCommunicate("node-0", "node-2"))
        require.True(t, canCommunicate("node-0", "node-1"))

        // Wait for healing
        time.Sleep(21 * time.Second)
        require.True(t, canCommunicate("node-0", "node-2"))
    })
}
```

### 3. Test Fork Transitions
```go
func TestForkTransition(t *testing.T) {
    orchestrator, fm := setupForkTest(t)

    // Schedule a fork at block 100
    changes := []ProtocolChange{
        {Type: "shanghai", Description: "Enable withdrawals"},
    }
    require.NoError(t, fm.ScheduleFork("shanghai", 100, changes))

    // Define test case
    testCase := ForkTestCase{
        PreFork: func() error {
            // Verify pre-fork behavior
            return nil
        },
        AtFork: func() error {
            // Verify fork activation
            return nil
        },
        PostFork: func() error {
            // Verify post-fork behavior
            return nil
        },
    }

    // Run fork test
    require.NoError(t, fm.TestForkTransition(
        context.Background(),
        "shanghai",
        testCase,
    ))
}
```

### 4. Test Performance Profiling
```go
func TestPerformanceProfiling(t *testing.T) {
    orchestrator, profiler := setupProfilerTest(t)

    // Run profiling
    trace, err := profiler.StartProfiling(
        context.Background(),
        30*time.Second,
    )
    require.NoError(t, err)
    require.NotNil(t, trace)

    // Verify metrics collected
    require.Greater(t, len(trace.Metrics), 20)

    // Check for bottlenecks
    if trace.Bottleneck != nil {
        t.Logf("Bottleneck detected: %s", trace.Bottleneck.Description)
        require.NotEmpty(t, trace.Bottleneck.Suggestions)
    }

    // Verify TPS is reasonable
    avgTPS := calculateAverageTPS(trace.Metrics)
    require.Greater(t, avgTPS, 10.0)
}
```

### 5. Test Time Manipulation
```go
func TestTimeManipulation(t *testing.T) {
    orchestrator, tc := setupTimeTest(t)

    t.Run("FastForward", func(t *testing.T) {
        initialSlot := getCurrentSlot()

        // Fast forward 10 minutes
        require.NoError(t, tc.FastForward(10*time.Minute))

        newSlot := getCurrentSlot()
        expectedSlot := initialSlot + 50 // 10 minutes / 12 seconds
        require.Equal(t, expectedSlot, newSlot)
    })

    t.Run("SpeedChange", func(t *testing.T) {
        // Double the speed
        require.NoError(t, tc.SetSpeed(2.0))

        initialSlot := getCurrentSlot()
        time.Sleep(6 * time.Second) // Should be 2 slots at 2x speed

        newSlot := getCurrentSlot()
        require.Equal(t, initialSlot+2, newSlot)
    })

    t.Run("Freeze", func(t *testing.T) {
        initialSlot := getCurrentSlot()

        require.NoError(t, tc.Freeze())
        time.Sleep(13 * time.Second)

        // Slot shouldn't advance while frozen
        require.Equal(t, initialSlot, getCurrentSlot())

        require.NoError(t, tc.Unfreeze())
    })
}
```

## Technical Considerations

### Important Notes
- **Resource Intensive**: Advanced features require significant CPU/memory
- **Timing Sensitivity**: Time manipulation affects consensus
- **State Management**: Snapshots require substantial disk space
- **Network Isolation**: Fault injection requires network namespace support

### Gotchas
1. **Validator Rotation**: Must wait for epoch boundaries
2. **Time Manipulation**: Can break consensus if not coordinated
3. **Fault Recovery**: Some faults may require manual intervention
4. **Performance Impact**: Profiling itself affects performance
5. **Fork Coordination**: All nodes must agree on fork parameters

### Performance Considerations
- Use sampling for profiling to reduce overhead
- Limit chaos testing intensity in resource-constrained environments
- Cache snapshot data for faster restore
- Implement rate limiting for event streams
- Use buffered channels for high-frequency metrics

## Success Metrics
- Validator rotation completes within 1 epoch
- Fault injection has <5% performance overhead
- Fork transitions maintain consensus
- Profiling identifies bottlenecks with >90% accuracy
- Time manipulation maintains deterministic behavior
- All advanced features have >80% test coverage
- Documentation includes comprehensive usage examples