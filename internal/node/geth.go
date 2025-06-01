package node

import (
	"fmt"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/rs/zerolog"
	"os"
)

type Launcher struct {
	logger zerolog.Logger
}

func NewLauncher(logger zerolog.Logger) *Launcher {
	return &Launcher{logger: logger.With().Str("component", "node-launcher").Logger()}
}

func (l *Launcher) Launch(cfg Config) (*Handle, error) {
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

	return &Handle{
		Instance: stack,
		EnodeURL: stack.Server().NodeInfo().Enode,
		Config:   cfg,
	}, nil
}
