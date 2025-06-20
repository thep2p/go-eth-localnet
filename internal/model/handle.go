package model

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// Config defines the configuration parameters for a Geth node instance.
type Config struct {
	// ID represents the node identifier for sake of tracking and labeling.
	// It can be any arbitrary integer, typically starting from 0.
	// Uniqueness of IDs is not enforced, but it is recommended to use unique IDs for each node instance.
	ID enode.ID
	// DataDir is the directory where the node's data will be stored.
	DataDir string
	// P2PPort is the port used for peer-to-peer communication.
	P2PPort int
	// RPCPort defines the port for remote procedure calls.
	RPCPort int
	// PrivateKey is the private key used for signing transactions and messages.
	PrivateKey *ecdsa.PrivateKey

	StaticNodes []string // enode URLs of peers

	// Mine determines whether this node should produce blocks using the
	// simulated beacon. Only one node in the network may enable mining.
	Mine bool
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
func (h *Handle) ID() enode.ID {
	return h.config.ID
}

// RpcPort returns the port configured for remote procedure calls (RPC) for the Geth node instance.
// This port is used for interacting with the node via JSON-RPC or other RPC protocols.
func (h *Handle) RpcPort() int {
	return h.config.RPCPort
}

func (h *Handle) Server() *p2p.Server {
	return h.instance.Server()
}

// Mining returns true if the node was configured to produce blocks.
func (h *Handle) Mining() bool {
	return h.config.Mine
}
