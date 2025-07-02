package node_test

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
	"github.com/thep2p/go-eth-localnet/internal/utils"
)

// startNode initializes and starts a single Geth node for testing with given options.
// It sets up a temporary directory, a node manager, and ensures RPC readiness before returning.
func startNode(t *testing.T, opts ...node.LaunchOption) (
	context.Context,
	context.CancelFunc,
	*node.Manager) {
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
	t.Cleanup(
		func() {
			// ensure the node is stopped and cleaned up within a timeout
			testutils.RequireCallMustReturnWithinTimeout(
				t, manager.Done, 5*time.Second, "node shutdown failed",
			)
		},
	)

	require.NoError(t, manager.Start(ctx, opts...))
	gethNode := manager.GethNode()
	require.NotNil(t, gethNode)

	testutils.RequireRpcReadyWithinTimeout(t, ctx, manager.RPCPort(), 5*time.Second)

	return ctx, cancel, manager
}

// TestClientVersion verifies that the node returns a valid
// identifier for the `web3_clientVersion` RPC call.
func TestClientVersion(t *testing.T) {
	ctx, cancel, manager := startNode(t)
	defer cancel()

	client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	var ver string
	require.NoError(t, client.CallContext(ctx, &ver, model.EthWeb3ClientVersion))
	require.NotEmpty(t, ver)
	require.Contains(t, ver, "/")
}

// TestBlockProduction ensures that the single node produces blocks when mining.
func TestBlockProduction(t *testing.T) {
	ctx, cancel, manager := startNode(t)
	defer cancel()

	require.Eventually(
		t, func() bool {
			client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
			if err != nil {
				return false
			}
			defer client.Close()

			var hexNum string
			if err := client.CallContext(ctx, &hexNum, model.EthBlockNumber); err != nil {
				return false
			}

			num := testutils.HexToBigInt(t, hexNum)
			return num.Uint64() >= 3
		}, 15*time.Second, 500*time.Millisecond, "node failed to produce blocks",
	)
}

// TestBlockProductionMonitoring verifies that block numbers advance over time.
func TestBlockProductionMonitoring(t *testing.T) {
	ctx, cancel, manager := startNode(t)
	defer cancel()

	client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	var hex1 string
	require.NoError(t, client.CallContext(ctx, &hex1, model.EthBlockNumber))
	n1 := testutils.HexToBigInt(t, hex1)

	// Eventually the block number should increase, indicating that the node is producing blocks.
	require.Eventually(
		t, func() bool {
			var hex2 string
			require.NoError(t, client.CallContext(ctx, &hex2, model.EthBlockNumber))
			n2 := testutils.HexToBigInt(t, hex2)
			return n2.Uint64() > n1.Uint64()
		}, 5*time.Second, 500*time.Millisecond, "block number did not increase",
	)
}

