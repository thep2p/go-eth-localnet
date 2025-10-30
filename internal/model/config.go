package model

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// Config defines the configuration parameters for a Geth node instance.
type Config struct {
	// ID represents the node identifier for sake of tracking and labeling.
	// ID is the node’s enode identifier derived from its public key.
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

	// Engine API configuration for EL-CL communication.

	// EnableEngineAPI determines whether to expose the authenticated Engine API.
	// When true, EnginePort and JWTSecretPath must be configured.
	EnableEngineAPI bool

	// EnginePort is the port for authenticated Engine API RPC.
	// Only used when EnableEngineAPI is true.
	EnginePort int

	// JWTSecretPath is the filesystem path to the JWT secret file.
	// The file should contain a 32-byte hex-encoded secret.
	// Only used when EnableEngineAPI is true.
	JWTSecretPath string
}
