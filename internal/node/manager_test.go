package node_test

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
	"testing"
	"time"
)

func TestStartMultipleNodes(t *testing.T) {
	t.Parallel()
	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx, 3)
	require.NoError(t, err)
	require.Len(t, manager.Handles(), 3)
}

// TestStaticNodesAreNotIgnored validates that Geth does not ignore static-nodes.json
// if the file is written before the node has already started. This regression test verifies that
// writing static peer lists after stack.Start() is too late for Geth to load them.
//
// The test starts a local network of 3 nodes, writes static-nodes.json after startup (buggy path),
// shuts down, and restarts the nodes. Even on restart, peer count remains zero, confirming the bug.
//
// Fixing this test requires static-nodes.json to be written before node launch.
func TestStaticNodesAreNotIgnored(t *testing.T) {
	t.Parallel()

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

	//// Confirm static-nodes.json exists and is non-empty
	//for _, h := range manager.Handles() {
	//	staticPath := filepath.Join(h.DataDir(), "geth", "static-nodes.json")
	//	data, err := os.ReadFile(staticPath)
	//	require.NoError(t, err)
	//	require.Contains(t, string(data), "enode://")
	//}

	defer func() {
		// Shutdown the network
		cancel()
		for _, h := range manager.Handles() {
			testutils.RequirePortClosesWithinTimeout(t, h.RpcPort(), 5*time.Second)
		}
	}()

	// Expect non-zero peer counts (but will fail because static-nodes.json was ignored before)
	for _, h := range manager.Handles() {
		//require.Eventually(t, func() bool {
		//	endpoint := fmt.Sprintf("http://127.0.0.1:%d", h.RpcPort())
		//	client, err := rpc.Dial(endpoint)
		//	require.NoError(t, err)
		//
		//	var peerCount model.HexInt
		//	err = client.Call(&peerCount, "net_peerCount")
		//	require.NoError(t, err)
		//
		//	t.Logf("Node %s has %d peers", h.ID().String(), peerCount)
		//	return int(peerCount) > 0
		//}, 30*time.Second, 100*time.Millisecond, "Expected non-zero peer count after static-nodes.json was written")
		require.Eventually(t, func() bool {
			t.Logf("Node %s has %d peers", h.ID().String(), len(h.Server().Peers()))
			return len(h.Server().Peers()) > 0
		}, 10*time.Second, 250*time.Millisecond, "Expected non-zero peer count after static-nodes.json was written")
	}

	//// Relaunch using same datadirs
	//ctx2, cancel2 := context.WithCancel(context.Background())
	//defer cancel2()
	//manager2 := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))
	//err = manager2.Start(ctx2, 3)
	//require.NoError(t, err)
	//
	//for _, h := range manager2.Handles() {
	//	testutils.RequireRpcReadyWithinTimeout(t, ctx2, h.RpcPort(), 5*time.Second)
	//}

}

func TestManagerUniquePortsAndRPC(t *testing.T) {
	t.Parallel()

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

func TestManagerPeersPersistWithStaticNodes(t *testing.T) {
	t.Parallel()

	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, manager.Start(ctx, 3))
	handles := manager.Handles()
	require.Len(t, handles, 3)

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
			return len(peers) == len(handles)-1
		}, 10*time.Second, 250*time.Millisecond, "peer count mismatch")
	}

	cancel()
	for _, h := range handles {
		testutils.RequirePortClosesWithinTimeout(t, h.RpcPort(), 5*time.Second)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	manager2 := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), testutils.NewPortAssigner(t))

	require.NoError(t, manager2.Start(ctx2, 3))
	handles2 := manager2.Handles()
	require.Len(t, handles2, 3)

	for _, h := range handles2 {
		testutils.RequireRpcReadyWithinTimeout(t, ctx2, h.RpcPort(), 5*time.Second)
	}

	for _, h := range handles2 {
		require.Eventually(t, func() bool {
			client, err := rpc.DialContext(ctx2, fmt.Sprintf("http://127.0.0.1:%d", h.RpcPort()))
			if err != nil {
				return false
			}
			defer client.Close()
			var peers []any
			if err := client.CallContext(ctx2, &peers, "admin_peers"); err != nil {
				return false
			}
			return len(peers) == len(handles2)-1
		}, 10*time.Second, 250*time.Millisecond, "peer count mismatch on restart")
	}
}