// TestPostMergeBlockStructureValidation verifies the structure of blocks post-merge, ensuring
// PoW-related fields are zero or empty, and block production is functioning correctly.
func TestPostMergeBlockStructureValidation(t *testing.T) {
	ctx, cancel, manager := startNode(t)
	defer cancel()

	client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	// Fetch the latest block to validate its structure
	// the block map holds the latest block data by its attributes
	var block map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(
				ctx, &block, model.EthGetBlockByNumber, model.EthLatestBlock, false,
			); err != nil {
				return false
			}
			return true
		}, 5*time.Second, 500*time.Millisecond, "could not fetch latest block",
	)

	// Ethereum post-merge transitioned to PoS, so PoW-related fields should be zero or empty.
	// Difficulty is the computational effort required to mine a block, which is no longer applicable.
	diffStr, ok := block[model.BlockDifficulty].(string)
	require.True(t, ok)
	require.Equal(t, "0x0", strings.ToLower(diffStr), "difficulty should be zero post-merge")

	// Total difficulty is the cumulative difficulty of all blocks up to this point, which should also be zero for the first block.
	// If totalDifficulty is not set, it defaults to "0x0".
	tdStr, _ := block[model.BlockTotalDifficulty].(string)
	if tdStr == "" {
		tdStr = "0x0"
	}
	// Convert the total difficulty string to a big.Int for validation.
	td := testutils.HexToBigInt(t, tdStr)
	require.Zero(t, td.Int64())

	// Post-merge, mixHash represents commitment to the randomness in block proposal.
	// It should be a non-empty string.
	mix1, ok := block[model.BlockMixHash].(string)
	require.True(t, ok)
	require.NotEmpty(t, mix1)

	// mixHash should change with each new block, so we will fetch the latest block again
	// to ensure block production is working and mixHash is updated.
	require.Eventually(
		t, func() bool {
			var block2 map[string]interface{}
			if err := client.CallContext(
				ctx, &block2, model.EthGetBlockByNumber, model.EthLatestBlock, false,
			); err != nil {
				return false
			}
			mix2, ok := block2[model.BlockMixHash].(string)
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

	ctx, cancel, manager := startNode(t, node.WithGenesisAccount(aAddr, oneEth))
	defer cancel()

	client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	balA := testutils.GetBalance(t, ctx, client, aAddr)
	balB := testutils.GetBalance(t, ctx, client, bAddr)
	require.Equal(t, oneEth, balA)
	require.Zero(t, balB.Int64())

	var nonceHex string
	require.NoError(
		t,
		client.CallContext(
			ctx,
			&nonceHex,
			model.EthGetTransactionCount,
			aAddr.Hex(),
			model.EthLatestBlock,
		),
	)
	nonce := testutils.HexToBigInt(t, nonceHex)

	value := new(big.Int).Div(oneEth, big.NewInt(10))
	gasLimit := uint64(21000)            // Standard gas limit for ETH transfer transactions
	gasTipCap := big.NewInt(params.GWei) // Max tip we are willing to pay for the transaction
	// Max fee cap is set to 2x the tip cap, which is a common practice
	// to ensure the transaction is processed quickly.
	gasFeeCap := new(big.Int).Mul(big.NewInt(2), gasTipCap)

	tx := types.NewTx(
		&types.DynamicFeeTx{
			// Identify the chain ID for the transaction (1337 is a common local testnet ID)
			ChainID:   manager.ChainID(),
			Nonce:     nonce.Uint64(),
			Gas:       gasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			To:        &bAddr,
			Value:     value,
		},
	)

	// Sign the transaction with the private key of account A
	signer := types.LatestSignerForChainID(manager.ChainID())
	signedTx, err := types.SignTx(tx, signer, aKey)
	require.NoError(t, err)

	// Marshal the signed transaction to binary format
	txBytes, err := signedTx.MarshalBinary()
	require.NoError(t, err)

	// Send the signed transaction to the node and get the transaction hash
	var txHash common.Hash
	require.NoError(
		t,
		client.CallContext(ctx, &txHash, model.EthSendRawTransaction, utils.ByteToHex(txBytes)),
	)

	// Verify that eventually the transaction is included in a block, executed, and the balances are updated.
	var receipt map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(
				ctx, &receipt, model.EthGetTransactionReceipt, txHash,
			); err != nil {
				return false
			}
			return receipt != nil && receipt[model.ReceiptBlockNumber] != nil
		}, 5*time.Second, 500*time.Millisecond, "receipt not available",
	)

	// The receipt should indicate success (status 1) and gas used should be non-zero.
	require.Equal(t, "0x1", receipt[model.ReceiptStatus])

	gasUsedHex, ok := receipt[model.ReceiptGasUsed].(string)
	require.True(t, ok)
	gasUsed := testutils.HexToBigInt(t, gasUsedHex)

	effGasPriceHex, ok := receipt[model.ReceiptEffectiveGasPrice].(string)
	require.True(t, ok)
	effGasPrice := testutils.HexToBigInt(t, effGasPriceHex)

	// Balance of account A should decrease by the value sent plus the gas used times the effective gas price.
	expectedA := new(big.Int).Sub(
		balA,
		new(big.Int).Add(value, new(big.Int).Mul(gasUsed, effGasPrice)),
	)
	// Balance of account B should increase by the value sent.
	expectedB := new(big.Int).Add(balB, value)
	require.Equal(t, expectedA, testutils.GetBalance(t, ctx, client, aAddr))
	require.Equal(t, expectedB, testutils.GetBalance(t, ctx, client, bAddr))
}

