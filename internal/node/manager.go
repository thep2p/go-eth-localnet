package node

// Package node manages multiple in-process Geth nodes for local testing.

import (
	"context"
	"fmt"
	"math/big"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	gethnode "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/model"
)

// localNetChainID represents the chain ID for a local Ethereum network (1337).
const localNetChainID = 1337

// Manager starts and stops multiple Geth nodes backed by simulated beacons.
// It exposes the running nodes and waits for shutdown.
type Manager struct {
	logger        zerolog.Logger
	baseDataDir   string
	launcher      *Launcher
	assignNewPort func() int
	chainID       *big.Int

	nodes    []*gethnode.Node
	configs  []model.Config
	shutdown chan struct{}
	cancel   context.CancelFunc
}

// NewNodeManager constructs a Manager that will launch multiple nodes.
func NewNodeManager(
	logger zerolog.Logger,
	launcher *Launcher,
	baseDataDir string,
	assignNewPort func() int) *Manager {
	return &Manager{
		logger:        logger.With().Str("component", "node-manager").Logger(),
		baseDataDir:   baseDataDir,
		launcher:      launcher,
		assignNewPort: assignNewPort,
		shutdown:      make(chan struct{}),
		chainID:       big.NewInt(localNetChainID),
		nodes:         make([]*gethnode.Node, 0),
		configs:       make([]model.Config, 0),
	}
}

// Start launches a single node and waits until its RPC endpoint is reachable.
func (m *Manager) Start(ctx context.Context, opts ...LaunchOption) error {
	return m.StartNode(ctx, true, nil, opts...)
}

// StartNode launches a node with the given configuration.
// mine indicates whether this node should mine blocks.
// staticNodes contains enode URLs of peers this node should connect to.
func (m *Manager) StartNode(ctx context.Context, mine bool, staticNodes []string, opts ...LaunchOption) error {
	if m.cancel == nil {
		ctx, m.cancel = context.WithCancel(ctx)
		go m.handleShutdown(ctx)
	}

	priv, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	nodeIndex := len(m.nodes)
	cfg := model.Config{
		ID:          enode.PubkeyToIDV4(&priv.PublicKey),
		DataDir:     filepath.Join(m.baseDataDir, fmt.Sprintf("node%d", nodeIndex)),
		P2PPort:     m.assignNewPort(),
		RPCPort:     m.assignNewPort(),
		PrivateKey:  priv,
		StaticNodes: staticNodes,
		Mine:        mine,
	}

	n, err := m.launcher.Launch(cfg, opts...)
	if err != nil {
		return fmt.Errorf("launch node %d: %w", nodeIndex, err)
	}

	m.nodes = append(m.nodes, n)
	m.configs = append(m.configs, cfg)

	rpcURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.RPCPort)
	deadline := time.Now().Add(StartupTimeout)
	for {
		if time.Now().After(deadline) {
			_ = n.Close()
			return fmt.Errorf("rpc %q never came up", rpcURL)
		}
		client, err := rpc.DialContext(ctx, rpcURL)
		if err == nil {
			client.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	m.logger.Info().Int("node_index", nodeIndex).Str("enode", n.Server().NodeInfo().Enode).Msg("Node started")
	return nil
}

func (m *Manager) handleShutdown(ctx context.Context) {
	<-ctx.Done()
	for i, n := range m.nodes {
		if err := n.Close(); err != nil {
			m.logger.Error().Err(err).Int("node_index", i).Msg("failed to close geth node")
		}
	}
	close(m.shutdown)
}

// GethNode returns the first running node instance or nil if no nodes are started.
func (m *Manager) GethNode() *gethnode.Node {
	if len(m.nodes) == 0 {
		return nil
	}
	return m.nodes[0]
}

// GethNodes returns all running node instances.
func (m *Manager) GethNodes() []*gethnode.Node {
	return m.nodes
}

// GetNode returns the node at the given index.
func (m *Manager) GetNode(index int) *gethnode.Node {
	if index < 0 || index >= len(m.nodes) {
		return nil
	}
	return m.nodes[index]
}

func (m *Manager) ChainID() *big.Int {
	return m.chainID
}

// RPCPort returns the RPC port the first node is using.
func (m *Manager) RPCPort() int {
	if len(m.configs) == 0 {
		return 0
	}
	return m.configs[0].RPCPort
}

// GetRPCPort returns the RPC port for the node at the given index.
func (m *Manager) GetRPCPort(index int) int {
	if index < 0 || index >= len(m.configs) {
		return 0
	}
	return m.configs[index].RPCPort
}

// NodeCount returns the number of running nodes.
func (m *Manager) NodeCount() int {
	return len(m.nodes)
}

// Done waits for the shutdown signal and ensures any cleanup is completed.
// It can be used in tests to ensure the node has stopped before proceeding.
func (m *Manager) Done() {
	<-m.shutdown
}
