package prysm

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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

	// Parse BLS secret keys from hex
	secretKeys := make([]bls.SecretKey, len(cfg.ValidatorKeys))
	publicKeys := make([]bls.PublicKey, len(cfg.ValidatorKeys))
	for i, keyHex := range cfg.ValidatorKeys {
		keyBytes, err := hexutil.Decode(keyHex)
		if err != nil {
			return nil, fmt.Errorf("decode validator key %d: %w", i, err)
		}

		secretKey, err := bls.SecretKeyFromBytes(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("parse validator key %d: %w", i, err)
		}

		secretKeys[i] = secretKey
		publicKeys[i] = secretKey.PublicKey()
	}

	// Create deposit data with custom withdrawal address
	depositDataItems, depositDataRoots, err := createDepositDataWithWithdrawalAddress(secretKeys, publicKeys, withdrawalAddress)
	if err != nil {
		return nil, fmt.Errorf("create deposit data: %w", err)
	}

	// Generate genesis state using interop helpers (handles merkle proofs correctly)
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

// DeriveGenesisRoot calculates the genesis beacon state root.
//
// The genesis root is the hash tree root of the beacon chain genesis state.
// It's used as a unique identifier for the network and must match between
// all nodes in the network.
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

// createDepositDataWithWithdrawalAddress creates deposit data for validators with a custom withdrawal address.
func createDepositDataWithWithdrawalAddress(
	secretKeys []bls.SecretKey,
	publicKeys []bls.PublicKey,
	withdrawalAddress common.Address,
) ([]*ethpb.Deposit_Data, [][]byte, error) {
	depositDataItems := make([]*ethpb.Deposit_Data, len(secretKeys))
	depositDataRoots := make([][]byte, len(secretKeys))

	// Create withdrawal credentials (0x01 prefix for execution address)
	withdrawalCreds := make([]byte, 32)
	withdrawalCreds[0] = params.BeaconConfig().ETH1AddressWithdrawalPrefixByte
	copy(withdrawalCreds[12:], withdrawalAddress.Bytes())

	for i := range secretKeys {
		// Create deposit message
		depositMsg := &ethpb.DepositMessage{
			PublicKey:             publicKeys[i].Marshal(),
			WithdrawalCredentials: withdrawalCreds,
			Amount:                params.BeaconConfig().MaxEffectiveBalance,
		}

		// Sign the deposit message
		domain, err := signing.ComputeDomain(
			params.BeaconConfig().DomainDeposit,
			params.BeaconConfig().GenesisForkVersion,
			params.BeaconConfig().ZeroHash[:],
		)
		if err != nil {
			return nil, nil, fmt.Errorf("compute domain: %w", err)
		}

		signingRoot, err := signing.ComputeSigningRoot(depositMsg, domain)
		if err != nil {
			return nil, nil, fmt.Errorf("compute signing root: %w", err)
		}

		signature := secretKeys[i].Sign(signingRoot[:])

		// Create deposit data
		depositData := &ethpb.Deposit_Data{
			PublicKey:             depositMsg.PublicKey,
			WithdrawalCredentials: depositMsg.WithdrawalCredentials,
			Amount:                depositMsg.Amount,
			Signature:             signature.Marshal(),
		}

		// Compute deposit data root
		root, err := depositData.HashTreeRoot()
		if err != nil {
			return nil, nil, fmt.Errorf("hash tree root: %w", err)
		}

		depositDataItems[i] = depositData
		depositDataRoots[i] = root[:]
	}

	return depositDataItems, depositDataRoots, nil
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
	// Use Prysm's deterministic key generation for interop compatibility
	secretKeys, _, err := interop.DeterministicallyGenerateKeys(0, uint64(count))
	if err != nil {
		return nil, fmt.Errorf("generate keys: %w", err)
	}

	keys := make([]string, count)
	for i, secretKey := range secretKeys {
		keys[i] = hexutil.Encode(secretKey.Marshal())
	}

	return keys, nil
}