// TestRevertingTransaction ensures that a transaction which reverts
// returns a failed receipt and still consumes gas.
func TestRevertingTransaction(t *testing.T) {
	key := testutils.PrivateKeyFixture(t)
	addr := crypto.PubkeyToAddress(key.PublicKey)

	oneEth := new(big.Int).Mul(big.NewInt(1), big.NewInt(params.Ether))

	ctx, cancel, manager := startNode(t, node.WithGenesisAccount(addr, oneEth))
	defer cancel()

	client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	var nonceHex string
	require.NoError(
		t,
		client.CallContext(
			ctx,
			&nonceHex,
			model.EthGetTransactionCount,
			addr.Hex(),
			model.EthLatestBlock,
		),
	)
	nonce := testutils.HexToBigInt(t, nonceHex)

	gasLimit := uint64(100000)
	gasTipCap := big.NewInt(params.GWei)
	gasFeeCap := new(big.Int).Mul(big.NewInt(2), gasTipCap)

	// This transaction is a contract creation (To: nil) with the init code `60006000fd`.
	// The code corresponds to: PUSH1 0x00; PUSH1 0x00; REVERT.
	// As a result, the contract constructor unconditionally reverts,
	// which causes the transaction to fail during execution despite being valid and mined.
	// This is useful for testing transaction failure paths, but will result in a receipt
	// with status 0 (failure) and no contract deployed.
	tx := types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   manager.ChainID(),
			Nonce:     nonce.Uint64(),
			Gas:       gasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			To:        nil,
			Value:     big.NewInt(0),
			Data:      common.Hex2Bytes("60006000fd"),
		},
	)

	// Sign the transaction with the private key
	signer := types.LatestSignerForChainID(manager.ChainID())
	signedTx, err := types.SignTx(tx, signer, key)
	require.NoError(t, err)

	// Marshal the signed transaction to binary format
	txBytes, err := signedTx.MarshalBinary()
	require.NoError(t, err)

	// Send the signed transaction to the node and get the transaction hash
	var txHash common.Hash
	require.NoError(
		t,
		client.CallContext(ctx, &txHash, model.EthSendRawTransaction, utils.ByteToHex(txBytes)),
	)

	// Verify that eventually the transaction is included in a block and executed.
	var receipt map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(
				ctx, &receipt, model.EthGetTransactionReceipt, txHash,
			); err != nil {
				return false
			}
			return receipt != nil && receipt[model.ReceiptBlockNumber] != nil
		}, 5*time.Second, 500*time.Millisecond, "receipt not available",
	)

	// The receipt should indicate a failure (status 0) and gas used should be non-zero.
	require.Equal(t, "0x0", receipt[model.ReceiptStatus])

	gasUsedHex, ok := receipt[model.ReceiptGasUsed].(string)
	require.True(t, ok)
	gasUsed := testutils.HexToBigInt(t, gasUsedHex)
	require.NotZero(t, gasUsed.Uint64())
}

