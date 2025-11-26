package prysm

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
)

// GenerateGenesisState creates a beacon chain genesis state from configuration.
//
// GenerateGenesisState performs the following steps:
// 1. Converts validator keys to Prysm deposits
// 2. Creates the beacon state with configured validators
// 3. Sets up the execution payload header linking to Geth genesis block
// 4. Calculates the genesis state root
//
// The returned genesis state can be used to initialize a Prysm beacon node.
// Returns an error if the configuration is invalid or genesis creation fails.
func GenerateGenesisState(
	cfg consensus.Config,
	withdrawalAddress common.Address,
	gethGenesisHash common.Hash,
	gethGenesisNumber uint64,
	gethGenesisTimestamp uint64,
) ([]byte, error) {
	if cfg.ChainID == 0 {
		return nil, fmt.Errorf("chain id is required")
	}
	if cfg.GenesisTime.IsZero() {
		return nil, fmt.Errorf("genesis time is required")
	}
	if len(cfg.ValidatorKeys) == 0 {
		return nil, fmt.Errorf("at least one validator is required")
	}

	// TODO(#47): Implement genesis state generation using Prysm v5 API
	// https://github.com/thep2p/go-eth-localnet/issues/47
	// This will involve:
	// 1. Converting ValidatorKeys []string to []*ethpb.Deposit using withdrawalAddress
	// 2. Creating a beacon state with the configured validators
	// 3. Converting gethGenesis* params to *enginev1.ExecutionPayloadHeader
	// 4. Calculating the state root
	// 5. Marshaling the state to SSZ format for Prysm

	return nil, fmt.Errorf("genesis state generation not yet implemented")
}

// DeriveGenesisRoot calculates the genesis beacon state root.
//
// The genesis root is the hash tree root of the beacon chain genesis state.
// It's used as a unique identifier for the network and must match between
// all nodes in the network.
func DeriveGenesisRoot(genesisState []byte) ([32]byte, error) {
	if len(genesisState) == 0 {
		return [32]byte{}, fmt.Errorf("genesis state is empty")
	}

	// TODO(#47): Implement genesis root calculation
	// https://github.com/thep2p/go-eth-localnet/issues/47
	// This will compute the SSZ hash tree root of the beacon state

	return [32]byte{}, fmt.Errorf("genesis root calculation not yet implemented")
}

// DefaultGenesisTime returns a genesis time suitable for local development.
//
// For local networks, we typically start the beacon chain immediately
// or with a small delay to allow for setup. This returns a time 30 seconds
// in the past to ensure the network starts producing blocks immediately.
func DefaultGenesisTime() time.Time {
	return time.Now().Add(-30 * time.Second)
}

// GenerateTestValidators creates validator keys for testing.
//
// This is a convenience function for local development that generates
// the specified number of BLS validator keys deterministically as hex strings.
// These keys can be used with consensus.Config.ValidatorKeys.
//
// WARNING: The generated keys are deterministic and MUST NOT be used
// in production. They are suitable only for local testing.
func GenerateTestValidators(count int) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf("validator count must be positive")
	}
	if count > 1000 {
		return nil, fmt.Errorf("validator count too large: %d (max 1000)", count)
	}

	keys := make([]string, count)
	for i := 0; i < count; i++ {
		// TODO(#47): Implement deterministic BLS12-381 key generation for testing
		// https://github.com/thep2p/go-eth-localnet/issues/47
		// This will generate proper BLS keys from a deterministic seed
		keys[i] = fmt.Sprintf("test-validator-key-%d", i)
	}

	return keys, nil
}

// MinGenesisActiveValidatorCount returns the minimum number of validators
// needed to start a beacon chain.
//
// For Ethereum mainnet, this is 16384 validators. For local development,
// we can use a much smaller number (typically 1-4 validators).
func MinGenesisActiveValidatorCount() int {
	// For local development, we only need 1 validator
	// (mainnet requires 16384)
	return 1
}

// GenesisDelay returns the time to wait after genesis before producing blocks.
//
// This gives nodes time to initialize and connect to peers. For local
// development, this can be very short or zero.
func GenesisDelay() time.Duration {
	// For local development, start producing blocks immediately
	return 0
}
