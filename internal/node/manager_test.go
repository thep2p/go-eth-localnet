package node_test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
)

// startNode is a helper that launches a single node and waits for RPC readiness.
func startNode(t *testing.T) (context.Context, context.CancelFunc, *node.Manager, *model.Handle) {
	t.Helper()

	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(testutils.Logger(t), launcher, tmp.Path(), func() int {
		return testutils.NewPort(t)
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(tmp.Remove)
	t.Cleanup(manager.Wait)

	require.NoError(t, manager.Start(ctx))
	handle := manager.Handle()
	require.NotNil(t, handle)

	testutils.RequireRpcReadyWithinTimeout(t, ctx, handle.RpcPort(), 5*time.Second)

	return ctx, cancel, manager, handle
}

// TestClientVersion verifies that the node returns a valid
// identifier for the `web3_clientVersion` RPC call.
func TestClientVersion(t *testing.T) {
	ctx, cancel, _, handle := startNode(t)
	defer cancel()

	client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", handle.RpcPort()))
	require.NoError(t, err)
	defer client.Close()

	var ver string
	require.NoError(t, client.CallContext(ctx, &ver, "web3_clientVersion"))
	require.NotEmpty(t, ver)
	require.Contains(t, ver, "/")
}

// TestBlockProduction ensures that the single node produces blocks when mining.
func TestBlockProduction(t *testing.T) {
	ctx, cancel, _, handle := startNode(t)
	defer cancel()

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
		return num.Uint64() >= 3
	}, 15*time.Second, 500*time.Millisecond, "node failed to produce blocks")
}

// TestBlockProductionMonitoring verifies that block numbers advance over time.
func TestBlockProductionMonitoring(t *testing.T) {
	ctx, cancel, _, handle := startNode(t)
	defer cancel()

	client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", handle.RpcPort()))
	require.NoError(t, err)
	defer client.Close()

	var hex1 string
	require.NoError(t, client.CallContext(ctx, &hex1, "eth_blockNumber"))
	n1, ok := new(big.Int).SetString(strings.TrimPrefix(hex1, "0x"), 16)
	require.True(t, ok, "invalid block number")

	time.Sleep(5 * time.Second)

	var hex2 string
	require.NoError(t, client.CallContext(ctx, &hex2, "eth_blockNumber"))
	n2, ok := new(big.Int).SetString(strings.TrimPrefix(hex2, "0x"), 16)
	require.True(t, ok, "invalid block number")

	require.Greater(t, n2.Uint64(), n1.Uint64(), "block number did not increase")
}

// TestPostMergeBlockStructureValidation checks PoS block header fields.
func TestPostMergeBlockStructureValidation(t *testing.T) {
	ctx, cancel, _, handle := startNode(t)
	defer cancel()

	client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", handle.RpcPort()))
	require.NoError(t, err)
	defer client.Close()

	time.Sleep(5 * time.Second)

	var block map[string]interface{}
	require.NoError(t, client.CallContext(ctx, &block, "eth_getBlockByNumber", "latest", false))

	diffStr, ok := block["difficulty"].(string)
	require.True(t, ok)
	require.Equal(t, "0x0", strings.ToLower(diffStr))

	tdStr, _ := block["totalDifficulty"].(string)
	if tdStr == "" {
		tdStr = "0x0"
	}
	td, ok := new(big.Int).SetString(strings.TrimPrefix(tdStr, "0x"), 16)
	require.True(t, ok)
	require.Zero(t, td.Int64())

	mix1, ok := block["mixHash"].(string)
	require.True(t, ok)
	require.NotEmpty(t, mix1)

	time.Sleep(3 * time.Second)

	var block2 map[string]interface{}
	require.NoError(t, client.CallContext(ctx, &block2, "eth_getBlockByNumber", "latest", false))
	mix2, ok := block2["mixHash"].(string)
	require.True(t, ok)
	require.NotEmpty(t, mix2)
	require.NotEqual(t, mix1, mix2)
}