// TestContractDeploymentAndInteraction verifies that a contract can be deployed,
// interacted with, and emits events as expected.
func TestContractDeploymentAndInteraction(t *testing.T) {
	key := testutils.PrivateKeyFixture(t)
	addr := crypto.PubkeyToAddress(key.PublicKey)

	oneEth := new(big.Int).Mul(big.NewInt(1), big.NewInt(params.Ether))

	ctx, cancel, manager := startNode(t, node.WithGenesisAccount(addr, oneEth))
	defer cancel()

	client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	var nonceHex string
	require.NoError(
		t,
		client.CallContext(
			ctx,
			&nonceHex,
			model.EthGetTransactionCount,
			addr.Hex(),
			model.EthLatestBlock,
		),
	)
	nonce := testutils.HexToBigInt(t, nonceHex)

	gasLimit := uint64(1_000_000)
	gasTipCap := big.NewInt(params.GWei)
	gasFeeCap := new(big.Int).Mul(big.NewInt(2), gasTipCap)

	bin := "0x6080604052348015600e575f5ffd5b506101778061001c5f395ff3fe608060405234801561000f575f5ffd5b5060043610610034575f3560e01c80633fa4f2451461003857806360fe47b114610056575b5f5ffd5b610040610072565b60405161004d91906100cf565b60405180910390f35b610070600480360381019061006b9190610116565b610077565b005b5f5481565b805f819055507f93fe6d397c74fdf1402a8b72e47b68512f0510d7b98a4bc4cbdf6ac7108b3c59816040516100ac91906100cf565b60405180910390a150565b5f819050919050565b6100c9816100b7565b82525050565b5f6020820190506100e25f8301846100c0565b92915050565b5f5ffd5b6100f5816100b7565b81146100ff575f5ffd5b50565b5f81359050610110816100ec565b92915050565b5f6020828403121561012b5761012a6100e8565b5b5f61013884828501610102565b9150509291505056fea2646970667358221220bff5b3d2cf840672c9b15e1fe904503f8ce19ad937610e1f5743c63ef3a838b664736f6c634300081e0033"
	abiJSON := `[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"newValue","type":"uint256"}],"name":"ValueChanged","type":"event"},{"inputs":[{"internalType":"uint256","name":"v","type":"uint256"}],"name":"set","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"value","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

	tx := types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   manager.ChainID(),
			Nonce:     nonce.Uint64(),
			Gas:       gasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			To:        nil,
			Value:     big.NewInt(0),
			Data:      common.FromHex(bin),
		},
	)

	signer := types.LatestSignerForChainID(manager.ChainID())
	signedTx, err := types.SignTx(tx, signer, key)
	require.NoError(t, err)

	txBytes, err := signedTx.MarshalBinary()
	require.NoError(t, err)

	var txHash common.Hash
	require.NoError(
		t,
		client.CallContext(ctx, &txHash, model.EthSendRawTransaction, utils.ByteToHex(txBytes)),
	)

	var receipt map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(
				ctx, &receipt, model.EthGetTransactionReceipt, txHash,
			); err != nil {
				return false
			}
			return receipt != nil && receipt[model.ReceiptBlockNumber] != nil
		}, 5*time.Second, 500*time.Millisecond, "receipt not available",
	)

	require.Equal(t, "0x1", receipt[model.ReceiptStatus])

	addrHex, ok := receipt["contractAddress"].(string)
	require.True(t, ok)
	contractAddr := common.HexToAddress(addrHex)

	var code string
	require.NoError(t, client.CallContext(ctx, &code, "eth_getCode", contractAddr.Hex(), model.EthLatestBlock))
	require.NotEqual(t, "0x", code)

	contractABI, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)

	callData, err := contractABI.Pack("value")
	require.NoError(t, err)

	var valHex string
	require.NoError(t, client.CallContext(ctx, &valHex, "eth_call", map[string]string{
		"to":   contractAddr.Hex(),
		"data": utils.ByteToHex(callData),
	}, model.EthLatestBlock))
	val := testutils.HexToBigInt(t, valHex)
	require.Zero(t, val.Int64())

	var nonceHex2 string
	require.NoError(
		t,
		client.CallContext(
			ctx,
			&nonceHex2,
			model.EthGetTransactionCount,
			addr.Hex(),
			model.EthLatestBlock,
		),
	)
	nonce2 := testutils.HexToBigInt(t, nonceHex2)

	setData, err := contractABI.Pack("set", big.NewInt(7))
	require.NoError(t, err)

	tx2 := types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   manager.ChainID(),
			Nonce:     nonce2.Uint64(),
			Gas:       gasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			To:        &contractAddr,
			Value:     big.NewInt(0),
			Data:      setData,
		},
	)

	signedTx2, err := types.SignTx(tx2, signer, key)
	require.NoError(t, err)

	txBytes2, err := signedTx2.MarshalBinary()
	require.NoError(t, err)

	var txHash2 common.Hash
	require.NoError(
		t,
		client.CallContext(ctx, &txHash2, model.EthSendRawTransaction, utils.ByteToHex(txBytes2)),
	)

	var receipt2 map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(
				ctx, &receipt2, model.EthGetTransactionReceipt, txHash2,
			); err != nil {
				return false
			}
			return receipt2 != nil && receipt2[model.ReceiptBlockNumber] != nil
		}, 5*time.Second, 500*time.Millisecond, "second receipt not available",
	)
	require.Equal(t, "0x1", receipt2[model.ReceiptStatus])

	require.NoError(t, client.CallContext(ctx, &valHex, "eth_call", map[string]string{
		"to":   contractAddr.Hex(),
		"data": utils.ByteToHex(callData),
	}, model.EthLatestBlock))
	val = testutils.HexToBigInt(t, valHex)
	require.Equal(t, int64(7), val.Int64())

	logs, ok := receipt2["logs"].([]interface{})
	require.True(t, ok)
	require.Len(t, logs, 1)
	logObj, ok := logs[0].(map[string]interface{})
	require.True(t, ok)
	topics, ok := logObj["topics"].([]interface{})
	require.True(t, ok)
	require.True(t, len(topics) > 0)
	eventID := crypto.Keccak256Hash([]byte("ValueChanged(uint256)"))
	require.Equal(t, eventID.Hex(), topics[0].(string))
	dataHex, ok := logObj["data"].(string)
	require.True(t, ok)
	data := common.FromHex(dataHex)
	eventVal := new(big.Int).SetBytes(data)
	require.Equal(t, int64(7), eventVal.Int64())
}
