package consensus

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/thep2p/skipgraph-go/modules"
	"github.com/thep2p/skipgraph-go/modules/component"
)

// BaseClient provides common functionality for all CL client implementations.
//
// BaseClient embeds component.Manager to provide lifecycle management following
// the Component pattern from skipgraph-go. Specific CL client implementations
// should embed BaseClient and provide startup logic via WithStartupLogic.
type BaseClient struct {
	*component.Manager
	logger zerolog.Logger
	config Config

	mu        sync.RWMutex
	readyChan chan interface{}
}

// NewBaseClient creates a new base client instance.
//
// The logger will be configured with a component field identifying the client type.
// Derived types should provide startup and shutdown logic using component.Manager options.
func NewBaseClient(logger zerolog.Logger, cfg Config, opts ...component.Option) *BaseClient {
	logger = logger.With().Str("component", fmt.Sprintf("cl-%s", cfg.Client)).Logger()

	b := &BaseClient{
		logger:    logger,
		config:    cfg,
		readyChan: make(chan interface{}),
	}

	// Create component.Manager with provided options
	b.Manager = component.NewManager(logger, opts...)

	return b
}

// Ready returns a channel that is closed when the client is operational.
//
// This override allows BaseClient to control readiness separately from
// the component.Manager's readiness, enabling custom readiness logic.
func (b *BaseClient) Ready() <-chan interface{} {
	return b.readyChan
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

// SetReady closes the ready channel to signal readiness.
//
// This should be called by derived types when they determine the client
// has achieved readiness (e.g., after syncing completes).
// Can only be called once - subsequent calls are ignored.
func (b *BaseClient) SetReady() {
	b.mu.Lock()
	defer b.mu.Unlock()

	select {
	case <-b.readyChan:
		// Already ready, do nothing
	default:
		b.logger.Info().Msg("consensus client is ready")
		close(b.readyChan)
	}
}

// IsReady returns true if the client is ready.
//
// Checks whether the ready channel has been closed.
func (b *BaseClient) IsReady() bool {
	select {
	case <-b.readyChan:
		return true
	default:
		return false
	}
}

// MonitorReadiness starts a background goroutine that checks readiness
// periodically until the context is cancelled.
//
// Derived types should call this in their startup logic and provide
// a checkReadiness function that returns true when the client is ready.
func (b *BaseClient) MonitorReadiness(ctx modules.ThrowableContext, checkReadiness func() bool) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				b.logger.Debug().Msg("readiness monitor stopped: context cancelled")
				return
			case <-ticker.C:
				if checkReadiness() {
					b.SetReady()
					return
				}
			}
		}
	}()
}
