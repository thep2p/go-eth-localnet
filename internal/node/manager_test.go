package node_test

import (
	"context"
	"github.com/ethereum/go-ethereum/accounts/abi"
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

// startNodes initializes and starts the specified number of Geth nodes for testing.
// It sets up a temporary directory, a node manager, and ensures RPC readiness before returning.
func startNodes(t *testing.T, nodeCount int, opts ...node.LaunchOption) (
	context.Context,
	context.CancelFunc,
	*node.Manager,
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
	t.Cleanup(
		func() {
			// ensure the node is stopped and cleaned up within a timeout
			testutils.RequireCallMustReturnWithinTimeout(
				t, manager.Done, node.ShutdownTimeout, "node shutdown failed",
			)
		},
	)

	require.NoError(t, manager.Start(ctx, nodeCount, opts...))
	gethNode := manager.GethNode()
	require.NotNil(t, gethNode)

	testutils.RequireRpcReadyWithinTimeout(t, ctx, manager.RPCPort(), node.OperationTimeout)

	return ctx, cancel, manager
}

// TestClientVersion verifies that the node returns a valid client version via web3_clientVersion RPC.
//
// This is the most basic health check - like asking "are you there?" to the node.
// The client version string identifies the software running the node (e.g., "Geth/v1.14.0...").
//
// Why this matters:
//   - Confirms the RPC endpoint is responsive (not crashed/frozen)
//   - Identifies which Ethereum client software and version is running
//   - Different clients (Geth, Besu, Nethermind) may have different behaviors
//   - Version info helps debug compatibility issues
//
// If this test fails, nothing else will work - it indicates:
//   - Node failed to start properly
//   - RPC port is not accessible
//   - Critical initialization failure
func TestClientVersion(t *testing.T) {
	ctx, cancel, manager := startNodes(t, 1)
	defer cancel()

	client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	var ver string
	require.NoError(t, client.CallContext(ctx, &ver, model.EthWeb3ClientVersion))
	require.NotEmpty(t, ver)
	require.Contains(t, ver, "/")
}

// TestBlockProduction verifies that the node actively produces new blocks.
//
// Blockchain basics: A blockchain is a chain of blocks, each containing transactions.
// New blocks must be regularly produced to process transactions and advance the chain.
// In Ethereum, blocks are produced approximately every 12 seconds.
//
// This test ensures our local node is:
//   - Running its consensus mechanism (simulated beacon for development)
//   - Successfully mining/producing new blocks
//   - Growing the blockchain beyond the genesis block (block 0)
//
// We check if block number reaches at least 3, confirming:
//   - Genesis block (0) exists
//   - Node produced block 1, 2, 3... (active block production)
//
// If this fails, the blockchain is "frozen" - no transactions can be processed.
// Common causes: consensus not started, mining disabled, or configuration issues.
func TestBlockProduction(t *testing.T) {
	ctx, cancel, manager := startNodes(t, 1)
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

// TestBlockProductionMonitoring verifies continuous block production by checking advancement over time.
//
// This test captures the "heartbeat" of the blockchain - ensuring blocks keep coming.
// Unlike TestBlockProduction which just checks if we got past genesis, this monitors
// ongoing block production to ensure the chain stays alive.
//
// The test:
//  1. Records current block number (snapshot 1)
//  2. Waits a period of time
//  3. Checks block number again (snapshot 2)
//  4. Verifies snapshot 2 > snapshot 1
//
// This detects "stalled chain" scenarios where:
//   - Initial blocks were produced but then stopped
//   - Consensus mechanism halted after startup
//   - Network issues preventing block propagation
//
// In production, stalled chains mean no transactions can be confirmed,
// making the entire network unusable even if nodes appear "healthy".
func TestBlockProductionMonitoring(t *testing.T) {
	ctx, cancel, manager := startNodes(t, 1)
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

// TestPostMergeBlockStructureValidation verifies blocks follow Proof-of-Stake structure after The Merge.
//
// Historical context: Ethereum transitioned from Proof-of-Work (PoW) to Proof-of-Stake (PoS)
// in September 2022, called "The Merge". This changed how blocks are created:
//   - Before: Miners solved computational puzzles (high "difficulty")
//   - After: Validators are chosen based on staked ETH (difficulty = 0)
//
// This test validates our blocks have proper PoS structure:
//  1. difficulty == 0x0 (no mining puzzles in PoS)
//  2. mixHash exists and changes (randomness beacon for validator selection)
//  3. totalDifficulty matches expected values (cumulative from genesis)
//
// Why this matters:
//   - Ensures we're running post-merge Ethereum (not outdated pre-merge)
//   - Validates consensus layer integration is working
//   - Confirms blocks contain required randomness for security
//
// If this fails, the node may be:
//   - Running pre-merge configuration (critical misconfiguration)
//   - Missing consensus layer connection
//   - Producing invalid blocks that peers would reject
func TestPostMergeBlockStructureValidation(t *testing.T) {
	ctx, cancel, manager := startNodes(t, 1)
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

// TestSimpleETHTransfer validates the complete lifecycle of sending cryptocurrency between accounts.
//
// Core concept: Ethereum transactions transfer value (ETH) or data between accounts.
// Every transaction must be signed with the sender's private key (proving ownership).
//
// This test performs a real ETH transfer:
//  1. Creates two accounts: A (funded with 1 ETH) and B (empty)
//  2. A sends 0.1 ETH to B
//  3. Verifies the transaction is included in a block
//  4. Confirms balances updated correctly (including gas fees)
//
// Transaction components tested:
//   - Nonce: Prevents replay attacks (each transaction has unique number)
//   - Gas: Fee paid to validators for processing (like postage for mail)
//   - Value: Amount of ETH being sent
//   - Signature: Cryptographic proof the owner authorized this
//
// This is THE fundamental operation of any blockchain - transferring value.
// If this fails, the blockchain cannot fulfill its primary purpose as a
// decentralized ledger for value transfer.
func TestSimpleETHTransfer(t *testing.T) {
	// Creates two accounts with 1 ETH each, sends a transaction from one to the other,
	// Accounts A and B.
	aKey := testutils.PrivateKeyFixture(t)
	aAddr := crypto.PubkeyToAddress(aKey.PublicKey)

	bKey := testutils.PrivateKeyFixture(t)
	bAddr := crypto.PubkeyToAddress(bKey.PublicKey)

	oneEth := new(big.Int).Mul(big.NewInt(1), big.NewInt(params.Ether))

	ctx, cancel, manager := startNodes(t, 1, node.WithPreFundGenesisAccount(aAddr, oneEth))
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

// TestRevertingTransaction verifies that failed transactions are handled correctly by the network.
//
// Not all transactions succeed - some fail by design (like requiring conditions not met).
// The blockchain must handle failures gracefully while still charging for the attempt.
//
// This test deploys a "always-fail" smart contract:
//   - Contract code: PUSH 0, PUSH 0, REVERT (always reverts/fails)
//   - Transaction gets included in a block (miners/validators still process it)
//   - Status shows failure (status = 0x0)
//   - Gas is still consumed (computational work was done)
//
// Why failed transactions still cost gas:
//   - Prevents denial-of-service attacks (can't spam free failing transactions)
//   - Validators did work checking the transaction, deserve compensation
//   - Similar to non-refundable processing fees in traditional systems
//
// This ensures the network remains economically secure even when processing
// invalid operations, and that applications can detect and handle failures.
func TestRevertingTransaction(t *testing.T) {
	key := testutils.PrivateKeyFixture(t)
	addr := crypto.PubkeyToAddress(key.PublicKey)

	oneEth := new(big.Int).Mul(big.NewInt(1), big.NewInt(params.Ether))

	ctx, cancel, manager := startNodes(t, 1, node.WithPreFundGenesisAccount(addr, oneEth))
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

// TestContractDeploymentAndInteraction tests the full smart contract lifecycle on the blockchain.
//
// Smart contracts are programs that run ON the blockchain itself - like apps on a phone.
// Once deployed, they live forever at an address and anyone can interact with them.
//
// This comprehensive test validates:
//  1. DEPLOYMENT: Uploading contract code to blockchain (like installing an app)
//  2. VERIFICATION: Contract exists at predicted address with correct bytecode
//  3. READ: Calling view functions without transactions (free queries)
//  4. WRITE: Sending transactions to modify contract state (costs gas)
//  5. EVENTS: Contract emits logs that applications can listen to
//
// We use a SimpleStorageContract that:
//   - Stores a single number
//   - Has a "set" function to change it
//   - Has a "value" function to read it
//   - Emits "ValueChanged" event when updated
//
// Smart contracts power:
//   - DeFi (decentralized finance) protocols
//   - NFTs and digital ownership
//   - DAOs (decentralized organizations)
//   - Any logic that needs to run trustlessly
//
// If this test fails, the platform cannot support any decentralized applications,
// reducing the blockchain to just a simple payment network.
func TestContractDeploymentAndInteraction(t *testing.T) {
	key := testutils.PrivateKeyFixture(t)
	addr := crypto.PubkeyToAddress(key.PublicKey)

	oneEth := new(big.Int).Mul(big.NewInt(1), big.NewInt(params.Ether))

	ctx, cancel, manager := startNodes(t, 1, node.WithPreFundGenesisAccount(addr, oneEth))
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
	require.NoError(
		t,
		client.CallContext(ctx, &txHash, model.EthSendRawTransaction, utils.ByteToHex(txBytes)),
	)

	// Eventually the transaction should be included in a block and a receipt should be available.
	var receipt map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(
				ctx,
				&receipt,
				model.EthGetTransactionReceipt,
				txHash,
			); err != nil {
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
	require.NoError(
		t,
		client.CallContext(
			ctx,
			&code,
			model.ReceiptGetByteCode,
			contractAddr.Hex(),
			model.EthBlockLatest,
		),
	)
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
				model.CallContextTo:   contractAddr.Hex(),
				model.CallContextData: utils.ByteToHex(callData),
			}, model.EthBlockLatest,
		),
	)
	val := testutils.HexToBigInt(t, valHex)
	// The initial value should be 0 since the contract is just deployed.
	require.Zero(t, val.Int64())

	// Now we will set a new value (e.g., 7) using the `set` function of the contract.
	// First, we need to get the nonce for the next transaction.
	var nonceHex2 string
	require.NoError(
		t,
		client.CallContext(
			ctx,
			&nonceHex2,
			model.EthGetTransactionCount,
			addr.Hex(),
			model.EthBlockLatest,
		),
	)
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
	require.NoError(
		t,
		client.CallContext(ctx, &txHash2, model.EthSendRawTransaction, utils.ByteToHex(txBytes2)),
	)

	// Eventually the transaction should be included in a block and a receipt should be available.
	var receipt2 map[string]interface{}
	require.Eventually(
		t, func() bool {
			if err := client.CallContext(
				ctx,
				&receipt2,
				model.EthGetTransactionReceipt,
				txHash2,
			); err != nil {
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
			map[string]string{
				model.CallContextTo:   contractAddr.Hex(),
				model.CallContextData: utils.ByteToHex(callData),
			},
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

// TestPeerConnectivity_TwoNodes verifies basic peer-to-peer network formation between two nodes.
//
// Blockchains are peer-to-peer networks - nodes must connect to share blocks and transactions.
// Without peer connections, each node is an isolated island, unable to participate in consensus.
//
// This test creates a minimal network:
//  1. Starts two independent nodes
//  2. Tells node1 about node2's address (like sharing a phone number)
//  3. Verifies they establish connection
//  4. Checks both report having at least one peer
//
// The connection uses "enode" URLs - unique addresses that identify nodes:
//
//	enode://[public-key]@[ip]:[port]
//	Like: enode://abc123...@127.0.0.1:30303
//
// Why peer connectivity matters:
//   - Blocks must propagate to all nodes for consensus
//   - Transactions spread through peer gossip
//   - Isolated nodes can't participate in the network
//   - Network partition would create competing chains
//
// This is the foundation of decentralization - nodes discovering and maintaining
// connections without central coordination.
func TestPeerConnectivity_TwoNodes(t *testing.T) {
	ctx, cancel, manager := startNodes(t, 2)
	defer cancel()

	require.Equal(t, 2, manager.NodeCount())

	node1 := manager.GetNode(0)
	node2 := manager.GetNode(1)
	require.NotNil(t, node1)
	require.NotNil(t, node2)

	node2Enode := node2.Server().NodeInfo().Enode

	client1, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client1.Close()

	client2, err := rpc.DialContext(ctx, utils.LocalAddress(manager.GetRPCPort(1)))
	require.NoError(t, err)
	defer client2.Close()

	// ensure node1 knows about node2
	require.NoError(t, client1.CallContext(ctx, nil, "admin_addPeer", node2Enode))

	// wait for peer connections
	require.Eventually(
		t, func() bool {
			var count string
			if err := client1.CallContext(ctx, &count, model.NetPeerCount); err != nil {
				return false
			}
			return count != "0x0"
		}, 10*time.Second, 500*time.Millisecond, "peers did not connect",
	)
}

// TestPeerConnectivity_FiveNodes validates network formation and mesh topology with multiple nodes.
//
// Real blockchain networks have thousands of nodes forming complex mesh topologies.
// This test simulates a small network to verify our implementation scales beyond simple pairs.
//
// Network topology with 5 nodes:
//   - Node 0: The miner/block producer (all others connect to it)
//   - Nodes 1-4: Non-mining nodes that sync blocks
//   - Each node connects to multiple peers for redundancy
//   - Forms a mesh where information can flow multiple paths
//
// The test validates:
//  1. All 5 nodes start successfully
//  2. Cross-connections are established (not just star topology)
//  3. Each node reports having peers
//  4. The miner node is well-connected (critical for block propagation)
//
// Why multi-node testing matters:
//   - Consensus bugs often appear only with 3+ nodes
//   - Network partitions become possible with multiple nodes
//   - Tests gossip protocol efficiency
//   - Validates that blocks reach all nodes despite complex paths
//   - Ensures no node becomes isolated as network grows
//
// This represents the minimum viable production network - enough nodes for
// resilience but small enough to test efficiently.
func TestPeerConnectivity_FiveNodes(t *testing.T) {
	nodeCount := 5
	ctx, cancel, manager := startNodes(t, 5)
	defer cancel()

	require.Equal(t, nodeCount, manager.NodeCount())

	// Verify all nodes are created
	nodes := make([]*rpc.Client, nodeCount)
	for i := 0; i < 5; i++ {
		require.NotNil(t, manager.GetNode(i), "node %d should not be nil", i)

		client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.GetRPCPort(i)))
		require.NoError(t, err)
		defer client.Close()
		nodes[i] = client
	}

	// Ensure all peer nodes (1-4) know about each other and the miner (node 0)
	// Since nodes 1-4 already have node 0 as a static peer, we need to add cross-connections
	for i := 1; i < nodeCount; i++ {
		nodeI := manager.GetNode(i)
		enodeI := nodeI.Server().NodeInfo().Enode

		// Add this node to all other nodes' peer lists
		for j := 0; j < 5; j++ {
			if i != j {
				require.NoError(t, nodes[j].CallContext(ctx, nil, "admin_addPeer", enodeI))
			}
		}
	}

	// Wait for peer connections to establish
	// Each node should eventually see at least one peer (in practice, should see all 4 others)
	for i := 0; i < nodeCount; i++ {
		nodeIndex := i
		require.Eventually(
			t, func() bool {
				var count string
				if err := nodes[nodeIndex].CallContext(
					ctx,
					&count,
					model.NetPeerCount,
				); err != nil {
					return false
				}
				return count != "0x0"
			}, 15*time.Second, 500*time.Millisecond, "node %d did not connect to peers", i,
		)
	}

	// Verify that the miner node (node 0) has the most connections
	// since all other nodes connect to it as their bootstrap node
	var minerPeerCount string
	require.NoError(t, nodes[0].CallContext(ctx, &minerPeerCount, model.NetPeerCount))

	// Convert hex peer count to int for validation
	peerCount := testutils.HexToBigInt(t, minerPeerCount)
	require.Greater(t, peerCount.Int64(), int64(0), "miner should have at least one peer")

	// Optional: Verify that we can get some peer count from each node
	for i := 1; i < nodeCount; i++ {
		var peerCountHex string
		require.NoError(t, nodes[i].CallContext(ctx, &peerCountHex, model.NetPeerCount))
		peerCountInt := testutils.HexToBigInt(t, peerCountHex)
		require.GreaterOrEqual(
			t,
			peerCountInt.Int64(),
			int64(1),
			"node %d should have at least one peer",
			i,
		)
	}
}

// TestSyncStatus verifies that the node reports it is NOT syncing via eth_syncing RPC call.
//
// IMPORTANT: This test is NOT about peer communication (nodes do share blocks with each other).
// Instead, eth_syncing tells us if a node is downloading historical blocks to catch up.
//
// In our local dev network, syncing should NEVER happen because:
//  1. All nodes start from the same genesis block at time T=0
//  2. All nodes witness block production in real-time as it happens
//  3. No node joins late or falls behind needing to "catch up"
//
// Think of it like a live sports game:
//   - Peer communication = commentators describing plays as they happen (normal)
//   - Block synchronization = watching a recording to catch up on missed quarters (shouldn't happen here)
//
// If eth_syncing returns true in our controlled environment, it signals a CRITICAL FAILURE:
//   - The node believes it's missing blocks (but from where? it was here the whole time!)
//   - Possible consensus layer/execution layer desynchronization
//   - Network partition causing the node to miss blocks it should have seen
//   - The node may be on a minority fork while thinking it needs to catch up
//
// This is different from production networks where nodes routinely sync when they:
//   - First join an existing network (need blocks 0 to current)
//   - Reconnect after being offline (need blocks missed during downtime)
//   - Fall behind due to slow processing (need recent blocks)
//
// In our case, a syncing node in a network it helped create from genesis indicates something
// is fundamentally broken with consensus or network communication.
func TestSyncStatus(t *testing.T) {
	ctx, cancel, manager := startNodes(t, 1)
	defer cancel()

	client, err := rpc.DialContext(ctx, utils.LocalAddress(manager.RPCPort()))
	require.NoError(t, err)
	defer client.Close()

	// eth_syncing returns either false (not syncing) or an object with sync progress.
	// For a local dev node that's up-to-date, we expect false or minimal sync activity.
	var syncStatus interface{}
	require.NoError(t, client.CallContext(ctx, &syncStatus, model.EthSyncing))

	// The response can be either:
	// - false (boolean) when not syncing
	// - nil (when not syncing in some implementations)
	// - an object with sync progress when syncing

	// If it's nil or false, the node is not syncing
	if syncStatus == nil {
		// Node is not syncing (nil response)
		return
	}

	// Check if it's a boolean false
	if syncBool, isBool := syncStatus.(bool); isBool {
		require.False(t, syncBool, "node should not be syncing (should return false)")
		return
	}

	// If it's an object, check if it's actually syncing blocks or just indexing
	if syncMap, isMap := syncStatus.(map[string]interface{}); isMap {
		// Extract currentBlock and highestBlock to determine if the node is actively syncing blocks
		currentBlockHex, hasCurrentBlock := syncMap["currentBlock"].(string)
		highestBlockHex, hasHighestBlock := syncMap["highestBlock"].(string)

		// If both fields exist and are equal, the node is caught up (not actively syncing blocks)
		// This may still show sync status due to transaction indexing, which is acceptable
		if hasCurrentBlock && hasHighestBlock {
			currentBlock := testutils.HexToBigInt(t, currentBlockHex)
			highestBlock := testutils.HexToBigInt(t, highestBlockHex)

			// If currentBlock == highestBlock, the node is caught up with the chain
			// Transaction indexing may still be ongoing, but that's not a sync issue
			if currentBlock.Cmp(highestBlock) == 0 {
				// Node is caught up with the chain, not actively syncing blocks
				return
			}

			// If currentBlock < highestBlock, the node is actively syncing
			t.Fatalf(
				"node is actively syncing blocks (current: %s, highest: %s)",
				currentBlock,
				highestBlock,
			)
		}
	}

	// If we can't determine the sync status properly, fail with the raw response
	t.Fatalf("unexpected sync status response: %+v", syncStatus)
}
