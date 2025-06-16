package node

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"path/filepath"
	"sync"
)

type Manager struct {
	logger zerolog.Logger
	// baseDataDir specifies the root directory for storing node data and configuration files.
	baseDataDir string
	launcher    *Launcher
	// portAssigner is a function that returns an available P2P port for each node.
	portAssigner internal.PortAssigner
	// handlesMu is a mutex to protect concurrent access to the handles slice.
	handlesMu sync.Mutex // Protects access to handles
	// handles is a slice of node handles, each representing a running Geth node instance.
	handles []*model.Handle
}

func NewNodeManager(logger zerolog.Logger, launcher *Launcher, baseDataDir string, portAssigner internal.PortAssigner) *Manager {
	return &Manager{
		logger:       logger.With().Str("component", "node-manager").Logger(),
		launcher:     launcher,
		baseDataDir:  baseDataDir,
		portAssigner: portAssigner,
	}
}

func (m *Manager) Start(ctx context.Context, n int) error {
	// Step 1: launch all nodes
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
		}

		handle, err := m.launcher.Launch(cfg)
		if err != nil {
			return fmt.Errorf("node %d launch: %w", i, err)
		}
		m.handlesMu.Lock()
		m.handles = append(m.handles, handle)
		m.handlesMu.Unlock()
	}

	// Step 2: Collect enode URLs for static peers
	enodes := make([]string, len(m.handles))
	for i, handle := range m.handles {
		enodes[i] = handle.NodeURL()
	}

	// Step 3: parse and dial peers on each node
	for i, h := range m.handles {
		srv := h.Server()
		lg := m.logger.With().Str("node", h.ID().String()).Logger()
		for j, peerURL := range enodes {
			if i == j {
				continue // Skip self
			}
			peer, err := enode.Parse(enode.ValidSchemes, peerURL)
			if err != nil {
				return fmt.Errorf("parse peer %d for node %d: %w", j, i, err)
			}
			srv.AddPeer(peer)
			lg = lg.With().Str("added_peer", peer.ID().String()).Logger()
		}
		lg.Debug().Msg("Added peers to node")
	}

	// Step 4: Wait for shutdown
	go func() {
		<-ctx.Done()
		m.logger.Info().Msg("Context cancelled, shutting down nodes")
		for _, handle := range m.handles {
			if err := handle.Close(); err != nil {
				m.logger.Error().Str("id", handle.ID().String()).Err(err).Msg("Failed to close node")
			} else {
				m.logger.Info().Str("id", handle.ID().String()).Msg("Node closed successfully")
			}
		}
	}()

	return nil
}

// Handles returns a slice of all currently managed node handles.
func (m *Manager) Handles() []*model.Handle {
	m.handlesMu.Lock()
	defer m.handlesMu.Unlock()

	return append([]*model.Handle{}, m.handles...)
}
