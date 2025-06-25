// Package node provides functionality for managing Ethereum nodes in a local network environment.
// It orchestrates a full-mesh by launching in-process Geth nodes and dialing each peer directly.
package node

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal"
	"github.com/thep2p/go-eth-localnet/internal/model"
)

// Manager orchestrates a mesh of Geth nodes by dialing peers directly via the P2P server.
type Manager struct {
	logger       zerolog.Logger
	baseDataDir  string
	launcher     *Launcher
	portAssigner internal.PortAssigner

	mu       sync.Mutex
	handles  []*model.Handle
	shutdown sync.WaitGroup
	cancel   context.CancelFunc
}

// cleanup stops all running nodes and waits for them to exit.
// It should only be used when Start encounters an error before
// the network's shutdown goroutine has been spawned.
func (m *Manager) cleanup() {
	for _, h := range m.handles {
		_ = h.Close()
		m.shutdown.Done()
	}
	m.handles = nil
}

// NewNodeManager constructs a Manager that will launch and wire up n nodes.
func NewNodeManager(logger zerolog.Logger, launcher *Launcher, baseDataDir string, portAssigner internal.PortAssigner) *Manager {
	return &Manager{
		logger:       logger.With().Str("component", "node-manager").Logger(),
		baseDataDir:  baseDataDir,
		launcher:     launcher,
		portAssigner: portAssigner,
	}
}

// Start launches n nodes, waits for them to accept RPC, then dials each into a full mesh.
// Start launches n nodes, configures static peers, and waits until each node's
// RPC endpoint becomes reachable. It returns an error if any node fails to
// launch or expose its RPC within the timeout.
func (m *Manager) Start(ctx context.Context, n int) error {
	ctx, m.cancel = context.WithCancel(ctx)
	// 1) Prepare configs and enode URLs for all nodes
	configs := make([]model.Config, n)
	enodes := make([]string, n)
	for i := 0; i < n; i++ {
		priv, err := crypto.GenerateKey()
		if err != nil {
			return fmt.Errorf("generate key for node %d: %w", i, err)
		}

		cfg := model.Config{
			ID:         enode.PubkeyToIDV4(&priv.PublicKey),
			DataDir:    filepath.Join(m.baseDataDir, fmt.Sprintf("node%d", i)),
			P2PPort:    m.portAssigner.NewPort(),
			RPCPort:    m.portAssigner.NewPort(),
			PrivateKey: priv,
			Mine:       i == 0,
		}

		url := enode.NewV4(&priv.PublicKey, net.IP{127, 0, 0, 1}, cfg.P2PPort, 0).URLv4()
		configs[i] = cfg
		enodes[i] = url
	}

	// 2) Populate static nodes for each config before launch
	for i := range configs {
		for j, url := range enodes {
			if i == j {
				continue
			}
			configs[i].StaticNodes = append(configs[i].StaticNodes, url)
		}
	}

	// 3) Launch all nodes
	for i := range configs {
		m.logger.Info().Int("index", i).Msg("Launching node")
		h, err := m.launcher.Launch(configs[i])
		if err != nil {
			m.cleanup()
			return fmt.Errorf("launch node %d: %w", i, err)
		}

		m.mu.Lock()
		m.handles = append(m.handles, h)
		m.mu.Unlock()
		m.shutdown.Add(1)
	}

	// 4) Wait for each node's RPC to be ready
	for _, h := range m.handles {
		rpcURL := fmt.Sprintf("http://127.0.0.1:%d", h.RpcPort())
		deadline := time.Now().Add(5 * time.Second)
		for {
			if time.Now().After(deadline) {
				m.cleanup()
				return fmt.Errorf("rpc %q never came up", rpcURL)
			}
			client, err := rpc.DialContext(ctx, rpcURL)
			if err == nil {
				client.Close()
				m.logger.Info().Str("node", h.ID().String()).Msg("RPC ready")
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 5) Dial each peer directly via the P2P server
	for i, h := range m.handles {
		srv := h.Server()
		for j, url := range enodes {
			if i == j {
				continue
			}
			peer, err := enode.Parse(enode.ValidSchemes, url)
			if err != nil {
				m.cleanup()
				return fmt.Errorf("parse peer %q: %w", url, err)
			}
			m.logger.Debug().Str("from", h.ID().String()).Str("to", peer.String()).Msg("Add peer")
			srv.AddPeer(peer)
		}
	}

	// 6) Clean up on context cancel
	go func() {
		<-ctx.Done()
		for _, h := range m.handles {
			m.logger.Info().Str("node", h.ID().String()).Msg("Shutting down")
			_ = h.Close()
			m.shutdown.Done()
		}
	}()

	return nil
}

// Handles returns a snapshot of the active node handles.
func (m *Manager) Handles() []*model.Handle {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*model.Handle{}, m.handles...)
}

// Wait cancels the network context and blocks until all nodes have shut down.
func (m *Manager) Wait() {
	if m.cancel != nil {
		m.cancel()
	}
	m.shutdown.Wait()
}
