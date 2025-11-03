package consensus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// BaseClient provides common functionality for all CL client implementations.
//
// BaseClient handles lifecycle management, readiness monitoring, and state
// tracking. Specific CL client implementations should embed BaseClient and
// override methods as needed.
type BaseClient struct {
	logger zerolog.Logger
	config Config

	mu      sync.RWMutex
	running bool
	ready   bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewBaseClient creates a new base client instance.
//
// The logger will be configured with a component field identifying the client type.
func NewBaseClient(logger zerolog.Logger, cfg Config) *BaseClient {
	return &BaseClient{
		logger: logger.With().Str("component", fmt.Sprintf("cl-%s", cfg.Client)).Logger(),
		config: cfg,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start begins base client operations.
//
// This method should be called by derived types after performing their
// own initialization. It starts background monitoring goroutines.
func (b *BaseClient) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return fmt.Errorf("client already running")
	}

	b.running = true
	b.logger.Info().Msg("starting consensus client")

	go b.monitorReadiness(ctx)

	return nil
}

// Stop initiates graceful shutdown.
//
// After calling Stop, Wait should be called to ensure all resources
// are properly cleaned up.
func (b *BaseClient) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return nil
	}

	b.logger.Info().Msg("stopping consensus client")
	close(b.stopCh)
	b.running = false
	return nil
}

// Wait blocks until the client is stopped.
//
// This ensures all background goroutines have completed and resources
// have been released.
func (b *BaseClient) Wait() error {
	<-b.doneCh
	b.logger.Info().Msg("consensus client stopped")
	return nil
}

// Ready returns true when the client is operational.
//
// A client is ready when it has synced sufficiently to participate
// in consensus operations.
func (b *BaseClient) Ready() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ready
}

// BeaconEndpoint returns the Beacon API endpoint.
//
// The endpoint is constructed from the configured beacon port and
// can be used to query beacon chain state via the standard Beacon API.
func (b *BaseClient) BeaconEndpoint() string {
	return fmt.Sprintf("http://127.0.0.1:%d", b.config.BeaconPort)
}

// Config returns the client configuration.
//
// This allows derived types to access the configuration without
// needing to store it separately.
func (b *BaseClient) Config() Config {
	return b.config
}

// Logger returns the client logger.
//
// Derived types can use this to log messages with consistent formatting.
func (b *BaseClient) Logger() zerolog.Logger {
	return b.logger
}

// SetReady updates the ready state.
//
// This should be called by derived types when they determine the client
// has achieved readiness (e.g., after syncing completes).
func (b *BaseClient) SetReady(ready bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ready = ready
	if ready {
		b.logger.Info().Msg("consensus client is ready")
	}
}

// IsRunning returns true if the client is currently running.
func (b *BaseClient) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// monitorReadiness checks client readiness in a loop.
//
// This method runs in a background goroutine and checks readiness
// periodically. Derived types should override checkReadiness to
// implement client-specific readiness checks.
func (b *BaseClient) monitorReadiness(ctx context.Context) {
	defer close(b.doneCh)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.logger.Debug().Msg("readiness monitor stopped: context cancelled")
			return
		case <-b.stopCh:
			b.logger.Debug().Msg("readiness monitor stopped: stop signal received")
			return
		case <-ticker.C:
			// Check readiness (implement in derived types)
			b.checkReadiness()
		}
	}
}

// checkReadiness is overridden by specific implementations.
//
// The base implementation does nothing. Derived types should override
// this to perform client-specific readiness checks (e.g., querying
// the Beacon API for sync status).
func (b *BaseClient) checkReadiness() {
	// Override in specific implementations
}
