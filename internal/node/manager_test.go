package node_test

import (
	"context"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/thep2p/go-eth-localnet/internal/contracts"
	"github.com/thep2p/go-eth-localnet/internal/model"
	"github.com/thep2p/go-eth-localnet/internal/utils"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
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
				t, manager.Done, node.ShutdownTimeout, "node shutdown failed",
			)
		},
	)

	require.NoError(t, manager.Start(ctx, opts...))
	gethNode := manager.GethNode()
	require.NotNil(t, gethNode)

	testutils.RequireRpcReadyWithinTimeout(t, ctx, manager.RPCPort(), node.OperationTimeout)

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
			// Block number should be at least 3 to ensure the node is producing blocks (at least 2 blocks + genesis).
			return num.Uint64() >= 3
		}, 3*node.OperationTimeout, 500*time.Millisecond, "node failed to produce blocks",
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
		}, node.OperationTimeout, 500*time.Millisecond, "block number did not increase",
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
				ctx, &block, model.EthGetBlockByNumber, model.EthBlockLatest, false,
			); err != nil {
				return false
			}
			return true
		}, node.OperationTimeout, 500*time.Millisecond, "could not fetch latest block",
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
				ctx, &block2, model.EthGetBlockByNumber, model.EthBlockLatest, false,
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
		}, node.OperationTimeout, 500*time.Millisecond, "could not fetch latest block again",
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

	ctx, cancel, manager := startNode(t, node.WithPreFundGenesisAccount(aAddr, oneEth))
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
			model.EthBlockLatest,
		),
	)
	nonce := testutils.HexToBigInt(t, nonceHex)

	// Prepare a transaction to send 0.1 ETH from account A to account B.
	txValue := new(big.Int).Div(oneEth, big.NewInt(10))
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
			Value:     txValue,
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
		}, node.OperationTimeout, 500*time.Millisecond, "receipt not available",
	)

	require.Equal(t, model.ReceiptTxStatusSuccess, receipt[model.ReceiptStatus])

	gasUsedHex, ok := receipt[model.ReceiptGasUsed].(string)
	require.True(t, ok)
	gasUsed := testutils.HexToBigInt(t, gasUsedHex)
	// 0 <= gasUsed <= 21000
	require.Less(t, uint64(0), gasUsed.Uint64())
	require.LessOrEqual(t, gasUsed.Uint64(), gasLimit)

	effGasPriceHex, ok := receipt[model.ReceiptEffectiveGasPrice].(string)
	require.True(t, ok)
	effGasPrice := testutils.HexToBigInt(t, effGasPriceHex)

	// Balance of account A should decrease by the value sent plus the gas used times the effective gas price.
	expectedA := new(big.Int).Sub(
		balA,
		new(big.Int).Add(txValue, new(big.Int).Mul(gasUsed, effGasPrice)),
	)
	// Balance of account B should increase by the value sent.
	expectedB := new(big.Int).Add(balB, txValue)
	require.Equal(t, expectedA, testutils.GetBalance(t, ctx, client, aAddr))
	require.Equal(t, expectedB, testutils.GetBalance(t, ctx, client, bAddr))
}

