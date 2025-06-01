package node

import "github.com/ethereum/go-ethereum/node"

// Config defines the configuration parameters for a Geth node instance.
type Config struct {
	// ID represents the node identifier.
	ID int
	// DataDir is the directory where the node's data will be stored.
	DataDir string
	// P2PPort is the port used for peer-to-peer communication.
	P2PPort int
	// RPCPort defines the port for remote procedure calls.
	RPCPort int
}

// Handle represents a running Geth node instance.
type Handle struct {
	// Instance is a reference to the active Geth node instance.
	Instance *node.Node
	// EnodeURL is the enode URL of the Geth node, used for peer discovery.
	EnodeURL string
	// Config holds the configuration details for the Geth node.
	Config Config
}
