# Step 2a: Consensus Layer Client Abstraction

## Overview

Before we can launch specific CL clients like Prysm, we need a clean abstraction layer that defines how ANY consensus client integrates with our system. This issue creates the foundational interfaces and types that all CL client implementations will use, following the same pattern as our existing `Launcher` for Geth nodes.

This abstraction enables us to support multiple CL clients (Prysm, Lighthouse, Nimbus, etc.) without duplicating code and provides a consistent interface for the Manager to orchestrate EL+CL pairs.

## Why This Matters

- **Extensibility**: Easy to add new CL clients without changing core orchestration logic
- **Testability**: Can create mock CL clients for testing
- **Consistency**: All CL clients follow the same lifecycle and configuration patterns
- **Separation of Concerns**: Core orchestration logic doesn't need to know CL client specifics

## Acceptance Criteria

- [ ] `CLClient` interface defines standard CL client operations
- [ ] `CLConfig` struct contains all CL client configuration
- [ ] `CLLauncher` interface for creating CL client instances
- [ ] Base implementation of common CL client functionality
- [ ] Integration points with existing `node.Manager`
- [ ] Mock CL client for testing the abstraction
- [ ] Tests verify interface contracts

## Implementation Tasks

### 1. Define Core CL Client Interface
```go
// internal/consensus/client.go
package consensus

import (
    "context"
    "github.com/ethereum/go-ethereum/common"
)

// Client represents a Consensus Layer client instance.
type Client interface {
    // Start begins the CL client's operation.
    Start(ctx context.Context) error

    // Stop gracefully shuts down the CL client.
    Stop() error

    // Wait blocks until the client is fully stopped.
    Wait() error

    // Ready returns true when the client is synced and operational.
    Ready() bool

    // BeaconEndpoint returns the Beacon API endpoint URL.
    BeaconEndpoint() string

    // ValidatorKeys returns the validator public keys managed by this client.
    ValidatorKeys() []string

    // Metrics returns client metrics (slots, peers, etc.).
    Metrics() (*Metrics, error)
}

// Metrics contains CL client operational metrics.
type Metrics struct {
    CurrentSlot    uint64
    HeadSlot       uint64
    FinalizedSlot  uint64
    PeerCount      int
    IsSyncing      bool
    ValidatorCount int
}
```

### 2. CL Configuration Model
```go
// internal/consensus/config.go
package consensus

import (
    "crypto/ecdsa"
    "time"
)

// Config holds configuration for a Consensus Layer client.
type Config struct {
    // Client identifies which CL implementation to use (prysm, lighthouse, etc.).
    Client string

    // DataDir is the directory for CL client data.
    DataDir string

    // Network configuration
    ChainID     uint64
    GenesisTime time.Time
    GenesisRoot [32]byte

    // Ports
    BeaconPort int // Beacon API port
    P2PPort    int // P2P networking port
    RPCPort    int // gRPC or other RPC port

    // Connection to Execution Layer
    EngineEndpoint string // Engine API endpoint of paired EL
    JWTSecret      []byte // JWT secret for Engine API auth

    // P2P configuration
    Bootnodes   []string      // ENR addresses of boot nodes
    StaticPeers []string      // Static peer connections
    PrivateKey  *ecdsa.PrivateKey // Node identity key

    // Validator configuration
    ValidatorKeys []string // Validator private keys (for testing)
    FeeRecipient  common.Address

    // Optional: Checkpoint sync
    CheckpointSyncURL string
    GenesisStateURL   string
}
```

### 3. CL Launcher Interface
```go
// internal/consensus/launcher.go
package consensus

import (
    "github.com/rs/zerolog"
)

// Launcher creates and configures CL client instances.
type Launcher interface {
    // Launch creates and starts a new CL client with the given configuration.
    Launch(cfg Config) (Client, error)

    // Name returns the name of this launcher (e.g., "prysm", "lighthouse").
    Name() string

    // ValidateConfig checks if the configuration is valid for this launcher.
    ValidateConfig(cfg Config) error
}

// LaunchOption modifies CL client configuration before launch.
type LaunchOption func(*Config)

// WithValidatorKeys configures validator keys for block production.
func WithValidatorKeys(keys []string) LaunchOption {
    return func(cfg *Config) {
        cfg.ValidatorKeys = keys
    }
}

// WithCheckpointSync enables checkpoint sync from a trusted source.
func WithCheckpointSync(url string) LaunchOption {
    return func(cfg *Config) {
        cfg.CheckpointSyncURL = url
    }
}

// WithBootnodes configures bootstrap nodes for P2P discovery.
func WithBootnodes(bootnodes []string) LaunchOption {
    return func(cfg *Config) {
        cfg.Bootnodes = bootnodes
    }
}
```

