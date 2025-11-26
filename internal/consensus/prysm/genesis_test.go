package prysm_test

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/consensus/prysm"
)

// TestDefaultGenesisTime verifies default genesis time generation.
func TestDefaultGenesisTime(t *testing.T) {
	t.Parallel()

	genesisTime := prysm.DefaultGenesisTime()
	now := time.Now()

	// Genesis time should be in the past (within last minute)
	assert.True(t, genesisTime.Before(now))
	assert.True(t, now.Sub(genesisTime) < time.Minute)
}

// TestGenerateTestValidators verifies validator generation for testing.
func TestGenerateTestValidators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		count     int
		wantError string
	}{
		{
			name:  "single validator",
			count: 1,
		},
		{
			name:  "multiple validators",
			count: 4,
		},
		{
			name:      "zero validators",
			count:     0,
			wantError: "must be positive",
		},
		{
			name:      "negative validators",
			count:     -1,
			wantError: "must be positive",
		},
		{
			name:      "too many validators",
			count:     1001,
			wantError: "too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := prysm.GenerateTestValidators(tt.count)

			if tt.wantError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantError)
				return
			}

			require.NoError(t, err)
			require.Len(t, keys, tt.count)

			// Verify all keys are non-empty strings
			for i, key := range keys {
				assert.NotEmpty(t, key, "validator %d should have private key", i)
			}

			// Verify keys are unique
			if tt.count > 1 {
				assert.NotEqual(t, keys[0], keys[1], "validators should have unique keys")
			}
		})
	}
}

// TestGenerateGenesisStateValidation verifies genesis state validation.
func TestGenerateGenesisStateValidation(t *testing.T) {
	t.Parallel()

	withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validatorKeys, err := prysm.GenerateTestValidators(1)
	require.NoError(t, err)
	gethGenesisHash := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")

	tests := []struct {
		name      string
		cfg       consensus.Config
		wantError string
	}{
		{
			name: "missing chain id",
			cfg: consensus.Config{
				GenesisTime:   time.Now(),
				ValidatorKeys: validatorKeys,
			},
			wantError: "chain id",
		},
		{
			name: "missing genesis time",
			cfg: consensus.Config{
				ChainID:       1337,
				ValidatorKeys: validatorKeys,
			},
			wantError: "genesis time",
		},
		{
			name: "missing validators",
			cfg: consensus.Config{
				ChainID:     1337,
				GenesisTime: time.Now(),
			},
			wantError: "at least one validator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := prysm.GenerateGenesisState(tt.cfg, withdrawalAddr, gethGenesisHash, 0, uint64(time.Now().Unix()))
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantError)
			require.Nil(t, state)
		})
	}
}

// TestGenerateGenesisState verifies genesis state generation.
func TestGenerateGenesisState(t *testing.T) {
	t.Skip("Skipping until #47 is implemented: https://github.com/thep2p/go-eth-localnet/issues/47")
	t.Parallel()

	withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validatorKeys, err := prysm.GenerateTestValidators(4)
	require.NoError(t, err)

	now := time.Now()
	cfg := consensus.Config{
		ChainID:       1337,
		GenesisTime:   now,
		ValidatorKeys: validatorKeys,
		FeeRecipient:  withdrawalAddr,
	}

	gethGenesisHash := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	gethGenesisNumber := uint64(0)
	gethGenesisTimestamp := uint64(now.Unix())

	state, err := prysm.GenerateGenesisState(cfg, withdrawalAddr, gethGenesisHash, gethGenesisNumber, gethGenesisTimestamp)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotEmpty(t, state)
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
	t.Skip("Skipping until #47 is implemented: https://github.com/thep2p/go-eth-localnet/issues/47")
	t.Parallel()

	// Create a dummy genesis state for testing
	genesisState := []byte("test-genesis-state")

	root, err := prysm.DeriveGenesisRoot(genesisState)
	require.NoError(t, err)
	require.NotEqual(t, common.Hash{}, root)
}
