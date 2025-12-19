package prysm

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/signing"
	state_native "github.com/prysmaticlabs/prysm/v5/beacon-chain/state/state-native"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/crypto/bls"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/prysmaticlabs/prysm/v5/runtime/interop"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
)

// GenerateGenesisState creates a beacon chain genesis state from configuration.
//
// Converts validator keys to Prysm deposits and generates an SSZ-encoded genesis
// state. Each validator gets independent withdrawal credentials from cfg.WithdrawalAddresses.
//
// Returns the SSZ-encoded genesis state or an error. All errors are critical and
// indicate genesis state generation cannot proceed.
func GenerateGenesisState(cfg consensus.Config) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create deposit data with per-validator withdrawal addresses
	depositDataItems, depositDataRoots, err := createDepositDataWithWithdrawalAddresses(cfg.ValidatorKeys, cfg.WithdrawalAddresses)
	if err != nil {
		return nil, fmt.Errorf("create deposit data: %w", err)
	}

	// Generate genesis state using interop helpers
	// Needs both depositDataItems (to populate validator registry) and depositDataRoots
	// (to build the deposit merkle tree in beacon state's eth1_data field).
	// The roots become leaves in the deposit tree that tracks all validator deposits.
	genesisTime := uint64(cfg.GenesisTime.Unix())
	protoState, _, err := interop.GenerateGenesisStateFromDepositData(context.Background(), genesisTime, depositDataItems, depositDataRoots)
	if err != nil {
		return nil, fmt.Errorf("generate genesis state: %w", err)
	}

	// Wrap proto state in beacon state interface
	st, err := state_native.InitializeFromProtoPhase0(protoState)
	if err != nil {
		return nil, fmt.Errorf("initialize state: %w", err)
	}

	// Marshal state to SSZ format
	sszBytes, err := st.MarshalSSZ()
	if err != nil {
		return nil, fmt.Errorf("marshal state to ssz: %w", err)
	}

	return sszBytes, nil
}

// DeriveGenesisRoot calculates the genesis beacon state root from SSZ-encoded state.
//
// Returns the hash tree root used as the network identifier. All errors are critical
// and indicate the genesis state is invalid or corrupted.
func DeriveGenesisRoot(genesisState []byte) (common.Hash, error) {
	if len(genesisState) == 0 {
		return common.Hash{}, fmt.Errorf("genesis state is empty")
	}

	// Unmarshal SSZ into proto message (Phase0 for now)
	protoState := &ethpb.BeaconState{}
	if err := protoState.UnmarshalSSZ(genesisState); err != nil {
		return common.Hash{}, fmt.Errorf("unmarshal ssz: %w", err)
	}

	// Wrap in beacon state interface
	st, err := state_native.InitializeFromProtoPhase0(protoState)
	if err != nil {
		return common.Hash{}, fmt.Errorf("initialize state: %w", err)
	}

	// Compute hash tree root
	root, err := st.HashTreeRoot(context.Background())
	if err != nil {
		return common.Hash{}, fmt.Errorf("compute hash tree root: %w", err)
	}

	return common.BytesToHash(root[:]), nil
}

