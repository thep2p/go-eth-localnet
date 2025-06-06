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
	// instance is a reference to the active Geth node instance.
	instance *node.Node
	// nodeURL is the enode URL of the Geth node, used for peer discovery.
	nodeURL string
	// config holds the configuration details for the Geth node.
	config Config
}

// NewHandle initializes and returns a new Handle for a Geth node instance with the provided configuration.
func NewHandle(instance *node.Node, enodeURL string, config Config) *Handle {
	return &Handle{
		instance: instance,
		nodeURL:  enodeURL,
		config:   config,
	}
}

// Close stops the Geth node instance and releases resources.
func (h *Handle) Close() error {
	return h.instance.Close()
}

// NodeURL returns the enode URL of the Geth node, which is used for peer discovery.
func (h *Handle) NodeURL() string {
	return h.nodeURL
}

// DataDir returns the directory where the Geth node's data is stored.
func (h *Handle) DataDir() string {
	return h.config.DataDir
}

// ID returns the identifier of the Geth node instance.
func (h *Handle) ID() int {
	return h.config.ID
}
