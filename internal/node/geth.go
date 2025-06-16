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
	"github.com/ethereum/go-ethereum/p2p/enode"
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

// Launch starts a Geth node, injecting StaticNodes from the config into p2p.Config.
func (l *Launcher) Launch(cfg model.Config) (*model.Handle, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir datadir: %w", err)
	}

	// Build P2P configuration with static peers
	p2pCfg := p2p.Config{
		ListenAddr:  fmt.Sprintf(":%d", cfg.P2PPort),
		PrivateKey:  cfg.PrivateKey,
		NoDiscovery: true,
		StaticNodes: make([]*enode.Node, 0, len(cfg.StaticNodes)),
	}
	for _, url := range cfg.StaticNodes {
		n, err := enode.Parse(enode.ValidSchemes, url)
		if err != nil {
			return nil, fmt.Errorf("invalid static node %q: %w", url, err)
		}
		p2pCfg.StaticNodes = append(p2pCfg.StaticNodes, n)
	}

	// Prepare node configuration
	nodeCfg := &node.Config{
		DataDir:           cfg.DataDir,
		Name:              fmt.Sprintf("node-%s", cfg.ID.String()),
		P2P:               p2pCfg,
		HTTPHost:          "127.0.0.1",
		HTTPPort:          cfg.RPCPort,
		HTTPModules:       []string{"eth", "net", "web3", "admin"},
		UseLightweightKDF: true,
	}

	// Create Ethereum node stack
	stack, err := node.New(nodeCfg)
	if err != nil {
		return nil, fmt.Errorf("new node: %w", err)
	}

	// Attach ETH service
	if _, err := eth.New(stack, &ethconfig.Config{NetworkId: 1337}); err != nil {
		return nil, fmt.Errorf("new eth service: %w", err)
	}

	// Start the node
	if err := stack.Start(); err != nil {
		return nil, fmt.Errorf("start node: %w", err)
	}

	// Log the enode URL
	l.logger.Info().Str("enode", stack.Server().NodeInfo().Enode).Str("id", cfg.ID.String()).Msg("Node started")

	// Return a handle for further management
	return model.NewHandle(stack, stack.Server().NodeInfo().Enode, cfg), nil
}