### 4. Base CL Client Implementation
```go
// internal/consensus/base_client.go
package consensus

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/rs/zerolog"
)

// BaseClient provides common functionality for all CL client implementations.
type BaseClient struct {
    logger   zerolog.Logger
    config   Config

    mu       sync.RWMutex
    running  bool
    ready    bool
    stopCh   chan struct{}
    doneCh   chan struct{}
}

// NewBaseClient creates a new base client instance.
func NewBaseClient(logger zerolog.Logger, cfg Config) *BaseClient {
    return &BaseClient{
        logger: logger.With().Str("component", fmt.Sprintf("cl-%s", cfg.Client)).Logger(),
        config: cfg,
        stopCh: make(chan struct{}),
        doneCh: make(chan struct{}),
    }
}

// Start begins base client operations (implement in derived types).
func (b *BaseClient) Start(ctx context.Context) error {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.running {
        return fmt.Errorf("client already running")
    }

    b.running = true
    go b.monitorReadiness(ctx)

    return nil
}

// Stop initiates graceful shutdown.
func (b *BaseClient) Stop() error {
    b.mu.Lock()
    defer b.mu.Unlock()

    if !b.running {
        return nil
    }

    close(b.stopCh)
    b.running = false
    return nil
}

// Wait blocks until the client is stopped.
func (b *BaseClient) Wait() error {
    <-b.doneCh
    return nil
}

// Ready returns true when the client is operational.
func (b *BaseClient) Ready() bool {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return b.ready
}

// BeaconEndpoint returns the Beacon API endpoint.
func (b *BaseClient) BeaconEndpoint() string {
    return fmt.Sprintf("http://127.0.0.1:%d", b.config.BeaconPort)
}

// monitorReadiness checks if the client is ready.
func (b *BaseClient) monitorReadiness(ctx context.Context) {
    defer close(b.doneCh)

    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-b.stopCh:
            return
        case <-ticker.C:
            // Check readiness (implement in derived types)
            // This is a placeholder - real implementation would check beacon API
            b.checkReadiness()
        }
    }
}

// checkReadiness is overridden by specific implementations.
func (b *BaseClient) checkReadiness() {
    // Override in specific implementations
}
```

### 5. Mock CL Client for Testing
```go
// internal/consensus/mock_client.go
package consensus

import (
    "context"
    "time"
)

// MockClient is a test implementation of the CL Client interface.
type MockClient struct {
    *BaseClient
    mockMetrics *Metrics
}

// NewMockClient creates a new mock CL client for testing.
func NewMockClient(cfg Config) *MockClient {
    return &MockClient{
        BaseClient: NewBaseClient(zerolog.Nop(), cfg),
        mockMetrics: &Metrics{
            CurrentSlot:    1,
            HeadSlot:       1,
            FinalizedSlot:  0,
            PeerCount:      5,
            IsSyncing:      false,
            ValidatorCount: 1,
        },
    }
}

// Start simulates starting a CL client.
func (m *MockClient) Start(ctx context.Context) error {
    if err := m.BaseClient.Start(ctx); err != nil {
        return err
    }

    // Simulate readiness after 100ms
    go func() {
        time.Sleep(100 * time.Millisecond)
        m.mu.Lock()
        m.ready = true
        m.mu.Unlock()
    }()

    return nil
}

// ValidatorKeys returns mock validator keys.
func (m *MockClient) ValidatorKeys() []string {
    return m.config.ValidatorKeys
}

// Metrics returns mock metrics.
func (m *MockClient) Metrics() (*Metrics, error) {
    return m.mockMetrics, nil
}

// MockLauncher creates mock CL clients.
type MockLauncher struct {
    logger zerolog.Logger
}

// NewMockLauncher creates a new mock launcher.
func NewMockLauncher(logger zerolog.Logger) *MockLauncher {
    return &MockLauncher{logger: logger}
}

// Launch creates a new mock CL client.
func (l *MockLauncher) Launch(cfg Config) (Client, error) {
    return NewMockClient(cfg), nil
}

// Name returns the launcher name.
func (l *MockLauncher) Name() string {
    return "mock"
}

// ValidateConfig validates the configuration.
func (l *MockLauncher) ValidateConfig(cfg Config) error {
    if cfg.BeaconPort == 0 {
        return fmt.Errorf("beacon port required")
    }
    return nil
}
```