// TestRevertingTransaction ensures that a transaction which reverts
// returns a failed receipt and still consumes gas.
func TestRevertingTransaction(t *testing.T) {
	key := testutils.PrivateKeyFixture(t)
	addr := crypto.PubkeyToAddress(key.PublicKey)

	oneEth := new(big.Int).Mul(big.NewInt(1), big.NewInt(params.Ether))

	ctx, cancel, manager := startNode(t, node.WithPreFundGenesisAccount(addr, oneEth))
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
			model.EthBlockLatest,
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
	require.Equal(t, model.ReceiptTxStatusFailure, receipt[model.ReceiptStatus])

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

	ctx, cancel, manager := startNode(t, node.WithPreFundGenesisAccount(addr, oneEth))
	defer cancel()

	client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	var nonceHex string
	require.NoError(t, client.CallContext(ctx, &nonceHex, model.EthGetTransactionCount, addr.Hex(), model.EthBlockLatest))
	nonce := testutils.HexToBigInt(t, nonceHex)

	// Set the gas limit, tip cap, and fee cap for the transaction.
	// We set a gas limit of 1,000,000 which is sufficient for contract deployment.
	gasLimit := uint64(1_000_000)
	gasTipCap := big.NewInt(params.GWei)
	gasFeeCap := new(big.Int).Mul(big.NewInt(2), gasTipCap)

	// Generate the ABI and bytecode for the SimpleStorageContract.
	// cf. internal/utils/contracts/SimpleStorageContract.sol
	bin, abiJSON, err := contracts.GenerateAbiAndBin("../contracts/SimpleStorageContract.sol")
	require.NoError(t, err, "failed to generate ABI and bytecode for SimpleStorageContract")

	// Deploys the contract using the bytecode.
	tx := types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   manager.ChainID(),
			Nonce:     nonce.Uint64(),
			Gas:       gasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			// To is nil as this is a contract deployment transaction, the signer is the contract creator.
			// There is no recipient address for contract creation.
			// We retrieve the contract address from the receipt after the transaction is included in a block.
			To: nil,
			// The value is set to 0 as we are not sending any Ether with the contract deployment.
			Value: big.NewInt(0),
			Data:  common.FromHex(bin),
		},
	)

	// Sign the transaction with the private key of the contract creator.
	signer := types.LatestSignerForChainID(manager.ChainID())
	signedTx, err := types.SignTx(tx, signer, key)
	require.NoError(t, err)

	txBytes, err := signedTx.MarshalBinary()
	require.NoError(t, err)

	// Send the signed transaction to the node and get the transaction hash.
	var txHash common.Hash
	require.NoError(t, client.CallContext(ctx, &txHash, model.EthSendRawTransaction, utils.ByteToHex(txBytes)))

	// Eventually the transaction should be included in a block and a receipt should be available.
	var receipt map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(ctx, &receipt, model.EthGetTransactionReceipt, txHash); err != nil {
				return false
			}
			return receipt != nil && receipt[model.ReceiptBlockNumber] != nil
		}, 5*time.Second, 500*time.Millisecond, "receipt not available",
	)

	require.Equal(t, model.ReceiptTxStatusSuccess, receipt[model.ReceiptStatus])

	addrHex, ok := receipt[model.ReceiptContractAddress].(string)
	require.True(t, ok)
	contractAddr := common.HexToAddress(addrHex)

	// Verify that the contract was deployed by checking its bytecode.
	var code string
	require.NoError(t, client.CallContext(ctx, &code, model.ReceiptGetByteCode, contractAddr.Hex(), model.EthBlockLatest))
	// The bytecode should not be empty, indicating the contract was deployed successfully.
	require.NotEqual(t, model.AccountEmptyContract, code)

	contractABI, err := abi.JSON(strings.NewReader(abiJSON))
	require.NoError(t, err)

	// Call the `value` function of the SimpleStorageContract to get the initial value (should be 0).
	// cf. internal/utils/contracts/SimpleStorageContract.sol
	callData, err := contractABI.Pack("value")
	require.NoError(t, err)

	// Call the contract to get the current `value` (without sending a transaction).
	var valHex string
	require.NoError(
		t, client.CallContext(
			ctx, &valHex, model.CallContextEthCall, map[string]string{
				model.CallContextTo: contractAddr.Hex(), model.CallContextData: utils.ByteToHex(callData),
			}, model.EthBlockLatest,
		),
	)
	val := testutils.HexToBigInt(t, valHex)
	// The initial value should be 0 since the contract is just deployed.
	require.Zero(t, val.Int64())

	// Now we will set a new value (e.g., 7) using the `set` function of the contract.
	// First, we need to get the nonce for the next transaction.
	var nonceHex2 string
	require.NoError(t, client.CallContext(ctx, &nonceHex2, model.EthGetTransactionCount, addr.Hex(), model.EthBlockLatest))
	nonce2 := testutils.HexToBigInt(t, nonceHex2)

	// Prepare the transaction to call the `set` function of the contract with a new value (7).
	// cf. internal/utils/contracts/SimpleStorageContract.sol
	setData, err := contractABI.Pack("set", big.NewInt(7))
	require.NoError(t, err)

	tx2 := types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   manager.ChainID(),
			Nonce:     nonce2.Uint64(),
			Gas:       gasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			// To is the contract address we just deployed.
			To: &contractAddr,
			// Value is set to 0 as we are not sending any Ether with this transaction.
			Value: big.NewInt(0),
			Data:  setData,
		},
	)

	// Sign the transaction with the private key of the contract creator.
	signedTx2, err := types.SignTx(tx2, signer, key)
	require.NoError(t, err)

	txBytes2, err := signedTx2.MarshalBinary()
	require.NoError(t, err)

	// Send the signed transaction to the node and get the transaction hash.
	var txHash2 common.Hash
	require.NoError(t, client.CallContext(ctx, &txHash2, model.EthSendRawTransaction, utils.ByteToHex(txBytes2)))

	// Eventually the transaction should be included in a block and a receipt should be available.
	var receipt2 map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(ctx, &receipt2, model.EthGetTransactionReceipt, txHash2); err != nil {
				return false
			}
			return receipt2 != nil && receipt2[model.ReceiptBlockNumber] != nil
		}, 5*time.Second, 500*time.Millisecond, "second receipt not available",
	)
	require.Equal(t, model.ReceiptTxStatusSuccess, receipt2[model.ReceiptStatus])

	// After the transaction is included in a block, we can check the new value.
	// We use the same callData to call the `value` function again.
	// This is not a transaction, but a call to the contract to get the current value.
	require.NoError(
		t, client.CallContext(
			ctx,
			&valHex,
			model.CallContextEthCall,
			map[string]string{model.CallContextTo: contractAddr.Hex(), model.CallContextData: utils.ByteToHex(callData)},
			model.EthBlockLatest,
		),
	)
	// The value should now be 7, as we set it in the previous transaction.
	val = testutils.HexToBigInt(t, valHex)
	require.Equal(t, int64(7), val.Int64())

	// The SimpleStorageContract emits an event when the value is set.
	// We can check the receipt for logs to verify that the event was emitted.
	// cf. internal/utils/contracts/SimpleStorageContract.sol
	// There should be one log in the receipt, corresponding to the ValueChanged event.
	logs, ok := receipt2[model.ReceiptLogs].([]interface{})
	require.True(t, ok)
	require.Len(t, logs, 1)

	// The log is a map with keys "topics" and "data".
	logObj, ok := logs[0].(map[string]interface{})
	require.True(t, ok)
	topics, ok := logObj[model.ReceiptLogTopics].([]interface{})
	require.True(t, ok)
	require.True(t, len(topics) > 0)

	// The first topic is the event signature hash for ValueChanged(uint256).
	// cf. internal/utils/contracts/SimpleStorageContract.sol
	eventID := crypto.Keccak256Hash([]byte("ValueChanged(uint256)"))
	require.Equal(t, eventID.Hex(), topics[0].(string))
	dataHex, ok := logObj[model.ReceiptLogData].(string)
	require.True(t, ok)

	// The data field contains the value set in the contract, which should be 7.
	data := common.FromHex(dataHex)
	eventVal := new(big.Int).SetBytes(data)
	require.Equal(t, int64(7), eventVal.Int64())
}

