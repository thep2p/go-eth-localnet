// Package node provides functionality for managing Ethereum nodes in a local network environment.
// It includes tools for launching, configuring, and orchestrating multiple Geth nodes,
// handling their lifecycle, managing peer connections, and coordinating node communication.
// The package supports creating full-mesh network topologies and manages node-specific
// configurations such as data directories, P2P ports, and RPC endpoints.
package node

import (
	"fmt"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"os"
)

// Launcher provides methods to initialize and manage Geth node instances. It uses a logger for operational logging.
type Launcher struct {
	logger zerolog.Logger
}

// NewLauncher constructs and returns a new Launcher instance with the provided logger.
func NewLauncher(logger zerolog.Logger) *Launcher {
	return &Launcher{logger: logger.With().Str("component", "node-launcher").Logger()}
}

// Launch initializes and starts a new Geth node based on the given configuration.
func (l *Launcher) Launch(cfg model.Config) (*model.Handle, error) {
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	stack, err := node.New(&node.Config{
		DataDir: cfg.DataDir,
		Name:    fmt.Sprintf("node-%d", cfg.ID),
		P2P: p2p.Config{
			ListenAddr: fmt.Sprintf(":%d", cfg.P2PPort),
		},
		HTTPHost:          "127.0.0.1",
		HTTPPort:          cfg.RPCPort,
		HTTPModules:       []string{"eth", "net", "web3"},
		UseLightweightKDF: true,
	})
	if err != nil {
		return nil, fmt.Errorf("new node: %w", err)
	}

	_, err = eth.New(stack, &ethconfig.Config{NetworkId: 1337})
	if err != nil {
		return nil, fmt.Errorf("new eth: %w", err)
	}

	if err := stack.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	l.logger.Info().Str("enode", stack.Server().NodeInfo().Enode).Msgf("Node %d started", cfg.ID)

	return model.NewHandle(stack, stack.Server().NodeInfo().Enode, cfg), nil
}
