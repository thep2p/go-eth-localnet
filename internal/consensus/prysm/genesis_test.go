package prysm_test

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/consensus/prysm"
	"github.com/thep2p/go-eth-localnet/internal/unittest"
)

// TestGenerateValidatorKeys verifies validator key generation for testing.
func TestGenerateValidatorKeys(t *testing.T) {
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
			keys, err := prysm.GenerateValidatorKeys(tt.count)
			require.NoError(t, err)
			require.Len(t, keys, tt.count)

			// Verify all keys are valid BLS12-381 keys
			for i, secretKey := range keys {
				// Verify we can derive public key
				publicKey := secretKey.PublicKey()
				require.NotNil(t, publicKey, "validator %d should have public key", i)
			}

			// Verify all keys are unique
			if tt.count > 1 {
				seen := make(map[string]bool)
				for i, secretKey := range keys {
					keyBytes := string(secretKey.Marshal())
					require.False(t, seen[keyBytes], "validator %d has duplicate key", i)
					seen[keyBytes] = true
				}
			}
		})
	}
}

// TestGenerateValidatorKeysDeterminism verifies keys are deterministic.
func TestGenerateValidatorKeysDeterminism(t *testing.T) {
	t.Parallel()

	// Generate validators twice
	keys1, err := prysm.GenerateValidatorKeys(10)
	require.NoError(t, err)

	keys2, err := prysm.GenerateValidatorKeys(10)
	require.NoError(t, err)

	// Verify same keys are generated (compare marshaled bytes)
	require.Len(t, keys1, len(keys2))
	for i := range keys1 {
		require.Equal(t, keys1[i].Marshal(), keys2[i].Marshal(), "same seed should generate same keys")
	}
}

// TestGenerateGenesisStateValidation verifies genesis state validation.
func TestGenerateGenesisStateValidation(t *testing.T) {
	t.Parallel()

	withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validatorKeys, err := prysm.GenerateValidatorKeys(1)
	require.NoError(t, err)

	// Base valid config for tests
	baseConfig := consensus.Config{
		DataDir:             "/tmp/test",
		ChainID:             1337,
		GenesisTime:         time.Now(),
		BeaconPort:          4000,
		P2PPort:             9000,
		EngineEndpoint:      "http://localhost:8551",
		JWTSecret:           []byte("secret"),
		ValidatorKeys:       validatorKeys,
		WithdrawalAddresses: []common.Address{withdrawalAddr},
	}

	tests := []struct {
		name      string
		cfg       consensus.Config
		wantError string
	}{
		{
			name: "missing chain id",
			cfg: func() consensus.Config {
				cfg := baseConfig
				cfg.ChainID = 0
				return cfg
			}(),
			wantError: "ChainID",
		},
		{
			name: "missing genesis time",
			cfg: func() consensus.Config {
				cfg := baseConfig
				cfg.GenesisTime = time.Time{}
				return cfg
			}(),
			wantError: "GenesisTime",
		},
		{
			name: "missing validators",
			cfg: func() consensus.Config {
				cfg := baseConfig
				cfg.ValidatorKeys = nil
				return cfg
			}(),
			wantError: "at least one validator",
		},
		{
			name: "mismatched withdrawal addresses count",
			cfg: func() consensus.Config {
				cfg := baseConfig
				cfg.WithdrawalAddresses = []common.Address{} // Empty slice, should error
				return cfg
			}(),
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

	validatorKeys, err := prysm.GenerateValidatorKeys(4)
	require.NoError(t, err)

	// Create unique withdrawal address for each validator
	withdrawalAddrs := unittest.RandomAddresses(t, 4)

	cfg := consensus.Config{
		DataDir:             "/tmp/test",
		ChainID:             1337,
		GenesisTime:         time.Now(),
		BeaconPort:          4000,
		P2PPort:             9000,
		EngineEndpoint:      "http://localhost:8551",
		JWTSecret:           []byte("secret"),
		ValidatorKeys:       validatorKeys,
		WithdrawalAddresses: withdrawalAddrs,
		FeeRecipient:        withdrawalAddrs[0],
	}

	state, err := prysm.GenerateGenesisState(cfg)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotEmpty(t, state)

	// Verify we can derive the genesis root
	_, err = prysm.DeriveGenesisRoot(state)
	require.NoError(t, err)

}

// TestDeriveGenesisRootValidation verifies genesis root validation.
func TestDeriveGenesisRootValidation(t *testing.T) {
	t.Parallel()

	root, err := prysm.DeriveGenesisRoot(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
	require.Zero(t, root, "root should be zero hash on error")

	root, err = prysm.DeriveGenesisRoot([]byte{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
	require.Zero(t, root, "root should be zero hash on error")
}

// TestDeriveGenesisRootDeterminism verifies genesis root derivation is deterministic.
func TestDeriveGenesisRootDeterminism(t *testing.T) {
	t.Parallel()

	// Generate a real genesis state for testing
	validatorKeys, err := prysm.GenerateValidatorKeys(2)
	require.NoError(t, err)

	// Create unique withdrawal address for each validator
	withdrawalAddrs := unittest.RandomAddresses(t, 2)
	cfg := consensus.Config{
		DataDir:             "/tmp/test",
		ChainID:             1337,
		GenesisTime:         time.Now(),
		BeaconPort:          4000,
		P2PPort:             9000,
		EngineEndpoint:      "http://localhost:8551",
		JWTSecret:           []byte("secret"),
		ValidatorKeys:       validatorKeys,
		WithdrawalAddresses: withdrawalAddrs,
		FeeRecipient:        withdrawalAddrs[0],
	}

	genesisState, err := prysm.GenerateGenesisState(cfg)
	require.NoError(t, err)

	// Test deriving root from the generated state
	root, err := prysm.DeriveGenesisRoot(genesisState)
	require.NoError(t, err)

	// Verify determinism - same state should produce same root
	root2, err := prysm.DeriveGenesisRoot(genesisState)
	require.NoError(t, err)
	require.Equal(t, root, root2)
}
