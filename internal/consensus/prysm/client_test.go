package prysm_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/consensus/prysm"
	"github.com/thep2p/go-eth-localnet/internal/node"
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

// TestClientLifecycle verifies the full beacon node lifecycle with a real Geth node.
// This test starts a Geth node with Engine API, then starts the Prysm beacon node
// connected to it, and verifies the beacon node becomes ready.
//
// Note: This test uses Prysm's minimal config, which modifies global state.
// It must not run in parallel with tests that depend on mainnet config.
func TestClientLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Save original Prysm config and restore after test
	// Prysm uses global state for beacon config, which affects other tests
	originalConfig := params.BeaconConfig().Copy()
	t.Cleanup(func() {
		params.OverrideBeaconConfig(originalConfig)
	})

	// Override logrus ExitFunc to prevent os.Exit during shutdown
	// Prysm's P2P service calls log.Fatal during context cancellation
	originalExitFunc := logrus.StandardLogger().ExitFunc
	logrus.StandardLogger().ExitFunc = func(code int) {
		t.Logf("logrus.Fatal called with code %d (ignored)", code)
	}
	t.Cleanup(func() {
		logrus.StandardLogger().ExitFunc = originalExitFunc
	})

	// Start Geth node with Engine API enabled
	gethCtx, gethCancel, manager := startGethWithEngineAPI(t)
	defer gethCancel()

	// Get Engine API connection info
	enginePort := manager.GetEnginePort(0)
	require.NotZero(t, enginePort, "engine api port should be assigned")

	jwtHex, err := manager.GetJWTSecret(0)
	require.NoError(t, err, "jwt secret should be readable")

	// Decode hex JWT secret to bytes
	jwtSecret, err := hex.DecodeString(strings.TrimSpace(string(jwtHex)))
	require.NoError(t, err, "jwt secret should be valid hex")

	// Create Prysm client config
	tmpDir := unittest.NewTempDir(t)
	t.Cleanup(tmpDir.Remove)

	validatorKeys, err := prysm.GenerateValidatorKeys(2)
	require.NoError(t, err)

	withdrawalAddrs := unittest.RandomAddresses(t, 2)

	cfg := consensus.Config{
		DataDir:             tmpDir.Path(),
		ChainID:             1337,
		GenesisTime:         time.Now().Add(-30 * time.Second),
		RPCPort:             unittest.NewPort(t),
		BeaconPort:          unittest.NewPort(t),
		P2PPort:             unittest.NewPort(t),
		EngineEndpoint:      fmt.Sprintf("http://127.0.0.1:%d", enginePort),
		JWTSecret:           jwtSecret,
		ValidatorKeys:       validatorKeys,
		WithdrawalAddresses: withdrawalAddrs,
		FeeRecipient:        withdrawalAddrs[0],
	}

	logger := unittest.Logger(t)
	client := prysm.NewClient(cfg, logger)

	// Start the Prysm client
	err = client.Start(gethCtx)
	require.NoError(t, err, "prysm client should start successfully")

	// Verify client becomes ready using skipgraph-go style helpers
	unittest.RequireReady(t, client)
	t.Log("prysm client is ready")

	// Give the node a moment to stabilize before shutdown
	// This helps avoid race conditions in Prysm's internal services
	time.Sleep(time.Second)

	// Stop the client
	client.Stop()

	// Verify client becomes done using skipgraph-go style helpers
	unittest.RequireDone(t, client)
	t.Log("prysm client stopped")
}

// TestClientP2PConfiguration verifies P2P networking is properly configured.
// This test starts the beacon node with P2P enabled and verifies it becomes ready.
// Note: We don't check specific P2P ports because Prysm's internal port allocation
// varies across environments. The node becoming ready indicates P2P initialized.
func TestClientP2PConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Save original Prysm config and restore after test
	originalConfig := params.BeaconConfig().Copy()
	t.Cleanup(func() {
		params.OverrideBeaconConfig(originalConfig)
	})

	// Override logrus ExitFunc to prevent os.Exit during shutdown
	originalExitFunc := logrus.StandardLogger().ExitFunc
	logrus.StandardLogger().ExitFunc = func(code int) {}
	t.Cleanup(func() {
		logrus.StandardLogger().ExitFunc = originalExitFunc
	})

	// Start Geth node with Engine API enabled
	gethCtx, gethCancel, manager := startGethWithEngineAPI(t)
	defer gethCancel()

	// Get Engine API connection info
	enginePort := manager.GetEnginePort(0)
	require.NotZero(t, enginePort)

	jwtHex, err := manager.GetJWTSecret(0)
	require.NoError(t, err)

	jwtSecret, err := hex.DecodeString(strings.TrimSpace(string(jwtHex)))
	require.NoError(t, err)

	// Create Prysm client config with P2P enabled
	tmpDir := unittest.NewTempDir(t)
	t.Cleanup(tmpDir.Remove)

	validatorKeys, err := prysm.GenerateValidatorKeys(2)
	require.NoError(t, err)

	withdrawalAddrs := unittest.RandomAddresses(t, 2)

	cfg := consensus.Config{
		DataDir:             tmpDir.Path(),
		ChainID:             1337,
		GenesisTime:         time.Now().Add(-30 * time.Second),
		RPCPort:             unittest.NewPort(t),
		BeaconPort:          unittest.NewPort(t),
		P2PPort:             unittest.NewPort(t),
		EngineEndpoint:      fmt.Sprintf("http://127.0.0.1:%d", enginePort),
		JWTSecret:           jwtSecret,
		ValidatorKeys:       validatorKeys,
		WithdrawalAddresses: withdrawalAddrs,
		FeeRecipient:        withdrawalAddrs[0],
	}

	logger := unittest.Logger(t)
	client := prysm.NewClient(cfg, logger)

	// Start the Prysm client
	err = client.Start(gethCtx)
	require.NoError(t, err)

	// Verify client becomes ready (includes P2P initialization)
	unittest.RequireReady(t, client)
	t.Log("prysm client with P2P is ready")

	// Give the node a moment to stabilize before shutdown
	// This helps avoid race conditions in Prysm's internal services
	time.Sleep(time.Second)

	// Stop the client
	client.Stop()
	unittest.RequireDone(t, client)
	t.Log("prysm client stopped cleanly")
}

// startGethWithEngineAPI starts a Geth node with Engine API enabled for testing.
func startGethWithEngineAPI(t *testing.T) (context.Context, context.CancelFunc, *node.Manager) {
	t.Helper()

	tmp := unittest.NewTempDir(t)
	launcher := node.NewLauncher(unittest.Logger(t))
	manager := node.NewNodeManager(
		unittest.Logger(t), launcher, tmp.Path(), func() int {
			return unittest.NewPort(t)
		},
	)

	// Enable Engine API before starting nodes
	require.NoError(t, manager.EnableEngineAPI(), "enable engine api should succeed")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(tmp.Remove)
	t.Cleanup(func() {
		unittest.RequireCallMustReturnWithinTimeout(
			t, manager.Done, node.ShutdownTimeout, "node shutdown failed",
		)
	})

	require.NoError(t, manager.Start(ctx, 1))
	gethNode := manager.GethNode()
	require.NotNil(t, gethNode)

	unittest.RequireRpcReadyWithinTimeout(t, ctx, manager.RPCPort(), node.OperationTimeout)

	return ctx, cancel, manager
}
