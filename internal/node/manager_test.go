package node_test

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
	"testing"
	"time"
)

// TestStartMultipleNodes_Startup tests that starting multiple Geth nodes works correctly.
func TestStartMultipleNodes_Startup(t *testing.T) {
	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx, 3)
	require.NoError(t, err)
	require.Len(t, manager.Handles(), 3)

	// Wait briefly to allow nodes to start
	for _, h := range manager.Handles() {
		testutils.RequireRpcReadyWithinTimeout(t, ctx, h.RpcPort(), 5*time.Second)
	}

	defer func() {
		// Shutdown the network
		cancel()
		manager.Wait()
		for _, h := range manager.Handles() {
			testutils.RequirePortClosesWithinTimeout(t, h.RpcPort(), 5*time.Second)
		}
	}()

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
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, manager.Start(ctx, 3))
	handles := manager.Handles()
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
	defer cancel()
	manager = node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))

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

// TestSingleMinerChainSync verifies that a single mining node produces blocks
// and the remaining nodes follow the same chain without forking.
func TestSingleMinerChainSync(t *testing.T) {
	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer manager.Wait()

	require.NoError(t, manager.Start(ctx, 3))
	handles := manager.Handles()
	require.Len(t, handles, 3)

	for _, h := range handles {
		testutils.RequireRpcReadyWithinTimeout(t, ctx, h.RpcPort(), 5*time.Second)
	}

	// Wait until each node sees two peers
	require.Eventually(t, func() bool {
		for _, h := range handles {
			if len(h.Server().Peers()) != 2 {
				return false
			}
		}
		return true
	}, 2*time.Second, 100*time.Millisecond)

	// Wait for at least six blocks and ensure all nodes report the same head
	require.Eventually(t, func() bool {
		numbers := make([]uint64, len(handles))
		hashes := make([]string, len(handles))
		for i, h := range handles {
			client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", h.RpcPort()))
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
			numbers[i] = num.Uint64()

			var block map[string]any
			if err := client.CallContext(ctx, &block, "eth_getBlockByNumber", hexNum, false); err != nil {
				return false
			}
			hashes[i], _ = block["hash"].(string)
		}

		if numbers[0] < 6 {
			return false
		}
		for i := 1; i < len(numbers); i++ {
			if numbers[i] != numbers[0] || hashes[i] != hashes[0] {
				return false
			}
		}
		return true
	}, 10*time.Second, 250*time.Millisecond)

	// Ensure no uncles are present at the head block
	require.Eventually(t, func() bool {
		for _, h := range handles {
			client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", h.RpcPort()))
			if err != nil {
				return false
			}
			defer client.Close()
			var hexNum string
			if err := client.CallContext(ctx, &hexNum, "eth_blockNumber"); err != nil {
				return false
			}
			var block map[string]any
			if err := client.CallContext(ctx, &block, "eth_getBlockByNumber", hexNum, false); err != nil {
				return false
			}
			uncles, ok := block["uncles"].([]any)
			if !ok || len(uncles) != 0 {
				return false
			}
		}
		return true
	}, 10*time.Second, 250*time.Millisecond)
}
