package model

// Constants in this file are used as keys to represent various data points related to Ethereum
// transactions, receipts, and blocks. They also define RPC method calls for interacting
// with an Ethereum node API.

const (
	// Receipt-related constants
	// ReceiptGasUsed represents the gas used in a transaction receipt.
	ReceiptGasUsed = "gasUsed"

	// ReceiptEffectiveGasPrice represents the effective gas price in a transaction receipt.
	ReceiptEffectiveGasPrice = "effectiveGasPrice"

	// ReceiptBlockNumber represents the block number in a transaction receipt.
	ReceiptBlockNumber = "blockNumber"

	// ReceiptStatus represents the status in a transaction receipt.
	ReceiptStatus = "status"

	// Block-related constants
	// BlockMixHash represents the mix hash field in a blockchain block.
	BlockMixHash = "mixHash"

	// BlockTotalDifficulty represents the total difficulty field in a blockchain block.
	BlockTotalDifficulty = "totalDifficulty"

	// BlockDifficulty represents the difficulty field in a blockchain block.
	BlockDifficulty = "difficulty"

	// Ethereum-related method constants
	// EthLatestBlock represents the latest block identifier in Ethereum.
	EthBlockLatest = "latest"

	// EthBlockNumber represents the method for retrieving the current block number.
	EthBlockNumber = "eth_blockNumber"

	// EthGetBlockByNumber represents the method for retrieving a block by its number.
	EthGetBlockByNumber = "eth_getBlockByNumber"

	// EthGetTransactionCount represents the method for retrieving transaction count for an address.
	EthGetTransactionCount = "eth_getTransactionCount"

	// EthGetTransactionReceipt represents the method for retrieving a transaction receipt.
	EthGetTransactionReceipt = "eth_getTransactionReceipt"

	// EthSendRawTransaction represents the method for sending a raw transaction to the network.
	EthSendRawTransaction = "eth_sendRawTransaction"

	// EthWeb3ClientVersion represents the method for retrieving the client version.
	EthWeb3ClientVersion = "web3_clientVersion"
)
