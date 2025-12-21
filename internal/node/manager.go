package node

// Package node manages multiple in-process Geth nodes for local testing.

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
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

	mu              sync.RWMutex
	nodes           []*gethnode.Node
	configs         []model.Config
	shutdown        chan struct{}
	cancel          context.CancelFunc
	enableEngineAPI bool
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

// Start launches the specified number of nodes. The first node will mine blocks,
// and subsequent nodes will connect to the first node as peers.
func (m *Manager) Start(ctx context.Context, nodeCount int, opts ...LaunchOption) error {
	if nodeCount <= 0 {
		return fmt.Errorf("node count must be positive, got %d", nodeCount)
	}

	ctx, m.cancel = context.WithCancel(ctx)
	go m.handleShutdown(ctx)

	// Start the first node (miner)
	if err := m.startSingleNode(ctx, true, nil, opts...); err != nil {
		return fmt.Errorf("failed to start miner node: %w", err)
	}

	// Get the enode of the first node for peer connections
	var staticNodes []string
	if nodeCount > 1 {
		m.mu.RLock()
		staticNodes = []string{m.nodes[0].Server().NodeInfo().Enode}
		m.mu.RUnlock()
	}

	// Start additional nodes as peers
	for i := 1; i < nodeCount; i++ {
		if err := m.startSingleNode(ctx, false, staticNodes, opts...); err != nil {
			return fmt.Errorf("failed to start peer node %d: %w", i, err)
		}
	}

	m.logger.Info().Int("node_count", nodeCount).Msg("all nodes started successfully")
	return nil
}

// StartNode launches a single Geth node with the specified configuration.
//
// Parameters:
//   - ctx: Context for cancellation and timeout.
//   - mine: If true, the node will mine blocks.
//   - staticNodes: List of enode URLs for peers to connect to.
//   - opts: Optional launch options for node configuration.
//
// Use StartNode when you need fine-grained control to start individual nodes,
// rather than starting a group of nodes with Start. Unlike Start, which launches
// multiple nodes and sets up peer connections automatically, StartNode allows you
// to launch nodes one at a time with custom settings.
func (m *Manager) StartNode(ctx context.Context, mine bool, staticNodes []string, opts ...LaunchOption) error {
	if m.cancel == nil {
		ctx, m.cancel = context.WithCancel(ctx)
		go m.handleShutdown(ctx)
	}

	return m.startSingleNode(ctx, mine, staticNodes, opts...)
}

// startSingleNode is the internal method to start a single node
func (m *Manager) startSingleNode(ctx context.Context, mine bool, staticNodes []string, opts ...LaunchOption) error {
	priv, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	m.mu.RLock()
	nodeIndex := len(m.nodes)
	m.mu.RUnlock()
	cfg := model.Config{
		ID:          enode.PubkeyToIDV4(&priv.PublicKey),
		DataDir:     filepath.Join(m.baseDataDir, fmt.Sprintf("node%d", nodeIndex)),
		P2PPort:     m.assignNewPort(),
		RPCPort:     m.assignNewPort(),
		PrivateKey:  priv,
		StaticNodes: staticNodes,
		Mine:        mine,
	}

	// Generate JWT secret and configure Engine API if enabled
	if m.enableEngineAPI {
		jwtPath, err := GenerateJWTSecret(cfg.DataDir)
		if err != nil {
			return fmt.Errorf("generate jwt secret: %w", err)
		}
		cfg.JWTSecretPath = jwtPath
		cfg.EnableEngineAPI = true
		cfg.EnginePort = m.assignNewPort()
	}

	n, err := m.launcher.Launch(cfg, opts...)
	if err != nil {
		return fmt.Errorf("launch node %d: %w", nodeIndex, err)
	}

	m.mu.Lock()
	m.nodes = append(m.nodes, n)
	m.configs = append(m.configs, cfg)
	m.mu.Unlock()

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

	m.logger.Info().Int("node_index", nodeIndex).Str("enode", n.Server().NodeInfo().Enode).Msg("node started")
	return nil
}

func (m *Manager) handleShutdown(ctx context.Context) {
	<-ctx.Done()
	m.mu.RLock()
	nodes := make([]*gethnode.Node, len(m.nodes))
	copy(nodes, m.nodes)
	m.mu.RUnlock()

	for i, n := range nodes {
		if err := n.Close(); err != nil {
			m.logger.Error().Err(err).Int("node_index", i).Msg("failed to close geth node")
		}
	}
	close(m.shutdown)
}

// GethNode returns the first running node instance or nil if no nodes are started.
// For multi-node setups, use GetNode(index) or GethNodes() to access specific nodes.```
func (m *Manager) GethNode() *gethnode.Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.nodes) == 0 {
		return nil
	}
	return m.nodes[0]
}

// GethNodes returns all running node instances.
func (m *Manager) GethNodes() []*gethnode.Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nodes := make([]*gethnode.Node, len(m.nodes))
	copy(nodes, m.nodes)
	return nodes
}

// GetNode returns the node at the given index.
func (m *Manager) GetNode(index int) *gethnode.Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
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
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.configs) == 0 {
		return 0
	}
	return m.configs[0].RPCPort
}

// GetRPCPort returns the RPC port for the node at the given index.
func (m *Manager) GetRPCPort(index int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if index < 0 || index >= len(m.configs) {
		return 0
	}
	return m.configs[index].RPCPort
}

// NodeCount returns the number of running nodes.
func (m *Manager) NodeCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.nodes)
}

// Done waits for the shutdown signal and ensures any cleanup is completed.
// It can be used in tests to ensure the node has stopped before proceeding.
func (m *Manager) Done() {
	<-m.shutdown
}

// EnableEngineAPI enables the Engine API for all nodes managed by this Manager.
// This must be called before starting any nodes. Returns an error if nodes
// have already been started.
func (m *Manager) EnableEngineAPI() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.nodes) > 0 {
		return fmt.Errorf("engine api must be enabled before starting nodes")
	}
	m.enableEngineAPI = true
	return nil
}

// GetEnginePort returns the Engine API port for the node at the given index.
// Returns 0 if the index is invalid or Engine API is not enabled.
func (m *Manager) GetEnginePort(index int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if index < 0 || index >= len(m.configs) {
		return 0
	}
	return m.configs[index].EnginePort
}

// GetJWTSecret returns the JWT secret for the node at the given index.
// Returns an error if the index is invalid or the JWT file cannot be read.
func (m *Manager) GetJWTSecret(index int) ([]byte, error) {
	m.mu.RLock()
	if index < 0 || index >= len(m.configs) {
		numConfigs := len(m.configs)
		m.mu.RUnlock()
		return nil, fmt.Errorf("engine api: node index %d out of range [0, %d)", index, numConfigs)
	}
	jwtPath := m.configs[index].JWTSecretPath
	// Release lock before file I/O to avoid blocking other operations.
	// JWT files are immutable once created, so the path remains valid.
	m.mu.RUnlock()

	if jwtPath == "" {
		return nil, fmt.Errorf("engine api: jwt not configured for node %d", index)
	}

	return os.ReadFile(jwtPath)
}
