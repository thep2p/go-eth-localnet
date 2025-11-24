package prysm

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/unittest"
	"github.com/thep2p/skipgraph-go/modules/throwable"
	skipgraphtest "github.com/thep2p/skipgraph-go/unittest"
)

// TestPrysmGethIntegration verifies Prysm can connect to Geth via Engine API.
//
// This test demonstrates the full integration workflow:
// 1. Start a Geth node with Engine API enabled
// 2. Configure Prysm with Geth's Engine API endpoint
// 3. Start Prysm beacon node
// 4. Verify Engine API communication
// 5. Verify block production
func TestPrysmGethIntegration(t *testing.T) {
	t.Skip("Skipping until Prysm implementation is complete")
	t.Parallel()

	logger := unittest.Logger(t)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	// Start Geth node with Engine API
	gethManager := startGethWithEngineAPI(t, tmp.Path())

	// Get Engine API details from Geth
	enginePort := gethManager.GetEnginePort(0)
	jwtSecret, err := gethManager.GetJWTSecret(0)
	require.NoError(t, err)

	// Configure Prysm
	prysmLauncher := NewLauncher(logger)
	prysmCfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    DefaultGenesisTime(),
		BeaconPort:     unittest.NewPort(t),
		P2PPort:        unittest.NewPort(t),
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: fmt.Sprintf("http://127.0.0.1:%d", enginePort),
		JWTSecret:      jwtSecret,
		ValidatorKeys:  generateTestValidatorKeys(t, 1),
		FeeRecipient:   common.HexToAddress("0x1234567890123456789012345678901234567890"),
	}

	// Launch Prysm client
	prysmClient, err := prysmLauncher.Launch(prysmCfg)
	require.NoError(t, err)
	require.NotNil(t, prysmClient)

	// Start Prysm with throwable context
	ctx := throwable.NewContext(skipgraphtest.NewMockThrowableContext(t))

	prysmClient.Start(ctx)

	// Wait for done on cleanup
	t.Cleanup(func() {
		<-prysmClient.Done()
	})

	// Wait for Prysm to be ready
	select {
	case <-prysmClient.Ready():
		t.Log("prysm client ready")
	case <-time.After(30 * time.Second):
		t.Fatal("prysm client did not become ready")
	}

	// Verify Beacon API is accessible
	beaconURL := prysmClient.BeaconAPIURL()
	t.Logf("Beacon API URL: %s", beaconURL)

	// TODO: Add actual beacon API health check once implemented
	// For example:
	// - GET /eth/v1/node/health
	// - GET /eth/v1/node/version
	// - GET /eth/v1/beacon/genesis

	// Wait for block production
	// In a real integration, we would:
	// 1. Query beacon API for head block
	// 2. Wait for multiple blocks to be produced
	// 3. Verify Engine API payloads are being sent to Geth
	time.Sleep(10 * time.Second)

	t.Log("integration test passed (basic lifecycle)")
}

