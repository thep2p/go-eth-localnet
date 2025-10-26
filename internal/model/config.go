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

	// EnginePort is the port for Engine API (authenticated RPC).
	// This port is used for EL-CL communication with JWT authentication.
	EnginePort int
	// JWTSecretPath is the path to JWT secret file for Engine API auth.
	// The file contains a 32-byte hex-encoded secret used for JWT signing.
	JWTSecretPath string
	// EnableEngineAPI enables the Engine API endpoints for CL communication.
	// When enabled, the node will accept authenticated Engine API requests.
	EnableEngineAPI bool
}