### 6. CL Registry for Multiple Implementations
```go
// internal/consensus/registry.go
package consensus

import (
    "fmt"
    "sync"
)

// Registry manages available CL client launchers.
type Registry struct {
    mu        sync.RWMutex
    launchers map[string]Launcher
}

// NewRegistry creates a new launcher registry.
func NewRegistry() *Registry {
    return &Registry{
        launchers: make(map[string]Launcher),
    }
}

// Register adds a launcher to the registry.
func (r *Registry) Register(name string, launcher Launcher) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.launchers[name]; exists {
        return fmt.Errorf("launcher %s already registered", name)
    }

    r.launchers[name] = launcher
    return nil
}

// Get returns a launcher by name.
func (r *Registry) Get(name string) (Launcher, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    launcher, exists := r.launchers[name]
    if !exists {
        return nil, fmt.Errorf("launcher %s not found", name)
    }

    return launcher, nil
}

// Available returns all registered launcher names.
func (r *Registry) Available() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()

    names := make([]string, 0, len(r.launchers))
    for name := range r.launchers {
        names = append(names, name)
    }
    return names
}

// DefaultRegistry is the global CL launcher registry.
var DefaultRegistry = NewRegistry()

func init() {
    // Register mock launcher by default for testing
    DefaultRegistry.Register("mock", NewMockLauncher(zerolog.Nop()))
}
```

## Testing Requirements

### 1. Test CL Client Interface Contract
```go
// internal/consensus/client_test.go
func TestClientLifecycle(t *testing.T) {
    cfg := Config{
        Client:     "mock",
        DataDir:    t.TempDir(),
        BeaconPort: 4000,
        P2PPort:    9000,
    }

    client := NewMockClient(cfg)
    ctx := context.Background()

    // Test Start
    require.NoError(t, client.Start(ctx))
    require.Eventually(t, client.Ready, 2*time.Second, 100*time.Millisecond)

    // Test Metrics
    metrics, err := client.Metrics()
    require.NoError(t, err)
    require.NotNil(t, metrics)
    require.Greater(t, metrics.CurrentSlot, uint64(0))

    // Test Stop
    require.NoError(t, client.Stop())
    require.NoError(t, client.Wait())
}
```

### 2. Test Registry
```go
func TestRegistry(t *testing.T) {
    registry := NewRegistry()

    // Register launcher
    launcher := NewMockLauncher(zerolog.Nop())
    require.NoError(t, registry.Register("test", launcher))

    // Get launcher
    got, err := registry.Get("test")
    require.NoError(t, err)
    require.Equal(t, launcher, got)

    // Duplicate registration should fail
    require.Error(t, registry.Register("test", launcher))

    // Unknown launcher should fail
    _, err = registry.Get("unknown")
    require.Error(t, err)
}
```

### 3. Test Launch Options
```go
func TestLaunchOptions(t *testing.T) {
    cfg := Config{
        Client: "mock",
    }

    // Apply options
    opts := []LaunchOption{
        WithValidatorKeys([]string{"key1", "key2"}),
        WithBootnodes([]string{"enr:node1", "enr:node2"}),
        WithCheckpointSync("http://checkpoint.example.com"),
    }

    for _, opt := range opts {
        opt(&cfg)
    }

    require.Equal(t, []string{"key1", "key2"}, cfg.ValidatorKeys)
    require.Equal(t, []string{"enr:node1", "enr:node2"}, cfg.Bootnodes)
    require.Equal(t, "http://checkpoint.example.com", cfg.CheckpointSyncURL)
}
```

## Technical Considerations

### Important Notes
- **Interface Stability**: This interface will be used by all CL implementations, so changes should be minimal
- **Context Propagation**: All operations should respect context cancellation
- **Error Handling**: Clear error messages for debugging multi-client setups
- **Resource Cleanup**: Proper cleanup in Stop/Wait methods

### Gotchas
1. **Readiness Detection**: Different CL clients have different readiness criteria
2. **Port Management**: Each CL client needs multiple ports (beacon, P2P, metrics)
3. **Genesis Configuration**: Must match between EL and CL exactly
4. **Time Synchronization**: CL clients are sensitive to clock skew

## Success Metrics
- Clean separation between abstraction and implementation
- Mock client passes all interface tests
- Easy to add new CL client implementations
- No changes needed to existing EL code
- >90% test coverage for abstraction layer