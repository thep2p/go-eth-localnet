package node_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	gethnode "github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
)

// startNode is a helper that launches a single node and waits for RPC readiness.
func startNode(t *testing.T, opts ...node.LaunchOption) (
	context.Context,
	context.CancelFunc,
	*node.Manager,
	*gethnode.Node,
) {
	t.Helper()

	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(
		testutils.Logger(t), launcher, tmp.Path(), func() int {
			return testutils.NewPort(t)
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(tmp.Remove)
	t.Cleanup(manager.Wait)

	require.NoError(t, manager.Start(ctx, opts...))
	gethNode := manager.GethNode()
	require.NotNil(t, gethNode)

	testutils.RequireRpcReadyWithinTimeout(t, ctx, manager.RPCPort(), 5*time.Second)

	return ctx, cancel, manager, gethNode
}

// TestClientVersion verifies that the node returns a valid
// identifier for the `web3_clientVersion` RPC call.
func TestClientVersion(t *testing.T) {
	ctx, cancel, manager, _ := startNode(t)
	defer cancel()

	client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	var ver string
	require.NoError(t, client.CallContext(ctx, &ver, "web3_clientVersion"))
	require.NotEmpty(t, ver)
	require.Contains(t, ver, "/")
}

// TestBlockProduction ensures that the single node produces blocks when mining.
func TestBlockProduction(t *testing.T) {
	ctx, cancel, manager, _ := startNode(t)
	defer cancel()

	require.Eventually(
		t, func() bool {
			client, err := rpc.DialContext(
				ctx,
				fmt.Sprintf("http://127.0.0.1:%d", manager.RPCPort()),
			)
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
		}, 15*time.Second, 500*time.Millisecond, "node failed to produce blocks",
	)
}

// TestBlockProductionMonitoring verifies that block numbers advance over time.
func TestBlockProductionMonitoring(t *testing.T) {
	ctx, cancel, manager, _ := startNode(t)
	defer cancel()

	client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", manager.RPCPort()))
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

// TestPostMergeBlockStructureValidation verifies the structure of blocks post-merge, ensuring
// PoW-related fields are zero or empty, and block production is functioning correctly.
func TestPostMergeBlockStructureValidation(t *testing.T) {
	ctx, cancel, manager, _ := startNode(t)
	defer cancel()

	client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	// Fetch the latest block to validate its structure
	// the block map holds the latest block data by its attributes
	var block map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(
				ctx,
				&block,
				"eth_getBlockByNumber",
				"latest",
				false,
			); err != nil {
				return false
			}
			return true
		}, 5*time.Second, 500*time.Millisecond, "could not fetch latest block",
	)

	// Ethereum post-merge transitioned to PoS, so PoW-related fields should be zero or empty.
	// Difficulty is the computational effort required to mine a block, which is no longer applicable.
	diffStr, ok := block["difficulty"].(string)
	require.True(t, ok)
	require.Equal(t, "0x0", strings.ToLower(diffStr), "difficulty should be zero post-merge")

	// Total difficulty is the cumulative difficulty of all blocks up to this point, which should also be zero for the first block.
	// If totalDifficulty is not set, it defaults to "0x0".
	tdStr, _ := block["totalDifficulty"].(string)
	if tdStr == "" {
		tdStr = "0x0"
	}
	// Convert the total difficulty string to a big.Int for validation.
	td, ok := new(big.Int).SetString(strings.TrimPrefix(tdStr, "0x"), 16)
	require.True(t, ok)
	require.Zero(t, td.Int64())

	// Post-merge, mixHash represents commitment to the randomness in block proposal.
	// It should be a non-empty string.
	mix1, ok := block["mixHash"].(string)
	require.True(t, ok)
	require.NotEmpty(t, mix1)

	// mixHash should change with each new block, so we will fetch the latest block again
	// to ensure block production is working and mixHash is updated.
	require.Eventually(
		t, func() bool {
			var block2 map[string]interface{}
			if err := client.CallContext(
				ctx,
				&block2,
				"eth_getBlockByNumber",
				"latest",
				false,
			); err != nil {
				return false
			}
			mix2, ok := block2["mixHash"].(string)
			require.True(t, ok, "mixHash should be a string")
			require.NotEmpty(t, mix2, "mixHash should not be empty in the latest block")
			if mix1 == mix2 {
				return false // mixHash should change with each new block
			}
			return true
		}, 3*time.Second, 500*time.Millisecond, "could not fetch latest block again",
	)

}

// TestSimpleETHTransfer validates basic transaction processing.
func TestSimpleETHTransfer(t *testing.T) {
	// Creates two accounts with 1 ETH each, sends a transaction from one to the other,
	// Accounts A and B.
	aKey := testutils.PrivateKeyFixture(t)
	aAddr := crypto.PubkeyToAddress(aKey.PublicKey)

	bKey := testutils.PrivateKeyFixture(t)
	bAddr := crypto.PubkeyToAddress(bKey.PublicKey)

	oneEth := new(big.Int).Mul(big.NewInt(1), big.NewInt(params.Ether))

	ctx, cancel, manager, _ := startNode(t, node.WithGenesisAccount(aAddr, oneEth))
	defer cancel()

	client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	balA := testutils.GetBalance(t, ctx, client, aAddr)
	balB := testutils.GetBalance(t, ctx, client, bAddr)
	require.Equal(t, oneEth, balA)
	require.Zero(t, balB.Int64())

	var nonceHex string
	require.NoError(
		t,
		client.CallContext(ctx, &nonceHex, "eth_getTransactionCount", aAddr.Hex(), "latest"),
	)
	nonce := testutils.HexToBigInt(t, nonceHex)

	value := new(big.Int).Div(oneEth, big.NewInt(10))
	gasLimit := uint64(21000)
	gasTip := big.NewInt(params.GWei)
	gasFeeCap := new(big.Int).Mul(big.NewInt(2), gasTip)

	tx := types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   big.NewInt(1337),
			Nonce:     nonce.Uint64(),
			Gas:       gasLimit,
			GasTipCap: gasTip,
			GasFeeCap: gasFeeCap,
			To:        &bAddr,
			Value:     value,
		},
	)

	signer := types.LatestSignerForChainID(big.NewInt(1337))
	signedTx, err := types.SignTx(tx, signer, aKey)
	require.NoError(t, err)

	txBytes, err := signedTx.MarshalBinary()
	require.NoError(t, err)

	var txHash common.Hash
	require.NoError(
		t,
		client.CallContext(
			ctx,
			&txHash,
			"eth_sendRawTransaction",
			"0x"+hex.EncodeToString(txBytes),
		),
	)

	var receipt map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(
				ctx,
				&receipt,
				"eth_getTransactionReceipt",
				txHash,
			); err != nil {
				return false
			}
			return receipt != nil && receipt["blockNumber"] != nil
		}, 5*time.Second, 500*time.Millisecond, "receipt not available",
	)

	require.Equal(t, "0x1", receipt["status"])

	gasUsedHex, ok := receipt["gasUsed"].(string)
	require.True(t, ok)
	gasUsed, _ := new(big.Int).SetString(strings.TrimPrefix(gasUsedHex, "0x"), 16)

	effGasPriceHex, ok := receipt["effectiveGasPrice"].(string)
	require.True(t, ok)
	effGasPrice, _ := new(big.Int).SetString(strings.TrimPrefix(effGasPriceHex, "0x"), 16)

	expectedA := new(big.Int).Sub(
		balA,
		new(big.Int).Add(value, new(big.Int).Mul(gasUsed, effGasPrice)),
	)
	expectedB := new(big.Int).Add(balB, value)
	require.Equal(t, expectedA, testutils.GetBalance(t, ctx, client, aAddr))
	require.Equal(t, expectedB, testutils.GetBalance(t, ctx, client, bAddr))
}
