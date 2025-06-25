package node_test

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
	"math/big"
	"strings"
	"testing"
	"time"
)

// TestStartMultipleNodes_Startup tests that starting multiple Geth nodes works correctly.
func TestStartMultipleNodes_Startup(t *testing.T) {
	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), func() int {
		return testutils.NewPort(t)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		manager.Wait()
		tmp.Remove()
	}()

	err := manager.Start(ctx, 3)
	require.NoError(t, err)
	require.Len(t, manager.Handles(), 3)
	require.True(t, manager.Handles()[0].Mining())
	require.False(t, manager.Handles()[1].Mining())
	require.False(t, manager.Handles()[2].Mining())
}

// TestMultipleGethNodes_StaticPeers validates that Geth does not ignore static-nodes.json
// if the file is written before the node has already started. This regression test verifies that
// writing static peer lists after stack.Start() is too late for Geth to load them.
//
// The test starts a local network of 3 nodes, writes static-nodes.json after startup (buggy path),
// shuts down, and restarts the nodes. Even on restart, peer count remains zero, confirming the bug.
//
// Fixing this test requires static-nodes.json to be written before node launch.
func TestMultipleGethNodes_StaticPeers(t *testing.T) {
	tmp := testutils.NewTempDir(t)

	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), func() int {
		return testutils.NewPort(t)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		manager.Wait()
		for _, h := range manager.Handles() {
			testutils.RequirePortClosesWithinTimeout(t, h.RpcPort(), 5*time.Second)
		}
		tmp.Remove()
	}()

	err := manager.Start(ctx, 3)
	require.NoError(t, err)
	require.Len(t, manager.Handles(), 3)

	// Wait briefly to allow nodes to start
	for _, h := range manager.Handles() {
		testutils.RequireRpcReadyWithinTimeout(t, ctx, h.RpcPort(), 5*time.Second)
	}

	// Expect non-zero peer counts (but will fail because static-nodes.json was ignored before)
	for _, h := range manager.Handles() {
		require.Eventually(t, func() bool {
			t.Logf("Node %s has %d peers", h.ID().String(), len(h.Server().Peers()))
			return len(h.Server().Peers()) > 0
		}, 10*time.Second, 250*time.Millisecond, "Expected non-zero peer count after static-nodes.json was written")
	}
}

// TestMultipleGethNodes_UniquePorts ensures that each node in the manager has a unique RPC and P2P port.
func TestMultipleGethNodes_UniquePorts(t *testing.T) {
	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), func() int {
		return testutils.NewPort(t)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		manager.Wait()
		tmp.Remove()
	}()

	require.NoError(t, manager.Start(ctx, 3))
	handles := manager.Handles()
	require.Len(t, handles, 3)

	rpcPorts := make(map[int]struct{})
	p2pPorts := make(map[int]struct{})
	for _, h := range handles {
		rpcPorts[h.RpcPort()] = struct{}{}

		n, err := enode.Parse(enode.ValidSchemes, h.NodeURL())
		require.NoError(t, err)
		p2pPorts[n.TCP()] = struct{}{}

		testutils.RequireRpcReadyWithinTimeout(t, ctx, h.RpcPort(), 5*time.Second)
		client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", h.RpcPort()))
		require.NoError(t, err)
		var ver string
		require.NoError(t, client.CallContext(ctx, &ver, "web3_clientVersion"))
		require.NotEmpty(t, ver)
		client.Close()
	}

	require.Equal(t, 3, len(rpcPorts), "expected unique RPC ports")
	require.Equal(t, 3, len(p2pPorts), "expected unique P2P ports")
}

