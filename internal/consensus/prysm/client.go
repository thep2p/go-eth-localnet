package prysm

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/skipgraph-go/modules"
	"github.com/thep2p/skipgraph-go/modules/component"
)

const (
	// StartupTimeout is the maximum time to wait for Prysm to start.
	StartupTimeout = 10 * time.Second
	// ShutdownTimeout is the maximum time to wait for Prysm to shut down.
	ShutdownTimeout = 10 * time.Second
	// ReadyTimeout is the maximum time to wait for Prysm to be ready.
	ReadyTimeout = 10 * time.Second
)

// Client manages a Prysm beacon node and validator client in-process.
//
// Client implements the Component lifecycle pattern from skipgraph-go/modules.
// It handles both beacon node and validator operations, connecting to a paired
// Geth execution layer node via the Engine API.
//
// The Client uses component.Manager for lifecycle management, which provides:
// - Automatic Ready/Done state tracking
// - Proper goroutine management
// - Clean error propagation via ThrowableContext
type Client struct {
	logger  zerolog.Logger
	config  consensus.Config
	manager *component.Manager

	// beaconNode is the in-process Prysm beacon chain node.
	// The beacon node implements the Ethereum consensus protocol,
	// processing attestations and blocks, and coordinating with
	// the execution layer via Engine API.
	//nolint:unused // Will be used when Prysm integration is implemented
	beaconNode interface{}

	// validatorClient is the in-process Prysm validator client.
	// The validator client manages validator keys and duties,
	// producing blocks and attestations when assigned.
	//nolint:unused // Will be used when Prysm integration is implemented
	validatorClient interface{}
}

// NewClient creates a new Prysm client with the given configuration.
//
// The client is not started automatically; call Start to launch the
// beacon node and validator client. The logger will be enhanced with
// component-specific fields for structured logging.
//
// The returned client uses the Component lifecycle pattern and must be started
// via Start(ctx) where ctx is a modules.ThrowableContext.
func NewClient(logger zerolog.Logger, cfg consensus.Config) *Client {
	componentLogger := logger.With().Str("component", "prysm-client").Logger()

	client := &Client{
		logger: componentLogger,
		config: cfg,
	}

	// Create component manager with startup logic
	client.manager = component.NewManager(
		componentLogger,
		component.WithStartupLogic(client.startup),
		component.WithShutdownLogic(client.shutdown),
	)

	return client
}

// Start launches the Prysm beacon node and validator client in-process.
//
// Start implements the modules.Startable interface. It performs the following
// initialization sequence via the component manager:
// 1. Validates configuration (ports, data directory, Engine API endpoint)
// 2. Initializes the beacon node with genesis configuration
// 3. Starts the beacon node and registers lifecycle hooks
// 4. Initializes and starts the validator client if keys are configured
// 5. Waits for the beacon API to become ready
//
// If any step fails, the error is propagated via ctx.ThrowIrrecoverable().
// Start must be called only once; calling it multiple times will panic.
func (c *Client) Start(ctx modules.ThrowableContext) {
	// Validate configuration before starting
	if err := c.validateConfig(); err != nil {
		c.logger.Error().Err(err).Msg("Invalid configuration")
		ctx.ThrowIrrecoverable(fmt.Errorf("invalid config: %w", err))
		return
	}

	c.logger.Info().Msg("Prysm client starting")
	c.manager.Start(ctx)
}

// Ready returns a channel that closes when the client is ready to serve requests.
//
// The client is considered ready when:
// - Beacon node is running and healthy
// - Beacon API is responding to requests
// - Validator client is initialized (if validators are configured)
//
// The channel remains closed once ready; it never reopens.
// This method implements the modules.ReadyDoneAware interface.
func (c *Client) Ready() <-chan interface{} {
	return c.manager.Ready()
}

// Done returns a channel that closes when the client has fully stopped.
//
// Done should be used to wait for cleanup to complete after stopping the client.
// It's safe to call Done multiple times or before calling Start.
// This method implements the modules.ReadyDoneAware interface.
func (c *Client) Done() <-chan interface{} {
	return c.manager.Done()
}

// validateConfig checks that all required configuration fields are set.
func (c *Client) validateConfig() error {
	if c.config.DataDir == "" {
		return fmt.Errorf("data directory is required")
	}
	if c.config.BeaconPort == 0 {
		return fmt.Errorf("beacon port is required")
	}
	if c.config.P2PPort == 0 {
		return fmt.Errorf("p2p port is required")
	}
	if c.config.EngineEndpoint == "" {
		return fmt.Errorf("engine endpoint is required")
	}
	if len(c.config.JWTSecret) == 0 {
		return fmt.Errorf("jwt secret is required")
	}
	return nil
}

