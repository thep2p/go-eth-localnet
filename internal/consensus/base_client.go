package consensus

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/thep2p/skipgraph-go/modules/component"
)

// BaseClient provides common functionality for all CL client implementations.
//
// BaseClient embeds component.Manager to provide lifecycle management following
// the Component pattern from skipgraph-go. Specific CL client implementations
// should embed BaseClient and provide their startup/shutdown logic.
//
// Example usage in a derived type:
//
//	type PrysmClient struct {
//	    *BaseClient
//	    process *exec.Cmd
//	}
//
//	func NewPrysmClient(logger zerolog.Logger, cfg Config) *PrysmClient {
//	    p := &PrysmClient{}
//	    p.BaseClient = NewBaseClient(logger, cfg,
//	        component.WithStartupLogic(p.startPrysm),
//	        component.WithShutdownLogic(p.stopPrysm),
//	    )
//	    return p
//	}
//
// The component.Manager handles all lifecycle coordination:
//   - Ready() signals when startup logic completes
//   - Done() signals when shutdown logic completes
//   - WithComponent() can add child components
type BaseClient struct {
	*component.Manager
	logger zerolog.Logger
	config Config
}

// NewBaseClient creates a new base client instance.
//
// The logger will be configured with a component field identifying the client type.
// Derived types should provide startup and shutdown logic using component.Manager options.
//
// The component.Manager handles:
//   - Lifecycle coordination (Start/Ready/Done)
//   - Executing startup logic in the correct order
//   - Managing child component dependencies
//   - Coordinating graceful shutdown
func NewBaseClient(logger zerolog.Logger, cfg Config, opts ...component.Option) *BaseClient {
	logger = logger.With().Str("component", fmt.Sprintf("cl-%s", cfg.Client)).Logger()

	b := &BaseClient{
		logger: logger,
		config: cfg,
	}

	// Create component.Manager with provided options
	// The manager will execute startup/shutdown logic and manage readiness
	b.Manager = component.NewManager(logger, opts...)

	return b
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
