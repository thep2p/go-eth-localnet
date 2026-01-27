package prysm_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/consensus/prysm"
	"github.com/thep2p/go-eth-localnet/internal/unittest"
)

// TestClientInvalidConfig verifies validation error handling during Start.
// These tests verify that invalid configurations are rejected early during
// genesis state generation, before the beacon node is created.
func TestClientInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		modifyCfg func(*consensus.Config)
		wantError string
	}{
		{
			name: "missing validators",
			modifyCfg: func(cfg *consensus.Config) {
				cfg.ValidatorKeys = nil
			},
			wantError: "validator",
		},
		{
			name: "zero chain id",
			modifyCfg: func(cfg *consensus.Config) {
				cfg.ChainID = 0
			},
			wantError: "ChainID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := unittest.NewTempDir(t)
			t.Cleanup(tmpDir.Remove)

			// Create minimal config
			validatorKeys, err := prysm.GenerateValidatorKeys(1)
			require.NoError(t, err)

			withdrawalAddrs := unittest.RandomAddresses(t, 1)

			cfg := consensus.Config{
				DataDir:             tmpDir.Path(),
				ChainID:             1337,
				GenesisTime:         time.Now().Add(-30 * time.Second),
				RPCPort:             unittest.NewPort(t),
				BeaconPort:          unittest.NewPort(t),
				P2PPort:             unittest.NewPort(t),
				EngineEndpoint:      "http://localhost:8551",
				JWTSecret:           []byte("0123456789abcdef0123456789abcdef"),
				ValidatorKeys:       validatorKeys,
				WithdrawalAddresses: withdrawalAddrs,
				FeeRecipient:        withdrawalAddrs[0],
			}

			// Apply test modification
			tt.modifyCfg(&cfg)

			logger := unittest.Logger(t)
			client := prysm.NewClient(cfg, logger)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = client.Start(ctx)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantError)
		})
	}
}

// TestNewClient verifies client construction.
func TestNewClient(t *testing.T) {
	t.Parallel()

	cfg := consensus.Config{
		DataDir:        "/tmp/test",
		ChainID:        1337,
		RPCPort:        4000,
		BeaconPort:     3500,
		P2PPort:        9000,
		EngineEndpoint: "http://localhost:8551",
		JWTSecret:      []byte("secret"),
	}
	logger := unittest.Logger(t)

	client := prysm.NewClient(cfg, logger)
	require.NotNil(t, client)

	// Verify Ready and Done channels are non-nil and not closed
	select {
	case <-client.Ready():
		t.Fatal("ready channel should not be closed before start")
	default:
		// Good - channel is open
	}

	select {
	case <-client.Done():
		t.Fatal("done channel should not be closed before start")
	default:
		// Good - channel is open
	}
}

// TestNewClientReadyDone verifies the Ready and Done channel semantics.
func TestNewClientReadyDone(t *testing.T) {
	t.Parallel()

	validatorKeys, err := prysm.GenerateValidatorKeys(2)
	require.NoError(t, err)

	withdrawalAddrs := unittest.RandomAddresses(t, 2)

	cfg := consensus.Config{
		DataDir:             "/tmp/test",
		ChainID:             1337,
		GenesisTime:         time.Now(),
		RPCPort:             4000,
		BeaconPort:          3500,
		P2PPort:             9000,
		EngineEndpoint:      "http://localhost:8551",
		JWTSecret:           []byte("secret"),
		ValidatorKeys:       validatorKeys,
		WithdrawalAddresses: withdrawalAddrs,
		FeeRecipient:        withdrawalAddrs[0],
	}
	logger := unittest.Logger(t)

	client := prysm.NewClient(cfg, logger)

	// Ready() should return a receive-only channel
	readyCh := client.Ready()
	require.NotNil(t, readyCh)

	// Done() should return a receive-only channel
	doneCh := client.Done()
	require.NotNil(t, doneCh)
}

// TestClientConstants verifies timeout constants are defined correctly.
func TestClientConstants(t *testing.T) {
	t.Parallel()

	// Verify timeout constants are reasonable values
	require.Greater(t, prysm.StartupTimeout, time.Duration(0))
	require.Greater(t, prysm.ShutdownTimeout, time.Duration(0))
	require.Greater(t, prysm.ReadyDoneTimeout, time.Duration(0))

	// StartupTimeout should be longer than ShutdownTimeout (starting takes longer)
	require.GreaterOrEqual(t, prysm.StartupTimeout, prysm.ShutdownTimeout)
}
