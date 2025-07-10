package model

const (
	// ReceiptGasUsed represents the gas used in a transaction receipt.
	ReceiptGasUsed = "gasUsed"

	// ReceiptEffectiveGasPrice represents the effective gas price in a transaction receipt.
	ReceiptEffectiveGasPrice = "effectiveGasPrice"

	// ReceiptBlockNumber represents the block number in a transaction receipt.
	ReceiptBlockNumber = "blockNumber"

	// ReceiptTxStatusSuccess represents a successful transaction status with a value of "0x1".
	ReceiptTxStatusSuccess = "0x1"

	// ReceiptTxStatusFailure represents a failed transaction status with a value of "0x0".
	ReceiptTxStatusFailure = "0x0"

	// ReceiptGetByteCode represents the keyword to retrieve the byte code of a smart contract.
	ReceiptGetByteCode = "eth_getCode"

	// CallContextEthCall represents the "eth_call" method used for executing a call on the Ethereum network without submitting a transaction.
	CallContextEthCall = "eth_call"

	// CallContextTo represents the "to" field in the call context, typically specifying the recipient address of the call.
	CallContextTo = "to"

	// CallContextData is a constant representing the "data" field in Ethereum call context requests.
	CallContextData = "data"

	// ReceiptContractAddress represents the key for accessing the contract address from a transaction receipt.
	ReceiptContractAddress = "contractAddress"

	// ReceiptStatus represents the status in a transaction receipt.
	ReceiptStatus = "status"

	// ReceiptLogs represents the logs field in a transaction receipt.
	ReceiptLogs = "logs"

	// ReceiptLogTopics represents the topics field in a transaction receipt log.
	ReceiptLogTopics = "topics"

	// ReceiptLogData represents the data field in a transaction receipt log.
	ReceiptLogData = "data"

	// BlockMixHash represents the mix hash field in a blockchain block.
	BlockMixHash = "mixHash"

	// BlockTotalDifficulty represents the total difficulty field in a blockchain block.
	BlockTotalDifficulty = "totalDifficulty"

	// BlockDifficulty represents the difficulty field in a blockchain block.
	BlockDifficulty = "difficulty"

	// EthLatestBlock represents the latest block identifier in Ethereum.
	EthLatestBlock = "latest"

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

	// AccountEmptyContract represents the default value for an empty contract address "0x".
	// This is used to indicate that no contract is deployed at the specified address.
	AccountEmptyContract = "0x"
)
