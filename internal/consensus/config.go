package consensus

import (
	"crypto/ecdsa"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Config holds configuration for the Prysm consensus client.
//
// Config defines all parameters needed to launch and run Prysm,
// including network settings, ports, and validator configuration.
type Config struct {
	// DataDir is the directory for Prysm client data.
	// Prysm stores beacon chain data, validator keys, and other persistent state here.
	DataDir string `validate:"required"`

	// Network configuration

	// ChainID identifies the Ethereum network (1337 for local development).
	ChainID uint64

	// GenesisTime is the Unix timestamp when the beacon chain genesis occurred.
	GenesisTime time.Time

	// GenesisRoot is the hash tree root of the genesis beacon state.
	GenesisRoot common.Hash

	// Ports

	// BeaconPort is the port for the Beacon API (typically 4000).
	BeaconPort int `validate:"required,gt=0"`

	// P2PPort is the port for P2P networking (typically 9000).
	P2PPort int `validate:"required,gt=0"`

	// RPCPort is the port for gRPC or other RPC services (client-specific).
	RPCPort int

	// Connection to Execution Layer

	// EngineEndpoint is the Engine API endpoint of the paired EL node.
	// Format: http://host:port
	EngineEndpoint string `validate:"required"`

	// JWTSecret is the JWT secret for Engine API authentication.
	// Must match the secret used by the paired EL node.
	JWTSecret []byte `validate:"required,min=1"`

	// P2P configuration

	// Bootnodes are ENR addresses of bootstrap nodes for peer discovery.
	Bootnodes []string

	// StaticPeers are static peer connections that should always be maintained.
	StaticPeers []string

	// PrivateKey is the node's P2P identity key.
	// If nil, a new key will be generated.
	PrivateKey *ecdsa.PrivateKey

	// Validator configuration

	// ValidatorKeys are validator private keys for block production (testing only).
	// In production, keys should be managed securely via remote signers.
	ValidatorKeys []string

	// WithdrawalAddresses are ethereum execution layer addresses where each
	// validator's rewards and withdrawn stake will be sent. Must have one
	// address per validator key.
	WithdrawalAddresses []common.Address

	// FeeRecipient is the Ethereum address that receives transaction fees
	// from blocks proposed by this validator.
	FeeRecipient common.Address

	// Optional: Checkpoint sync

	// CheckpointSyncURL is a trusted source for checkpoint sync data.
	// Checkpoint sync allows rapid syncing from a recent finalized checkpoint.
	CheckpointSyncURL string

	// GenesisStateURL is a URL to fetch the genesis beacon state.
	// Used for bootstrapping new clients.
	GenesisStateURL string
}
