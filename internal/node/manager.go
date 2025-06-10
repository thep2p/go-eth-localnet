package node

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"os"
	"path/filepath"
)

type Manager struct {
	logger zerolog.Logger
	// baseDataDir specifies the root directory for storing node data and configuration files.
	baseDataDir string
	// baseP2PPort specifies the starting port number for peer-to-peer communication.
	baseP2PPort int
	// baseRPCPort specifies the starting port number for remote procedure calls.
	baseRPCPort int
	launcher    *Launcher
	handles     []*model.Handle
}

func NewNodeManager(logger zerolog.Logger, launcher *Launcher, baseDataDir string, baseP2PPort int, baseRPCPort int) *Manager {
	return &Manager{
		logger:      logger.With().Str("component", "node-manager").Logger(),
		launcher:    launcher,
		baseDataDir: baseDataDir,
		baseP2PPort: baseP2PPort,
		baseRPCPort: baseRPCPort,
	}
}

func (m *Manager) Start(ctx context.Context, n int) error {
	enodes := make([]string, n)

	// Step 1: Launch each node
	for i := 0; i < n; i++ {
		cfg := model.Config{
			ID:      i,
			DataDir: filepath.Join(m.baseDataDir, fmt.Sprintf("node%d", i)),
			P2PPort: m.baseP2PPort + i,
			RPCPort: m.baseRPCPort + i,
		}
		handle, err := m.launcher.Launch(cfg)
		if err != nil {
			return fmt.Errorf("node %d launch: %w", i, err)
		}
		m.handles = append(m.handles, handle)
		enodes[i] = handle.NodeURL()
	}

	// Step 2: Write static-nodes.json for each node (full mesh)
	for i, h := range m.handles {
		peers := append(enodes[:i], enodes[i+1:]...)
		if err := writeStaticPeers(h.DataDir(), peers); err != nil {
			return fmt.Errorf("node %d static peers: %w", i, err)
		}
	}

	go func() {
		<-ctx.Done()
		m.logger.Info().Msg("Context cancelled, shutting down nodes")
		for _, handle := range m.handles {
			if err := handle.Close(); err != nil {
				m.logger.Error().Int("id", handle.ID()).Err(err).Msg("Failed to close node")
			} else {
				m.logger.Info().Int("id", handle.ID()).Msg("Node closed successfully")
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
	gethDir := filepath.Join(dataDir, "geth")
	if err := os.MkdirAll(gethDir, 0755); err != nil {
		return fmt.Errorf("mkdir geth dir: %w", err)
	}

	encoded, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal peers: %w", err)
	}

	staticPath := filepath.Join(gethDir, "static-nodes.json")
	if err := os.WriteFile(staticPath, encoded, 0644); err != nil {
		return fmt.Errorf("write static-nodes.json: %w", err)
	}
	return nil
}

// Handles returns a slice of all currently managed node handles.
func (m *Manager) Handles() []*model.Handle {
	return m.handles
}
