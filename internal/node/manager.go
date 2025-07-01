package node

// Package node manages a single in-process Geth node for local testing.

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

// Manager starts and stops a single Geth node backed by a simulated beacon.
// It exposes the running node and waits for shutdown.
type Manager struct {
	logger        zerolog.Logger
	baseDataDir   string
	launcher      *Launcher
	assignNewPort func() int
	chainID       *big.Int

	gethNode *gethnode.Node
	cfg      model.Config
	shutdown chan struct{}
	cancel   context.CancelFunc
}

// NewNodeManager constructs a Manager that will launch one node.
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
	}
}

// Start launches the single node and waits until its RPC endpoint is reachable.
func (m *Manager) Start(ctx context.Context, opts ...LaunchOption) error {
	ctx, m.cancel = context.WithCancel(ctx)

	priv, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	cfg := model.Config{
		ID:         enode.PubkeyToIDV4(&priv.PublicKey),
		DataDir:    filepath.Join(m.baseDataDir, "node0"),
		P2PPort:    m.assignNewPort(),
		RPCPort:    m.assignNewPort(),
		PrivateKey: priv,
		Mine:       true,
	}

	n, err := m.launcher.Launch(cfg, opts...)
	if err != nil {
		return fmt.Errorf("launch node: %w", err)
	}
	m.gethNode = n
	m.cfg = cfg

	rpcURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.RPCPort)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			_ = n.Close()
			close(m.shutdown)
			return fmt.Errorf("rpc %q never came up", rpcURL)
		}
		client, err := rpc.DialContext(ctx, rpcURL)
		if err == nil {
			client.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	go func() {
		<-ctx.Done()
		if err := n.Close(); err != nil {
			m.logger.Fatal().Err(err).Msg("failed to close geth node")
		}
		close(m.shutdown)
	}()

	return nil
}

// GethNode returns the running node instance or nil if the node is not started.
func (m *Manager) GethNode() *gethnode.Node { return m.gethNode }

func (m *Manager) ChainID() *big.Int {
	return m.chainID
}

// RPCPort returns the RPC port the node is using.
func (m *Manager) RPCPort() int { return m.cfg.RPCPort }

// Done waits for the shutdown signal and ensures any cleanup is completed.
// It can be used in tests to ensure the node has stopped before proceeding.
func (m *Manager) Done() {
	<-m.shutdown
}