// createDepositDataWithWithdrawalAddresses creates deposit data for validators.
//
// Deposit data is a cryptographically signed structure proving validator ownership.
// It contains: validator public key (BLS12-381), withdrawal credentials, stake amount
// (32 ETH), and BLS signature over these fields.
//
// Withdrawal address is a standard 20-byte Ethereum account address where validator
// rewards and stake are sent when the validator exits or receives withdrawals.
//
// The beacon state maintains a deposit merkle tree where each validator deposit is
// one leaf. During genesis, this function computes the hash tree root of each deposit
// data structure (which itself is a tree), which becomes a leaf in the beacon state's deposit tree (stored in
// the eth1_data field).
//
// Returns deposit data items and their hash tree roots. All errors are critical
// and indicate cryptographic operations or signing failed.
func createDepositDataWithWithdrawalAddresses(
	secretKeys []bls.SecretKey,
	withdrawalAddresses []common.Address,
) ([]*ethpb.Deposit_Data, [][]byte, error) {
	depositDataItems := make([]*ethpb.Deposit_Data, len(secretKeys))
	depositDataRoots := make([][]byte, len(secretKeys))

	for i, secretKey := range secretKeys {
		// Derive public key from secret key
		publicKey := secretKey.PublicKey()

		// Create withdrawal credentials (32 bytes for SSZ merkleization)
		// Beacon chain uses 32-byte fields for all hash-sized data to maintain
		// uniform merkle tree chunks. Withdrawal credentials structure:
		//
		//   [0x01][11 zero bytes][20-byte Ethereum address]
		//    ^^^^^               ^^^^^^^^^^^^^^^^^^^^^^^
		//    type                  your account address
		//
		// Type 0x01 = direct withdrawal to Ethereum address (modern standard)
		// Type 0x00 = BLS withdrawal credentials (legacy, pre-Shanghai)
		withdrawalCreds := make([]byte, 32)
		withdrawalCreds[0] = params.BeaconConfig().ETH1AddressWithdrawalPrefixByte // 0x01
		// Copy 20-byte Ethereum address into bytes 12-31 (bytes 1-11 remain zero padding)
		copy(withdrawalCreds[12:], withdrawalAddresses[i].Bytes())

		// Create deposit message (unsigned data to be staked)
		// MaxEffectiveBalance = 32 ETH, the standard validator stake
		depositMsg := &ethpb.DepositMessage{
			PublicKey:             publicKey.Marshal(),
			WithdrawalCredentials: withdrawalCreds,
			Amount:                params.BeaconConfig().MaxEffectiveBalance, // 32 ETH
		}

		// Sign the deposit message to prove ownership of the validator key
		// Domain separates signature purposes (deposit vs attestation vs block proposal)
		domain, err := signing.ComputeDomain(
			params.BeaconConfig().DomainDeposit,
			params.BeaconConfig().GenesisForkVersion,
			params.BeaconConfig().ZeroHash[:],
		)
		if err != nil {
			return nil, nil, fmt.Errorf("compute domain: %w", err)
		}

		// Compute signing root: hash(depositMsg + domain) for signature
		signingRoot, err := signing.ComputeSigningRoot(depositMsg, domain)
		if err != nil {
			return nil, nil, fmt.Errorf("compute signing root: %w", err)
		}

		// Sign with validator's BLS secret key to prove ownership
		signature := secretKey.Sign(signingRoot[:])

		// Create final deposit data (unsigned message + signature)
		// This proves: "I own this validator key and authorize this configuration"
		depositData := &ethpb.Deposit_Data{
			PublicKey:             depositMsg.PublicKey,
			WithdrawalCredentials: depositMsg.WithdrawalCredentials,
			Amount:                depositMsg.Amount,
			Signature:             signature.Marshal(),
		}

		// Compute deposit data root - two-level merkle tree structure:
		//
		// Level 1: Individual deposit data (this computation)
		//     [Hash Tree Root] ← HashTreeRoot() returns this
		//     /              \
		//   [hash(pubkey,   [hash(amount,
		//    withdrawal)]    signature)]
		//
		// Level 2: Beacon state deposit tree (built by GenerateGenesisStateFromDepositData)
		//       [Deposit Tree Root]
		//      /                   \
		//   [Hash]               [Hash]
		//   /    \               /    \
		// [Root0][Root1]     [Root2][Root3]
		//  ^LEAF  ^LEAF       ^LEAF  ^LEAF
		//
		// Note: depositDataRoots[i] is the "root" of level 1, but becomes a "leaf"
		// in level 2. It's called a root because HashTreeRoot() returns it.
		root, err := depositData.HashTreeRoot()
		if err != nil {
			return nil, nil, fmt.Errorf("hash tree root: %w", err)
		}

		depositDataItems[i] = depositData
		depositDataRoots[i] = root[:] // Root of deposit data, leaf of deposit tree
	}

	return depositDataItems, depositDataRoots, nil
}

// GenerateValidatorKeys generates deterministic BLS validator keys for testing.
// WARNING: Keys are NOT production-safe. All errors are critical.
func GenerateValidatorKeys(count int) ([]bls.SecretKey, error) {
	// Use Prysm's deterministic key generation for interop compatibility
	secretKeys, _, err := interop.DeterministicallyGenerateKeys(0, uint64(count))
	if err != nil {
		return nil, fmt.Errorf("generate keys: %w", err)
	}

	return secretKeys, nil
}
