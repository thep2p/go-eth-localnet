package node

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"net"
	"os"
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
	enodes := make([]string, n)
	configs := make([]model.Config, n)

	// Step 1: Precompute keys configs, and enode URLs
	for i := 0; i < n; i++ {
		priv, err := crypto.GenerateKey()
		if err != nil {
			return fmt.Errorf("generate key for node %d: %w", i, err)
		}

		p2pPort := m.portAssigner.NewPort()
		rpcPort := m.portAssigner.NewPort()
		enodeURL := enode.NewV4(&priv.PublicKey, net.IPv4(127, 0, 0, 1), p2pPort, 0).String()

		cfg := model.Config{
			ID:         enode.PubkeyToIDV4(&priv.PublicKey),
			DataDir:    filepath.Join(m.baseDataDir, fmt.Sprintf("node%d", i)),
			P2PPort:    p2pPort,
			RPCPort:    rpcPort,
			PrivateKey: priv,
			EnodeURL:   enodeURL,
		}
		configs[i] = cfg
		enodes[i] = enodeURL
	}

	// Step 2: Write static-nodes.json
	for i, cfg := range configs {
		peers := append(enodes[:i], enodes[i+1:]...)
		if err := writeStaticPeers(cfg.DataDir, peers); err != nil {
			return fmt.Errorf("node %d static peers: %w", i, err)
		}
	}

	// Step 3: Launch nodes with assigned private keys and enode URLs
	for i, cfg := range configs {
		handle, err := m.launcher.Launch(cfg)
		if err != nil {
			return fmt.Errorf("node %d launch: %w", i, err)
		}
		m.handlesMu.Lock()
		m.handles = append(m.handles, handle)
		m.handlesMu.Unlock()

		enodes[i] = handle.NodeURL() // Update real enode
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

// writeStaticPeers writes the given list of peer URLs to a static-nodes.json file in the specified data directory.
// Static nodes in Geth are specifically configured network nodes that this node will always try
// to maintain connections with. They are configured through the static-nodes.json file.
// These nodes are exempt from maximum peer limits, hence ensuring reliable network connectivity
// with trusted nodes. This also helps with the intial peer discovery.
//
// The static-nodes.json file contains a list of enode URLs in JSON format.
// When Geth node starts, it reads this file and attempts to establish connections with
// the nodes listed in it. This is particularly useful for private networks or testnets.
// These connections always persist regardless of the dynamic peer discovery process.
// File format
// ```json
// [
//
//	"enode://nodeID1@ip1:port1",
//	"enode://nodeID2@ip2:port2"
//
// ]
// ```
// Note this is particularly useful in this project as we need a private network with a full mesh topology,
// and reliable connections between nodes.
// Returns an error if the file cannot be written or if the peers cannot be encoded into JSON.
func writeStaticPeers(dataDir string, peers []string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("mkdir geth dir: %w", err)
	}

	encoded, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal peers: %w", err)
	}

	staticPath := filepath.Join(dataDir, "static-nodes.json")
	if err := os.WriteFile(staticPath, encoded, 0644); err != nil {
		return fmt.Errorf("write static-nodes.json: %w", err)
	}

	// read and print the file to verify
	//data, err := os.ReadFile(staticPath)
	//if err != nil {
	//	return fmt.Errorf("read static-nodes.json: %w", err)
	//}
	//fmt.Printf("static-nodes.json: %s\n", data)

	return nil
}

// Handles returns a slice of all currently managed node handles.
func (m *Manager) Handles() []*model.Handle {
	m.handlesMu.Lock()
	defer m.handlesMu.Unlock()

	return append([]*model.Handle{}, m.handles...)
}
