package node

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"net"
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
	// 1) Generate keys, ports, and “synthetic” enode URLs
	enodes := make([]string, n)
	cfgs := make([]model.Config, n)
	for i := 0; i < n; i++ {
		priv, _ := crypto.GenerateKey()
		p2pPort := m.portAssigner.NewPort()
		rpcPort := m.portAssigner.NewPort()
		// include ?discport=0 to match what Geth reports
		e := enode.NewV4(&priv.PublicKey, net.IPv4(127, 0, 0, 1), p2pPort, 0)
		enodes[i] = e.URLv4()

		cfgs[i] = model.Config{
			ID:         enode.PubkeyToIDV4(&priv.PublicKey),
			DataDir:    filepath.Join(m.baseDataDir, fmt.Sprintf("node%d", i)),
			P2PPort:    p2pPort,
			RPCPort:    rpcPort,
			PrivateKey: priv,
		}
	}

	// 2) Now launch each node *with* its static-peer list baked in
	for i := range cfgs {
		// peers = everyone except self
		peers := append(enodes[:i], enodes[i+1:]...)
		cfgs[i].StaticNodes = peers

		handle, err := m.launcher.Launch(cfgs[i])
		if err != nil {
			return err
		}
		m.handlesMu.Lock()
		m.handles = append(m.handles, handle)
		m.handlesMu.Unlock()
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
