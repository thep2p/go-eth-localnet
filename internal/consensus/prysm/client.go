// Package prysm provides integration with the Prysm consensus layer client.
package prysm

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/consensus"

	"github.com/prysmaticlabs/prysm/v5/beacon-chain/db/filesystem"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/execution"
	beaconnode "github.com/prysmaticlabs/prysm/v5/beacon-chain/node"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/sync/genesis"
)

// Timeout constants for client operations.
const (
	// StartupTimeout is the maximum time to wait for the beacon node to start.
	StartupTimeout = 30 * time.Second
	// ShutdownTimeout is the maximum time to wait for graceful shutdown.
	ShutdownTimeout = 10 * time.Second
	// ReadyDoneTimeout is the timeout for Ready/Done channel operations in tests.
	ReadyDoneTimeout = 10 * time.Second
)

// Client manages the lifecycle of a Prysm beacon node.
//
// Client wraps a Prysm BeaconNode and provides Start/Stop methods with
// Ready/Done channels for lifecycle synchronization.
type Client struct {
	cfg    consensus.Config
	logger zerolog.Logger

	mu         sync.RWMutex
	beaconNode *beaconnode.BeaconNode
	ready      chan struct{}
	done       chan struct{}
	cancel     context.CancelFunc
}

// NewClient creates a new Prysm beacon node client.
//
// Args:
//   - cfg: consensus configuration for the beacon node
//   - logger: logger for client operations
//
// Returns a configured Client ready to be started.
func NewClient(cfg consensus.Config, logger zerolog.Logger) *Client {
	return &Client{
		cfg:    cfg,
		logger: logger.With().Str("component", "prysm-client").Logger(),
		ready:  make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start initializes and starts the Prysm beacon node.
//
// The method generates genesis state, builds CLI context, and starts the beacon node.
// It blocks until the node signals readiness or context is cancelled.
//
// Args:
//   - ctx: context for cancellation
//
// Returns error if startup fails. On error, both Ready() and Done() channels are
// closed to prevent callers from blocking indefinitely.
//
// All errors are CRITICAL and indicate the beacon node cannot start.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.beaconNode != nil {
		c.mu.Unlock()
		return fmt.Errorf("client already started")
	}
	c.mu.Unlock()

	// Track startup state for proper channel cleanup on error paths.
	// - closeReady: if true when defer runs, close ready channel
	// - goroutineStarted: if false when defer runs, close done channel
	goroutineStarted := false
	closeReady := true
	defer func() {
		if closeReady {
			close(c.ready)
		}
		if !goroutineStarted {
			close(c.done)
		}
	}()

	// Ensure data directory exists
	if err := os.MkdirAll(c.cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Generate genesis state
	c.logger.Info().Msg("generating genesis state")
	genesisState, err := GenerateGenesisState(c.cfg)
	if err != nil {
		return fmt.Errorf("generate genesis state: %w", err)
	}

	// Write genesis state to file
	genesisPath := filepath.Join(c.cfg.DataDir, "genesis.ssz")
	if err := os.WriteFile(genesisPath, genesisState, 0644); err != nil {
		return fmt.Errorf("write genesis state: %w", err)
	}
	c.logger.Info().Str("path", genesisPath).Msg("genesis state written")

	// Build CLI context
	cliCtx, err := buildCLIContext(c.cfg, genesisPath)
	if err != nil {
		return fmt.Errorf("build cli context: %w", err)
	}

	// Create cancellable context for beacon node
	nodeCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	cliCtx.Context = nodeCtx

	// Create genesis initializer to load genesis state into database
	genesisInit, err := genesis.NewFileInitializer(genesisPath)
	if err != nil {
		cancel()
		return fmt.Errorf("create genesis initializer: %w", err)
	}

	// Create beacon node with genesis state, blob storage and execution chain configuration
	c.logger.Info().Msg("creating beacon node")
	blobPath := filepath.Join(c.cfg.DataDir, "blobs")
	bn, err := beaconnode.New(cliCtx, cancel,
		withGenesisInitializer(genesisInit),
		beaconnode.WithBlobStorageOptions(filesystem.WithBasePath(blobPath)),
		beaconnode.WithExecutionChainOptions([]execution.Option{
			execution.WithHttpEndpointAndJWTSecret(c.cfg.EngineEndpoint, c.cfg.JWTSecret),
		}),
	)
	if err != nil {
		cancel()
		return fmt.Errorf("create beacon node: %w", err)
	}

	c.mu.Lock()
	c.beaconNode = bn
	c.mu.Unlock()

	// Start beacon node in background - goroutine takes ownership of done channel
	goroutineStarted = true
	go func() {
		c.logger.Info().Msg("starting beacon node")
		bn.Start()
		// When Start() returns, node has stopped
		c.logger.Info().Msg("beacon node stopped")
		close(c.done)
	}()

	// Wait for gRPC server to be ready
	c.logger.Info().Int("port", c.cfg.RPCPort).Msg("waiting for beacon node to be ready")
	if err := c.waitForReady(ctx); err != nil {
		c.Stop()
		return fmt.Errorf("wait for ready: %w", err)
	}

	closeReady = false // Prevent double-close by defer
	close(c.ready)
	c.logger.Info().Msg("prysm beacon node started")
	return nil
}

// Stop gracefully shuts down the beacon node.
//
// Blocks until shutdown completes or ShutdownTimeout is reached.
func (c *Client) Stop() {
	c.mu.RLock()
	bn := c.beaconNode
	cancel := c.cancel
	c.mu.RUnlock()

	if bn == nil {
		return
	}

	c.logger.Info().Msg("stopping beacon node")

	if cancel != nil {
		cancel()
	}

	// Wait for done or timeout
	select {
	case <-c.done:
		c.logger.Info().Msg("prysm beacon node stopped gracefully")
	case <-time.After(ShutdownTimeout):
		c.logger.Warn().Msg("prysm beacon node shutdown timed out, forcing close")
		bn.Close()
	}
}

// Ready returns a channel that closes when the beacon node is ready.
func (c *Client) Ready() <-chan struct{} {
	return c.ready
}

// Done returns a channel that closes when the beacon node has stopped.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// waitForReady polls until gRPC server is accepting connections.
func (c *Client) waitForReady(ctx context.Context) error {
	deadline := time.Now().Add(StartupTimeout)
	addr := fmt.Sprintf("127.0.0.1:%d", c.cfg.RPCPort)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Try to connect to gRPC port
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("beacon node not ready within %s", StartupTimeout)
}

// withGenesisInitializer returns a BeaconNode option that sets the genesis initializer.
// This ensures the genesis state is loaded into the database before the beacon node starts.
func withGenesisInitializer(init genesis.Initializer) beaconnode.Option {
	return func(node *beaconnode.BeaconNode) error {
		node.GenesisInitializer = init
		return nil
	}
}