// TestMultipleGethNodes_StaticPeers_PostRestart tests that all nodes form a full mesh via static peers,
// after a restart, ensuring that static nodes are correctly persisted and utilized.
// Test ensures full mesh connectivity by checking that each node sees all others as peers.
func TestMultipleGethNodes_StaticPeers_PostRestart(t *testing.T) {
	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), func() int {
		return testutils.NewPort(t)
	})

	ctx, cancel := context.WithCancel(context.Background())
	var handles []*model.Handle
	defer func() {
		cancel()
		manager.Wait()
		for _, h := range handles {
			testutils.RequirePortClosesWithinTimeout(t, h.RpcPort(), 5*time.Second)
		}
		tmp.Remove()
	}()

	require.NoError(t, manager.Start(ctx, 3))
	handles = manager.Handles()
	require.Len(t, handles, 3)

	// Ensure all nodes are ready and can communicate
	for _, h := range handles {
		testutils.RequireRpcReadyWithinTimeout(t, ctx, h.RpcPort(), 5*time.Second)
	}

	for _, h := range handles {
		require.Eventually(t, func() bool {
			client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", h.RpcPort()))
			if err != nil {
				return false
			}
			defer client.Close()
			var peers []any
			if err := client.CallContext(ctx, &peers, "admin_peers"); err != nil {
				return false
			}
			return len(peers) == len(handles)-1 // Each node should see all others as peers
		}, 10*time.Second, 250*time.Millisecond, "peer count mismatch")
	}

	// Shutdown the network, and then restart it to ensure peers persist
	cancel()
	manager.Wait()
	for _, h := range handles {
		testutils.RequirePortClosesWithinTimeout(t, h.RpcPort(), 5*time.Second)
	}

	ctx, cancel = context.WithCancel(context.Background())
	defer func() {
		cancel()
		manager.Wait()
		tmp.Remove()
	}()
	launcher = node.NewLauncher(testutils.Logger(t))
	manager = node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), func() int {
		return testutils.NewPort(t)
	})

	require.NoError(t, manager.Start(ctx, 3))
	handles2 := manager.Handles()
	require.Len(t, handles2, 3)

	// Ensure all nodes are ready and can communicate after restart
	for _, h := range handles2 {
		testutils.RequireRpcReadyWithinTimeout(t, ctx, h.RpcPort(), 5*time.Second)
	}

	for _, h := range handles2 {
		require.Eventually(t, func() bool {
			client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", h.RpcPort()))
			if err != nil {
				return false
			}
			defer client.Close()
			var peers []any
			if err := client.CallContext(ctx, &peers, "admin_peers"); err != nil {
				return false
			}
			return len(peers) == len(handles2)-1 // Each node should still see all others as peers after restart
		}, 10*time.Second, 250*time.Millisecond, "peer count mismatch on restart")
	}
}

// setupNodes is a helper function to initialize and start a specified number of nodes.
// It returns the context, manager, and handles for the started nodes.
func setupNodes(t *testing.T, numNodes int) (context.Context, context.CancelFunc, *node.Manager, []*model.Handle) {
	t.Helper()

	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), func() int {
		return testutils.NewPort(t)
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(manager.Wait)
	t.Cleanup(tmp.Remove)

	require.NoError(t, manager.Start(ctx, numNodes))
	handles := manager.Handles()
	require.Len(t, handles, numNodes)

	for _, h := range handles {
		testutils.RequireRpcReadyWithinTimeout(t, ctx, h.RpcPort(), 5*time.Second)
	}

	return ctx, cancel, manager, handles
}

// TestNodeStartupAndRPC ensures that a single node can be successfully launched
// and its RPC interface becomes responsive.
func TestNodeStartupAndRPC(t *testing.T) {
	// Setup will start one node and check for RPC readiness.
	// The test passes if setup completes without errors.
	_, cancel, _, _ := setupNodes(t, 1)
	defer cancel()

}

// TestPeerDiscovery verifies that multiple nodes can discover and connect to each other.
func TestPeerDiscovery(t *testing.T) {
	_, cancel, _, handles := setupNodes(t, 3)
	defer cancel()

	// We expect each of the 3 nodes to connect to the other 2.
	expectedPeers := 2

	// Wait until each node reports having two peers.
	require.Eventually(t, func() bool {
		for _, h := range handles {
			if len(h.Server().Peers()) != expectedPeers {
				return false
			}
		}
		return true
	}, 10*time.Second, 200*time.Millisecond, "nodes did not connect to the expected number of peers")
}

// TestSingleMinerBlockProduction checks if a single mining node can produce blocks.
func TestSingleMinerBlockProduction(t *testing.T) {
	ctx, cancel, _, handles := setupNodes(t, 1)
	handle := handles[0]
	t.Cleanup(cancel)

	// Wait for the node to produce at least 3 blocks.
	require.Eventually(t, func() bool {
		client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", handle.RpcPort()))
		if err != nil {
			return false
		}
		defer client.Close()

		var hexNum string
		if err := client.CallContext(ctx, &hexNum, "eth_blockNumber"); err != nil {
			return false
		}

		num, ok := new(big.Int).SetString(strings.TrimPrefix(hexNum, "0x"), 16)
		if !ok {
			return false
		}

		// Check if block number is at least 3.
		return num.Uint64() >= 3
	}, 15*time.Second, 500*time.Millisecond, "single node failed to produce blocks")
}
