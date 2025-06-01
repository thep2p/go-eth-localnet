package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Manager struct {
	launcher *Launcher
	handles  []*Handle
}

func NewNodeManager(launcher *Launcher) *Manager {
	return &Manager{launcher: launcher}
}

func (m *Manager) StartN(n int, baseDataDir string, baseP2PPort, baseRPCPort int) error {
	enodes := make([]string, n)

	// Step 1: Launch each node
	for i := 0; i < n; i++ {
		cfg := Config{
			ID:      i,
			DataDir: filepath.Join(baseDataDir, fmt.Sprintf("node%d", i)),
			P2PPort: baseP2PPort + i,
			RPCPort: baseRPCPort + i,
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
	encoded, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, "geth", "static-nodes.json"), encoded, 0644)
}
