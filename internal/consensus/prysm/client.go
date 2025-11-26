package prysm

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/skipgraph-go/modules"
	"github.com/thep2p/skipgraph-go/modules/component"
)

const (
	// ReadyDoneTimeout is the maximum time to wait for Prysm to be ready.
	ReadyDoneTimeout = 10 * time.Second
)

// Client manages a Prysm beacon node and validator client in-process.
//
// Client implements the Component lifecycle pattern from skipgraph-go/modules.
// It handles both beacon node and validator operations, connecting to a paired
// Geth execution layer node via the Engine API.
type Client struct {
	*component.Manager

	logger zerolog.Logger
	config consensus.Config

	// beaconNode is the in-process Prysm beacon chain node.
	// The beacon node implements the Ethereum consensus protocol,
	// processing attestations and blocks, and coordinating with
	// the execution layer via Engine API.
	//nolint:unused // Will be used in #45: https://github.com/thep2p/go-eth-localnet/issues/45
	beaconNode interface{}

	// validatorClient is the in-process Prysm validator client.
	// The validator client manages validator keys and duties,
	// producing blocks and attestations when assigned.
	//nolint:unused // Will be used in #46: https://github.com/thep2p/go-eth-localnet/issues/46
	validatorClient interface{}
}

// NewClient creates a new Prysm client with the given configuration.
//
// The client is not started automatically; call Start to launch the
// beacon node and validator client.
//
// Returns an error if the configuration is invalid. Any return errors is irrecoverable and must crash the application.
func NewClient(logger zerolog.Logger, cfg consensus.Config) (*Client, error) {
	// Validate configuration
	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client := &Client{
		logger: logger.With().Str("component", "prysm-client").Logger(),
		config: cfg,
	}

	// Create component manager with startup logic
	client.Manager = component.NewManager(
		client.logger.With().Str("component", "component-manager").Logger(),
		component.WithStartupLogic(client.startup),
		component.WithShutdownLogic(client.shutdown),
	)

	return client, nil
}

// startup is the initialization logic executed by the component manager.
// It uses ThrowableContext for clean error propagation - if any step fails,
// the error is thrown and the application terminates gracefully.
func (c *Client) startup(ctx modules.ThrowableContext) {
	c.logger.Info().Msg("prysm client starting")

	// Initialize beacon node
	if err := c.initBeaconNode(ctx); err != nil {
		ctx.ThrowIrrecoverable(fmt.Errorf("beacon node initialization failed: %w", err))
		// basically, we crash the process at the line above, so the return is just for code clarity
		return
	}

	// Start beacon node
	if err := c.startBeaconNode(ctx); err != nil {
		ctx.ThrowIrrecoverable(fmt.Errorf("beacon node startup failed: %w", err))
		// basically, we crash the process at the line above, so the return is just for code clarity
		return
	}

	// Initialize validator if keys are configured
	if len(c.config.ValidatorKeys) > 0 {
		if err := c.initValidator(ctx); err != nil {
			ctx.ThrowIrrecoverable(fmt.Errorf("validator initialization failed: %w", err))
			return
		}
	}

	// Wait for beacon API to be ready
	if err := c.waitForBeaconAPI(ctx); err != nil {
		ctx.ThrowIrrecoverable(fmt.Errorf("beacon api readiness check failed: %w", err))
		return
	}

	c.logger.Info().Msg("prysm client ready")
}

// initBeaconNode initializes the Prysm beacon node with genesis configuration.
func (c *Client) initBeaconNode(ctx modules.ThrowableContext) error {
	c.logger.Info().Msg("initializing prysm beacon node")

	// TODO(#45): Implement beacon node initialization using Prysm v5 API
	// https://github.com/thep2p/go-eth-localnet/issues/45
	// This will involve:
	// 1. Creating beacon node configuration from consensus.Config
	// 2. Setting up genesis state
	// 3. Configuring execution layer connection (Engine API)
	// 4. Setting up P2P networking

	return nil
}

// startBeaconNode starts the Prysm beacon node.
func (c *Client) startBeaconNode(ctx modules.ThrowableContext) error {
	c.logger.Info().Msg("starting prysm beacon node")

	// TODO(#45): Implement beacon node startup using Prysm v5 API
	// https://github.com/thep2p/go-eth-localnet/issues/45
	// This will call the beacon node's Start method

	return nil
}

// initValidator initializes the Prysm validator client with configured keys.
func (c *Client) initValidator(ctx modules.ThrowableContext) error {
	c.logger.Info().Int("key_count", len(c.config.ValidatorKeys)).Msg("initializing prysm validator")

	// TODO(#46): Implement validator initialization using Prysm v5 API
	// https://github.com/thep2p/go-eth-localnet/issues/46
	// This will involve:
	// 1. Creating validator client configuration
	// 2. Loading validator keys
	// 3. Setting up beacon node connection
	// 4. Configuring fee recipient

	return nil
}

// waitForBeaconAPI waits for the beacon API to become responsive.
func (c *Client) waitForBeaconAPI(ctx modules.ThrowableContext) error {
	_ = ctx // will be used when health check is implemented
	c.logger.Info().Msg("waiting for beacon api to be ready")

	// TODO(#48): Implement actual health check against beacon API
	// https://github.com/thep2p/go-eth-localnet/issues/48

	return nil
}

// shutdown performs graceful shutdown of beacon node and validator.
func (c *Client) shutdown() {
	c.logger.Info().Msg("shutting down prysm components")

	// TODO(#45): Implement graceful shutdown
	// https://github.com/thep2p/go-eth-localnet/issues/45
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
