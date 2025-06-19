// launcher.go
package node

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/model"
)

// Launcher starts a Geth node, injecting StaticNodes from cfg.
type Launcher struct {
	logger zerolog.Logger
}

// NewLauncher returns a Launcher.
func NewLauncher(logger zerolog.Logger) *Launcher {
	return &Launcher{logger: logger.With().Str("component", "node-launcher").Logger()}
}

// Launch creates, configures, and starts a Geth node with static peers.
func (l *Launcher) Launch(cfg model.Config) (*model.Handle, error) {
	// ensure datadir
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir datadir: %w", err)
	}

	// build P2P config
	p2pCfg := p2p.Config{
		ListenAddr:  fmt.Sprintf(":%d", cfg.P2PPort),
		PrivateKey:  cfg.PrivateKey,
		NoDiscovery: true,
		StaticNodes: make([]*enode.Node, 0, len(cfg.StaticNodes)),
		MaxPeers:    len(cfg.StaticNodes) + 1,
	}
	for _, url := range cfg.StaticNodes {
		n, err := enode.Parse(enode.ValidSchemes, url)
		if err != nil {
			return nil, fmt.Errorf("invalid static node %q: %w", url, err)
		}
		p2pCfg.StaticNodes = append(p2pCfg.StaticNodes, n)
	}

	// node config
	nodeCfg := &node.Config{
		DataDir:           cfg.DataDir,
		Name:              fmt.Sprintf("node-%s", cfg.ID.String()),
		P2P:               p2pCfg,
		HTTPHost:          "127.0.0.1",
		HTTPPort:          cfg.RPCPort,
		HTTPModules:       []string{"eth", "net", "web3", "admin"},
		UseLightweightKDF: true,
	}

	stack, err := node.New(nodeCfg)
	if err != nil {
		return nil, fmt.Errorf("new node: %w", err)
	}
	if _, err := eth.New(stack, &ethconfig.Config{NetworkId: 1337}); err != nil {
		return nil, fmt.Errorf("attach eth: %w", err)
	}
	if err := stack.Start(); err != nil {
		return nil, fmt.Errorf("start node: %w", err)
	}

	l.logger.Info().Str("enode", stack.Server().NodeInfo().Enode).Str("id", cfg.ID.String()).Msg("Node started")
	return model.NewHandle(stack, stack.Server().NodeInfo().Enode, cfg), nil
}
