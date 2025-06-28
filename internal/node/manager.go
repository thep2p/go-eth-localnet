package node

// Package node manages a single in-process Geth node for local testing.

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	gethnode "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/model"
)

// Manager starts and stops a single Geth node backed by a simulated beacon.
// It exposes the running node and waits for shutdown.
type Manager struct {
	logger       zerolog.Logger
	baseDataDir  string
	launcher     *Launcher
	portAssigner func() int

	gethNode *gethnode.Node
	cfg      model.Config
	shutdown chan struct{}
	cancel   context.CancelFunc
}

// NewNodeManager constructs a Manager that will launch one node.
func NewNodeManager(logger zerolog.Logger, launcher *Launcher, baseDataDir string, portAssigner func() int) *Manager {
	return &Manager{
		logger:       logger.With().Str("component", "node-manager").Logger(),
		baseDataDir:  baseDataDir,
		launcher:     launcher,
		portAssigner: portAssigner,
		shutdown:     make(chan struct{}),
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
		P2PPort:    m.portAssigner(),
		RPCPort:    m.portAssigner(),
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
		_ = n.Close()
		close(m.shutdown)
	}()

	return nil
}

// GethNode returns the running node instance or nil if the node is not started.
func (m *Manager) GethNode() *gethnode.Node { return m.gethNode }

// RPCPort returns the RPC port the node is using.
func (m *Manager) RPCPort() int { return m.cfg.RPCPort }

// Wait blocks until the node has shut down.
func (m *Manager) Wait() {
	if m.cancel != nil {
		m.cancel()
	}
	<-m.shutdown
}