// startup is the initialization logic executed by the component manager.
// It uses ThrowableContext for clean error propagation - if any step fails,
// the error is thrown and the application terminates gracefully.
func (c *Client) startup(ctx modules.ThrowableContext) {
	// Initialize beacon node
	if err := c.initBeaconNode(ctx); err != nil {
		c.logger.Error().Err(err).Msg("Failed to initialize beacon node")
		ctx.ThrowIrrecoverable(fmt.Errorf("beacon node initialization failed: %w", err))
		return
	}

	// Start beacon node
	if err := c.startBeaconNode(ctx); err != nil {
		c.logger.Error().Err(err).Msg("Failed to start beacon node")
		ctx.ThrowIrrecoverable(fmt.Errorf("beacon node startup failed: %w", err))
		return
	}

	// Initialize validator if keys are configured
	if len(c.config.ValidatorKeys) > 0 {
		if err := c.initValidator(ctx); err != nil {
			c.logger.Error().Err(err).Msg("Failed to initialize validator")
			ctx.ThrowIrrecoverable(fmt.Errorf("validator initialization failed: %w", err))
			return
		}
	}

	// Wait for beacon API to be ready
	if err := c.waitForBeaconAPI(ctx); err != nil {
		c.logger.Error().Err(err).Msg("Beacon API never became ready")
		ctx.ThrowIrrecoverable(fmt.Errorf("beacon API readiness check failed: %w", err))
		return
	}

	c.logger.Info().Msg("Prysm client ready")
}

// initBeaconNode initializes the Prysm beacon node with genesis configuration.
func (c *Client) initBeaconNode(ctx modules.ThrowableContext) error {
	c.logger.Info().Msg("Initializing Prysm beacon node")

	// TODO: Implement beacon node initialization using Prysm v5 API
	// This will involve:
	// 1. Creating beacon node configuration from consensus.Config
	// 2. Setting up genesis state
	// 3. Configuring execution layer connection (Engine API)
	// 4. Setting up P2P networking

	return fmt.Errorf("beacon node initialization not yet implemented")
}

// startBeaconNode starts the Prysm beacon node.
func (c *Client) startBeaconNode(ctx modules.ThrowableContext) error {
	c.logger.Info().Msg("Starting Prysm beacon node")

	// TODO: Implement beacon node startup using Prysm v5 API
	// This will call the beacon node's Start method

	return fmt.Errorf("beacon node startup not yet implemented")
}

// initValidator initializes the Prysm validator client with configured keys.
func (c *Client) initValidator(ctx modules.ThrowableContext) error {
	c.logger.Info().Int("key_count", len(c.config.ValidatorKeys)).Msg("Initializing Prysm validator")

	// TODO: Implement validator initialization using Prysm v5 API
	// This will involve:
	// 1. Creating validator client configuration
	// 2. Loading validator keys
	// 3. Setting up beacon node connection
	// 4. Configuring fee recipient

	return fmt.Errorf("validator initialization not yet implemented")
}

// waitForBeaconAPI waits for the beacon API to become responsive.
func (c *Client) waitForBeaconAPI(ctx modules.ThrowableContext) error {
	c.logger.Info().Msg("Waiting for beacon API to be ready")

	deadline := time.Now().Add(ReadyTimeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("beacon API did not become ready within %v", ReadyTimeout)
		}

		// TODO: Implement actual health check against beacon API
		// For now, just simulate readiness
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		case <-time.After(100 * time.Millisecond):
			// Check health endpoint
			continue
		}
	}
}

// shutdown performs graceful shutdown of beacon node and validator.
func (c *Client) shutdown() {
	c.logger.Info().Msg("Shutting down Prysm components")

	// TODO: Implement graceful shutdown
	// This will:
	// 1. Stop validator client
	// 2. Stop beacon node
	// 3. Close all connections
	// 4. Clean up resources
}

// BeaconAPIURL returns the URL for the Beacon API.
func (c *Client) BeaconAPIURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", c.config.BeaconPort)
}

// P2PAddress returns the P2P multiaddr for this node.
func (c *Client) P2PAddress() string {
	return fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", c.config.P2PPort)
}