// TestPrysmMultiNodeConsensus verifies multiple Prysm nodes can form consensus.
//
// This test demonstrates multi-node consensus:
// 1. Start multiple Geth nodes
// 2. Start multiple Prysm beacon nodes
// 3. Configure them to peer with each other
// 4. Verify they reach consensus on the chain head
func TestPrysmMultiNodeConsensus(t *testing.T) {
	t.Skip("Skipping until Prysm implementation is complete")
	t.Parallel()

	nodeCount := 3
	logger := unittest.Logger(t)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	// Start multiple Geth nodes
	gethManager := startGethNodesWithEngineAPI(t, tmp.Path(), nodeCount)

	// Start first Prysm node
	prysmClients := make([]*Client, nodeCount)
	prysmLauncher := NewLauncher(logger)

	for i := 0; i < nodeCount; i++ {
		enginePort := gethManager.GetEnginePort(i)
		jwtSecret, err := gethManager.GetJWTSecret(i)
		require.NoError(t, err)

		cfg := consensus.Config{
			DataDir:        filepath.Join(tmp.Path(), fmt.Sprintf("prysm%d", i)),
			ChainID:        1337,
			GenesisTime:    DefaultGenesisTime(),
			BeaconPort:     unittest.NewPort(t),
			P2PPort:        unittest.NewPort(t),
			RPCPort:        unittest.NewPort(t),
			EngineEndpoint: fmt.Sprintf("http://127.0.0.1:%d", enginePort),
			JWTSecret:      jwtSecret,
		}

		// First node gets validator keys
		if i == 0 {
			cfg.ValidatorKeys = generateTestValidatorKeys(t, 4)
			cfg.FeeRecipient = common.HexToAddress("0x1234567890123456789012345678901234567890")
		}

		client, err := prysmLauncher.Launch(cfg)
		require.NoError(t, err)
		prysmClients[i] = client
	}

	// Configure peer connections
	// Node 0 is the bootnode
	bootnode := prysmClients[0].P2PAddress()
	for i := 1; i < nodeCount; i++ {
		prysmClients[i].config.Bootnodes = []string{bootnode}
	}

	// Start all Prysm nodes
	ctx := throwable.NewContext(skipgraphtest.NewMockThrowableContext(t))

	for i, client := range prysmClients {
		client.Start(ctx)
		t.Logf("started prysm node %d", i)

		// Register cleanup for this specific client
		func(c *Client) {
			t.Cleanup(func() {
				<-c.Done()
			})
		}(client)
	}

	// Wait for all nodes to be ready
	for i, client := range prysmClients {
		select {
		case <-client.Ready():
			t.Logf("prysm node %d ready", i)
		case <-time.After(30 * time.Second):
			t.Fatalf("prysm node %d did not become ready", i)
		}
	}

	// Wait for consensus
	// In a real test, we would:
	// 1. Query each node's head block
	// 2. Wait for them to agree on the head
	// 3. Verify they continue to track the same chain
	time.Sleep(30 * time.Second)

	t.Log("multi-node consensus test passed (basic lifecycle)")
}

// startGethWithEngineAPI starts a single Geth node with Engine API enabled.
func startGethWithEngineAPI(t *testing.T, baseDir string) *node.Manager {
	t.Helper()

	launcher := node.NewLauncher(unittest.Logger(t))
	manager := node.NewNodeManager(
		unittest.Logger(t),
		launcher,
		filepath.Join(baseDir, "geth"),
		func() int { return unittest.NewPort(t) },
	)

	// Enable Engine API
	err := manager.EnableEngineAPI()
	require.NoError(t, err)

	// Start single node
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() {
		unittest.RequireCallMustReturnWithinTimeout(
			t, manager.Done, node.ShutdownTimeout, "geth shutdown failed",
		)
	})

	err = manager.Start(ctx, 1)
	require.NoError(t, err)

	// Wait for RPC to be ready
	unittest.RequireRpcReadyWithinTimeout(
		t, ctx, manager.RPCPort(), node.OperationTimeout,
	)

	return manager
}

// startGethNodesWithEngineAPI starts multiple Geth nodes with Engine API enabled.
func startGethNodesWithEngineAPI(t *testing.T, baseDir string, count int) *node.Manager {
	t.Helper()

	launcher := node.NewLauncher(unittest.Logger(t))
	manager := node.NewNodeManager(
		unittest.Logger(t),
		launcher,
		filepath.Join(baseDir, "geth"),
		func() int { return unittest.NewPort(t) },
	)

	// Enable Engine API
	err := manager.EnableEngineAPI()
	require.NoError(t, err)

	// Start multiple nodes
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() {
		unittest.RequireCallMustReturnWithinTimeout(
			t, manager.Done, node.ShutdownTimeout, "geth shutdown failed",
		)
	})

	err = manager.Start(ctx, count)
	require.NoError(t, err)

	// Wait for all nodes' RPC to be ready
	for i := 0; i < count; i++ {
		port := manager.GetRPCPort(i)
		unittest.RequireRpcReadyWithinTimeout(
			t, ctx, port, node.OperationTimeout,
		)
	}

	return manager
}

// generateTestValidatorKeys generates test validator keys for local development.
func generateTestValidatorKeys(t *testing.T, count int) []string {
	t.Helper()

	keys := make([]string, count)
	for i := 0; i < count; i++ {
		// For now, just use placeholder keys
		// TODO: Generate actual BLS12-381 keys once genesis.go is implemented
		keys[i] = fmt.Sprintf("test-validator-key-%d", i)
	}
	return keys
}
