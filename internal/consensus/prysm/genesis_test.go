package prysm

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultGenesisTime verifies default genesis time generation.
func TestDefaultGenesisTime(t *testing.T) {
	t.Parallel()

	genesisTime := DefaultGenesisTime()
	now := time.Now()

	// Genesis time should be in the past (within last minute)
	assert.True(t, genesisTime.Before(now))
	assert.True(t, now.Sub(genesisTime) < time.Minute)
}

// TestMinGenesisActiveValidatorCount verifies minimum validator count.
func TestMinGenesisActiveValidatorCount(t *testing.T) {
	t.Parallel()

	minCount := MinGenesisActiveValidatorCount()
	assert.Equal(t, 1, minCount, "Local development should require only 1 validator")
}

// TestGenesisDelay verifies genesis delay configuration.
func TestGenesisDelay(t *testing.T) {
	t.Parallel()

	delay := GenesisDelay()
	assert.Equal(t, time.Duration(0), delay, "Local development should have no delay")
}

// TestGenerateTestValidators verifies validator generation for testing.
func TestGenerateTestValidators(t *testing.T) {
	t.Parallel()

	withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

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
			validators, err := GenerateTestValidators(tt.count, withdrawalAddr)

			if tt.wantError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantError)
				return
			}

			require.NoError(t, err)
			require.Len(t, validators, tt.count)

			// Verify all validators have required fields
			for i, v := range validators {
				assert.NotEmpty(t, v.PrivateKey, "validator %d should have private key", i)
				assert.Equal(t, withdrawalAddr, v.WithdrawalCredentials, "validator %d should have withdrawal address", i)
			}

			// Verify validators are unique
			if tt.count > 1 {
				assert.NotEqual(t, validators[0].PrivateKey, validators[1].PrivateKey, "validators should have unique keys")
			}
		})
	}
}

// TestGenerateGenesisStateValidation verifies genesis state validation.
func TestGenerateGenesisStateValidation(t *testing.T) {
	t.Parallel()

	withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validators, err := GenerateTestValidators(1, withdrawalAddr)
	require.NoError(t, err)

	tests := []struct {
		name      string
		cfg       GenesisConfig
		wantError string
	}{
		{
			name: "missing chain id",
			cfg: GenesisConfig{
				GenesisTime:       time.Now(),
				GenesisValidators: validators,
			},
			wantError: "chain id",
		},
		{
			name: "missing genesis time",
			cfg: GenesisConfig{
				ChainID:           1337,
				GenesisValidators: validators,
			},
			wantError: "genesis time",
		},
		{
			name: "missing validators",
			cfg: GenesisConfig{
				ChainID:     1337,
				GenesisTime: time.Now(),
			},
			wantError: "at least one validator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := GenerateGenesisState(tt.cfg)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantError)
			require.Nil(t, state)
		})
	}
}

// TestGenerateGenesisState verifies genesis state generation.
func TestGenerateGenesisState(t *testing.T) {
	t.Skip("Skipping until Prysm integration is implemented")
	t.Parallel()

	withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validators, err := GenerateTestValidators(4, withdrawalAddr)
	require.NoError(t, err)

	cfg := GenesisConfig{
		ChainID:           1337,
		GenesisTime:       time.Now(),
		GenesisValidators: validators,
		ExecutionPayloadHeader: ExecutionHeader{
			BlockHash:   common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
			BlockNumber: 0,
			Timestamp:   uint64(time.Now().Unix()),
		},
	}

	state, err := GenerateGenesisState(cfg)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotEmpty(t, state)
}

// TestDeriveGenesisRootValidation verifies genesis root validation.
func TestDeriveGenesisRootValidation(t *testing.T) {
	t.Parallel()

	root, err := DeriveGenesisRoot(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
	require.Equal(t, [32]byte{}, root)

	root, err = DeriveGenesisRoot([]byte{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
	require.Equal(t, [32]byte{}, root)
}

// TestDeriveGenesisRoot verifies genesis root calculation.
func TestDeriveGenesisRoot(t *testing.T) {
	t.Skip("Skipping until Prysm integration is implemented")
	t.Parallel()

	// Create a dummy genesis state for testing
	genesisState := []byte("test-genesis-state")

	root, err := DeriveGenesisRoot(genesisState)
	require.NoError(t, err)
	require.NotEqual(t, [32]byte{}, root)
}

// TestValidatorConfig verifies ValidatorConfig structure.
func TestValidatorConfig(t *testing.T) {
	t.Parallel()

	privateKey := "0x1234567890abcdef"
	withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	validator := ValidatorConfig{
		PrivateKey:            privateKey,
		WithdrawalCredentials: withdrawalAddr,
	}

	assert.Equal(t, privateKey, validator.PrivateKey)
	assert.Equal(t, withdrawalAddr, validator.WithdrawalCredentials)
}

// TestExecutionHeader verifies ExecutionHeader structure.
func TestExecutionHeader(t *testing.T) {
	t.Parallel()

	blockHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	blockNumber := uint64(123)
	timestamp := uint64(time.Now().Unix())

	header := ExecutionHeader{
		BlockHash:   blockHash,
		BlockNumber: blockNumber,
		Timestamp:   timestamp,
	}

	assert.Equal(t, blockHash, header.BlockHash)
	assert.Equal(t, blockNumber, header.BlockNumber)
	assert.Equal(t, timestamp, header.Timestamp)
}

// TestGenesisConfig verifies GenesisConfig structure.
func TestGenesisConfig(t *testing.T) {
	t.Parallel()

	withdrawalAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validators, err := GenerateTestValidators(2, withdrawalAddr)
	require.NoError(t, err)

	chainID := uint64(1337)
	genesisTime := time.Now()
	blockHash := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")

	cfg := GenesisConfig{
		ChainID:           chainID,
		GenesisTime:       genesisTime,
		GenesisValidators: validators,
		ExecutionPayloadHeader: ExecutionHeader{
			BlockHash:   blockHash,
			BlockNumber: 0,
			Timestamp:   uint64(genesisTime.Unix()),
		},
	}

	assert.Equal(t, chainID, cfg.ChainID)
	assert.Equal(t, genesisTime, cfg.GenesisTime)
	assert.Len(t, cfg.GenesisValidators, 2)
	assert.Equal(t, blockHash, cfg.ExecutionPayloadHeader.BlockHash)
	assert.Equal(t, uint64(0), cfg.ExecutionPayloadHeader.BlockNumber)
}