// TestPeerConnectivity verifies that nodes connect to each other and report a
// peer count greater than zero via the `net_peerCount` RPC method.
func TestPeerConnectivity(t *testing.T) {
	ctx1, cancel1, manager1 := startNode(t)
	defer cancel1()

	enodeURL := manager1.GethNode().Server().NodeInfo().Enode

	// second node configuration
	launcher2 := node.NewLauncher(testutils.Logger(t))
	tmp2 := testutils.NewTempDir(t)
	priv2 := testutils.PrivateKeyFixture(t)
	cfg2 := model.Config{
		ID:          enode.PubkeyToIDV4(&priv2.PublicKey),
		DataDir:     tmp2.Path(),
		P2PPort:     testutils.NewPort(t),
		RPCPort:     testutils.NewPort(t),
		PrivateKey:  priv2,
		StaticNodes: []string{enodeURL},
		Mine:        false,
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	t.Cleanup(tmp2.Remove)

	node2, err := launcher2.Launch(cfg2)
	require.NoError(t, err)

	go func() {
		<-ctx2.Done()
		_ = node2.Close()
	}()

	testutils.RequireRpcReadyWithinTimeout(t, ctx2, cfg2.RPCPort, node.OperationTimeout)

	node2Enode := node2.Server().NodeInfo().Enode

	client1, err := rpc.DialContext(ctx1, utils.LocalAddress(manager1.RPCPort()))
	require.NoError(t, err)
	defer client1.Close()

	client2, err := rpc.DialContext(ctx2, utils.LocalAddress(cfg2.RPCPort))
	require.NoError(t, err)
	defer client2.Close()

	// ensure node1 knows about node2
	require.NoError(t, client1.CallContext(ctx1, nil, "admin_addPeer", node2Enode))

	// wait for peer connections
	require.Eventually(
		t, func() bool {
			var count string
			if err := client1.CallContext(ctx1, &count, model.NetPeerCount); err != nil {
				return false
			}
			return count != "0x0"
		}, 10*time.Second, 500*time.Millisecond, "peers did not connect",
	)
}