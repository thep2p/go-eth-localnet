package consensus

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/thep2p/skipgraph-go/modules"
	"github.com/thep2p/skipgraph-go/modules/component"
)

// MockClient is a test implementation of the CL Client interface.
//
// MockClient simulates CL client behavior for testing without requiring
// actual CL client binaries or network connectivity. It's useful for
// testing orchestration logic and interface contracts.
type MockClient struct {
	*BaseClient
	mockMetrics *Metrics
	mu          sync.Mutex
}

// NewMockClient creates a new mock CL client for testing.
//
// The mock client simulates a functional CL client with configurable
// behavior through the provided configuration. It becomes ready after
// a short delay to simulate initialization time.
func NewMockClient(logger zerolog.Logger, cfg Config) *MockClient {
	// Configure logger first
	compLogger := logger.With().Str("component", fmt.Sprintf("cl-%s", cfg.Client)).Logger()

	m := &MockClient{
		mockMetrics: &Metrics{
			CurrentSlot:    1,
			HeadSlot:       1,
			FinalizedSlot:  0,
			PeerCount:      5,
			IsSyncing:      false,
			ValidatorCount: len(cfg.ValidatorKeys),
		},
	}

	// Configure startup logic to simulate readiness after delay
	startupLogic := func(ctx modules.ThrowableContext) {
		compLogger.Info().Msg("starting mock consensus client")

		// Start readiness monitoring that succeeds after 100ms
		readyTime := time.Now().Add(100 * time.Millisecond)
		m.MonitorReadiness(ctx, func() bool {
			return time.Now().After(readyTime)
		})
	}

	shutdownLogic := func() {
		compLogger.Info().Msg("shutting down mock consensus client")
	}

	m.BaseClient = NewBaseClient(logger, cfg,
		component.WithStartupLogic(startupLogic),
		component.WithShutdownLogic(shutdownLogic),
	)

	return m
}

// ValidatorKeys returns mock validator keys.
//
// Returns the validator keys configured in the client configuration.
func (m *MockClient) ValidatorKeys() []string {
	return m.config.ValidatorKeys
}

// Metrics returns mock metrics.
//
// The metrics simulate a healthy, operational CL client with
// reasonable default values.
func (m *MockClient) Metrics() (*Metrics, error) {
	if !m.IsReady() {
		return nil, fmt.Errorf("client not ready")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update metrics to simulate progression
	m.mockMetrics.CurrentSlot++
	m.mockMetrics.HeadSlot = m.mockMetrics.CurrentSlot

	return m.mockMetrics, nil
}

// MockLauncher creates mock CL clients.
//
// MockLauncher implements the Launcher interface for creating
// mock clients in tests.
type MockLauncher struct {
	logger zerolog.Logger
}

// NewMockLauncher creates a new mock launcher.
func NewMockLauncher(logger zerolog.Logger) *MockLauncher {
	return &MockLauncher{logger: logger}
}

// Launch creates a new mock CL client.
//
// The mock client will use the provided configuration but doesn't
// actually start any external processes.
func (l *MockLauncher) Launch(cfg Config) (Client, error) {
	if err := l.ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return NewMockClient(l.logger, cfg), nil
}

// Name returns the launcher name.
func (l *MockLauncher) Name() string {
	return "mock"
}

// ValidateConfig validates the configuration.
//
// Returns an error if the configuration is missing required fields
// for the mock launcher.
func (l *MockLauncher) ValidateConfig(cfg Config) error {
	if cfg.BeaconPort == 0 {
		return fmt.Errorf("beacon port required")
	}
	if cfg.DataDir == "" {
		return fmt.Errorf("data directory required")
	}
	return nil
}
