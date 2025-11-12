package prysm

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
)

// Launcher creates and configures Prysm consensus clients.
//
// Launcher provides a factory for creating Prysm clients with
// proper configuration. It follows the same pattern as the Geth
// node launcher, providing a consistent interface for node creation.
type Launcher struct {
	logger zerolog.Logger
}

// NewLauncher creates a new Prysm launcher with the given logger.
//
// The launcher is stateless and can be reused to create multiple
// Prysm clients. The logger will be enhanced with component-specific
// fields for each created client.
func NewLauncher(logger zerolog.Logger) *Launcher {
	return &Launcher{
		logger: logger.With().Str("component", "prysm-launcher").Logger(),
	}
}

// Launch creates and configures a new Prysm client.
//
// Launch performs validation on the configuration and returns a ready-to-start
// Prysm client. The client is not started automatically; call Start on the
// returned client to launch the beacon node and validator.
//
// Configuration requirements:
// - DataDir must be a valid directory path
// - BeaconPort, P2PPort, and RPCPort must be unique and available
// - EngineEndpoint must point to a valid Geth Engine API endpoint
// - JWTSecret must match the secret used by the Geth node
// - ValidatorKeys are optional but required for block production
//
// Returns an error if the configuration is invalid.
func (l *Launcher) Launch(cfg consensus.Config) (*Client, error) {
	// Validate configuration early
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}
	if cfg.BeaconPort == 0 {
		return nil, fmt.Errorf("beacon port is required")
	}
	if cfg.P2PPort == 0 {
		return nil, fmt.Errorf("p2p port is required")
	}
	if cfg.EngineEndpoint == "" {
		return nil, fmt.Errorf("engine endpoint is required")
	}
	if len(cfg.JWTSecret) == 0 {
		return nil, fmt.Errorf("jwt secret is required")
	}

	l.logger.Info().
		Str("data_dir", cfg.DataDir).
		Int("beacon_port", cfg.BeaconPort).
		Int("p2p_port", cfg.P2PPort).
		Str("engine_endpoint", cfg.EngineEndpoint).
		Int("validator_count", len(cfg.ValidatorKeys)).
		Msg("Creating Prysm client")

	return NewClient(l.logger, cfg), nil
}

// LaunchOption is a functional option for customizing Prysm client configuration.
//
// LaunchOption follows the same pattern as node.LaunchOption, allowing
// configuration to be applied during launch. This pattern provides flexibility
// for test-specific or environment-specific configuration.
type LaunchOption func(*consensus.Config)

// WithValidatorKeys configures the client with validator keys for block production.
//
// Each key should be a hex-encoded BLS12-381 private key. In production,
// keys should be managed securely via remote signers; this option is
// primarily for testing and local development.
func WithValidatorKeys(keys []string) LaunchOption {
	return func(cfg *consensus.Config) {
		cfg.ValidatorKeys = keys
	}
}

// WithBootnodes configures the client with bootstrap nodes for peer discovery.
//
// Bootnodes are ENR addresses of well-known nodes that help new nodes
// discover peers in the P2P network.
func WithBootnodes(bootnodes []string) LaunchOption {
	return func(cfg *consensus.Config) {
		cfg.Bootnodes = bootnodes
	}
}

// WithStaticPeers configures the client with static peer connections.
//
// Static peers are persistent connections that should always be maintained,
// useful for connecting to specific known nodes in a private network.
func WithStaticPeers(peers []string) LaunchOption {
	return func(cfg *consensus.Config) {
		cfg.StaticPeers = peers
	}
}

// WithCheckpointSync enables checkpoint sync from a trusted source.
//
// Checkpoint sync allows rapid syncing from a recent finalized checkpoint
// rather than syncing from genesis. This is recommended for mainnet but
// typically not needed for local development networks.
func WithCheckpointSync(checkpointURL string) LaunchOption {
	return func(cfg *consensus.Config) {
		cfg.CheckpointSyncURL = checkpointURL
	}
}

// WithGenesisState configures a custom genesis state URL.
//
// This allows bootstrapping from a pre-generated genesis state
// rather than generating one from genesis configuration.
func WithGenesisState(genesisURL string) LaunchOption {
	return func(cfg *consensus.Config) {
		cfg.GenesisStateURL = genesisURL
	}
}

// LaunchWithOptions creates a Prysm client with additional configuration options.
//
// This is a convenience method that combines Launch with option application.
// It applies all options to the configuration before creating the client.
func (l *Launcher) LaunchWithOptions(cfg consensus.Config, opts ...LaunchOption) (*Client, error) {
	// Apply all options
	for _, opt := range opts {
		opt(&cfg)
	}

	return l.Launch(cfg)
}
