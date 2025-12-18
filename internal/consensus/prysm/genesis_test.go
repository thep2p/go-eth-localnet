package prysm_test

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/prysmaticlabs/prysm/v5/crypto/bls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/consensus/prysm"
)

// TestGenerateTestValidators verifies validator generation for testing.
func TestGenerateTestValidators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
	}{
		{
			name:  "single validator",
			count: 1,
		},
		{
			name:  "multiple validators",
			count: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := prysm.GenerateTestValidators(tt.count)
			require.NoError(t, err)
			require.Len(t, keys, tt.count)

			// Verify all keys are valid BLS12-381 keys
			for i, keyHex := range keys {
				assert.NotEmpty(t, keyHex, "validator %d should have private key", i)

				// Decode and parse as BLS key
				keyBytes, err := hexutil.Decode(keyHex)
				require.NoError(t, err, "validator %d key should be valid hex", i)

				secretKey, err := bls.SecretKeyFromBytes(keyBytes)
				require.NoError(t, err, "validator %d key should be valid bls key", i)

				// Verify we can derive public key
				publicKey := secretKey.PublicKey()
				require.NotNil(t, publicKey, "validator %d should have public key", i)
			}

			// Verify keys are unique
			if tt.count > 1 {
				assert.NotEqual(t, keys[0], keys[1], "validators should have unique keys")
			}
		})
	}
}

// TestGenerateTestValidatorsDeterminism verifies keys are deterministic.
func TestGenerateTestValidatorsDeterminism(t *testing.T) {
	t.Parallel()

	// Generate validators twice
	keys1, err := prysm.GenerateTestValidators(10)
	require.NoError(t, err)

	keys2, err := prysm.GenerateTestValidators(10)
	require.NoError(t, err)

	// Verify same keys are generated
	require.Equal(t, keys1, keys2, "same seed should generate same keys")
}

// TestGenerateGenesisStateValidation verifies genesis state validation.
func TestGenerateGenesisStateValidation(t *testing.T) {
	t.Parallel()

	withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validatorKeys, err := prysm.GenerateTestValidators(1)
	require.NoError(t, err)

	tests := []struct {
		name      string
		cfg       consensus.Config
		wantError string
	}{
		{
			name: "missing chain id",
			cfg: consensus.Config{
				GenesisTime:         time.Now(),
				ValidatorKeys:       validatorKeys,
				WithdrawalAddresses: []common.Address{withdrawalAddr},
			},
			wantError: "chain id",
		},
		{
			name: "missing genesis time",
			cfg: consensus.Config{
				ChainID:             1337,
				ValidatorKeys:       validatorKeys,
				WithdrawalAddresses: []common.Address{withdrawalAddr},
			},
			wantError: "genesis time",
		},
		{
			name: "missing validators",
			cfg: consensus.Config{
				ChainID:             1337,
				GenesisTime:         time.Now(),
				WithdrawalAddresses: []common.Address{withdrawalAddr},
			},
			wantError: "at least one validator",
		},
		{
			name: "mismatched withdrawal addresses count",
			cfg: consensus.Config{
				ChainID:             1337,
				GenesisTime:         time.Now(),
				ValidatorKeys:       validatorKeys,
				WithdrawalAddresses: []common.Address{}, // Empty slice, should error
			},
			wantError: "withdrawal addresses count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := prysm.GenerateGenesisState(tt.cfg)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantError)
			require.Nil(t, state)
		})
	}
}

// TestGenerateGenesisState verifies genesis state generation.
func TestGenerateGenesisState(t *testing.T) {
	t.Parallel()

	validatorKeys, err := prysm.GenerateTestValidators(4)
	require.NoError(t, err)

	// Create unique withdrawal address for each validator
	withdrawalAddrs := []common.Address{
		common.HexToAddress("0x1111111111111111111111111111111111111111"),
		common.HexToAddress("0x2222222222222222222222222222222222222222"),
		common.HexToAddress("0x3333333333333333333333333333333333333333"),
		common.HexToAddress("0x4444444444444444444444444444444444444444"),
	}

	cfg := consensus.Config{
		ChainID:             1337,
		GenesisTime:         time.Now(),
		ValidatorKeys:       validatorKeys,
		WithdrawalAddresses: withdrawalAddrs,
		FeeRecipient:        withdrawalAddrs[0],
	}

	state, err := prysm.GenerateGenesisState(cfg)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotEmpty(t, state)

	// Verify we can derive the genesis root
	root, err := prysm.DeriveGenesisRoot(state)
	require.NoError(t, err)
	require.NotEqual(t, common.Hash{}, root)
}

// TestDeriveGenesisRootValidation verifies genesis root validation.
func TestDeriveGenesisRootValidation(t *testing.T) {
	t.Parallel()

	root, err := prysm.DeriveGenesisRoot(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
	require.Equal(t, common.Hash{}, root)

	root, err = prysm.DeriveGenesisRoot([]byte{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
	require.Equal(t, common.Hash{}, root)
}

// TestDeriveGenesisRoot verifies genesis root calculation.
func TestDeriveGenesisRoot(t *testing.T) {
	t.Parallel()

	// Generate a real genesis state for testing
	validatorKeys, err := prysm.GenerateTestValidators(2)
	require.NoError(t, err)

	// Create unique withdrawal address for each validator
	withdrawalAddrs := []common.Address{
		common.HexToAddress("0x1234567890123456789012345678901234567890"),
		common.HexToAddress("0xabcdabcdabcdabcdabcdabcdabcdabcdabcdabcd"),
	}

	cfg := consensus.Config{
		ChainID:             1337,
		GenesisTime:         time.Now(),
		ValidatorKeys:       validatorKeys,
		WithdrawalAddresses: withdrawalAddrs,
		FeeRecipient:        withdrawalAddrs[0],
	}

	genesisState, err := prysm.GenerateGenesisState(cfg)
	require.NoError(t, err)

	// Test deriving root from the generated state
	root, err := prysm.DeriveGenesisRoot(genesisState)
	require.NoError(t, err)
	require.NotEqual(t, common.Hash{}, root)

	// Verify determinism - same state should produce same root
	root2, err := prysm.DeriveGenesisRoot(genesisState)
	require.NoError(t, err)
	require.Equal(t, root, root2)
}
