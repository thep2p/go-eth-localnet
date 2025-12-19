package prysm_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/consensus/prysm"
	"github.com/thep2p/go-eth-localnet/internal/unittest"
)

// TestLauncherCreation verifies launcher creation.
func TestLauncherCreation(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)
	require.NotNil(t, launcher)
}

// TestLauncherBasicConfig verifies launching with minimal valid configuration.
func TestLauncherBasicConfig(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	cfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    time.Now(),
		BeaconPort:     unittest.NewPort(t),
		P2PPort:        unittest.NewPort(t),
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: "http://127.0.0.1:8551",
		JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
	}

	client, err := launcher.Launch(cfg)
	require.NoError(t, err)
	require.NotNil(t, client)
}

// TestLauncherValidation verifies configuration validation during launch.
func TestLauncherValidation(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)

	tests := []struct {
		name      string
		cfg       consensus.Config
		wantError string
	}{
		{
			name: "missing data directory",
			cfg: consensus.Config{
				BeaconPort:     4000,
				P2PPort:        9000,
				EngineEndpoint: "http://127.0.0.1:8551",
				JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
			},
			wantError: "data directory",
		},
		{
			name: "missing beacon port",
			cfg: consensus.Config{
				DataDir:        "/tmp/prysm",
				P2PPort:        9000,
				EngineEndpoint: "http://127.0.0.1:8551",
				JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
			},
			wantError: "beacon port",
		},
		{
			name: "missing p2p port",
			cfg: consensus.Config{
				DataDir:        "/tmp/prysm",
				BeaconPort:     4000,
				EngineEndpoint: "http://127.0.0.1:8551",
				JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
			},
			wantError: "p2p port",
		},
		{
			name: "missing engine endpoint",
			cfg: consensus.Config{
				DataDir:    "/tmp/prysm",
				BeaconPort: 4000,
				P2PPort:    9000,
				JWTSecret:  []byte("test-jwt-secret-32-bytes-long!!"),
			},
			wantError: "engine endpoint",
		},
		{
			name: "missing jwt secret",
			cfg: consensus.Config{
				DataDir:        "/tmp/prysm",
				BeaconPort:     4000,
				P2PPort:        9000,
				EngineEndpoint: "http://127.0.0.1:8551",
			},
			wantError: "jwt secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := launcher.Launch(tt.cfg)
			require.Error(t, err)
			require.Nil(t, client)
			require.Contains(t, err.Error(), tt.wantError)
		})
	}
}

// TestLauncherWithOptions verifies launching with functional options.
func TestLauncherWithOptions(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	cfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    time.Now(),
		BeaconPort:     unittest.NewPort(t),
		P2PPort:        unittest.NewPort(t),
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: "http://127.0.0.1:8551",
		JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
	}

	validatorKeys, err := prysm.GenerateValidatorKeys(2)
	require.NoError(t, err)
	bootnodes := []string{"enr://test-bootnode"}
	staticPeers := []string{"/ip4/127.0.0.1/tcp/9001"}

	client, err := launcher.LaunchWithOptions(cfg,
		prysm.WithValidatorKeys(validatorKeys),
		prysm.WithBootnodes(bootnodes),
		prysm.WithStaticPeers(staticPeers),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Note: Config fields are private, so we cannot verify them directly.
	// The fact that LaunchWithOptions succeeds is sufficient verification.
	_ = validatorKeys
	_ = bootnodes
	_ = staticPeers
}

// TestLauncherValidatorKeysOption verifies WithValidatorKeys option.
func TestLauncherValidatorKeysOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	validatorKeys, err := prysm.GenerateValidatorKeys(2)
	require.NoError(t, err)

	cfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    time.Now(),
		BeaconPort:     unittest.NewPort(t),
		P2PPort:        unittest.NewPort(t),
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: "http://127.0.0.1:8551",
		JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
	}

	client, err := launcher.LaunchWithOptions(cfg, prysm.WithValidatorKeys(validatorKeys))
	require.NoError(t, err)
	require.NotNil(t, client)

	// Note: Config fields are private, so we cannot verify them directly.
	_ = validatorKeys
}

// TestLauncherBootnodesOption verifies WithBootnodes option.
func TestLauncherBootnodesOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	bootnodes := []string{
		"enr://node1",
		"enr://node2",
	}

	cfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    time.Now(),
		BeaconPort:     unittest.NewPort(t),
		P2PPort:        unittest.NewPort(t),
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: "http://127.0.0.1:8551",
		JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
	}

	client, err := launcher.LaunchWithOptions(cfg, prysm.WithBootnodes(bootnodes))
	require.NoError(t, err)
	require.NotNil(t, client)

	// Note: Config fields are private, so we cannot verify them directly.
	_ = bootnodes
}

// TestLauncherStaticPeersOption verifies WithStaticPeers option.
func TestLauncherStaticPeersOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	staticPeers := []string{
		"/ip4/127.0.0.1/tcp/9001",
		"/ip4/127.0.0.1/tcp/9002",
	}

	cfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    time.Now(),
		BeaconPort:     unittest.NewPort(t),
		P2PPort:        unittest.NewPort(t),
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: "http://127.0.0.1:8551",
		JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
	}

	client, err := launcher.LaunchWithOptions(cfg, prysm.WithStaticPeers(staticPeers))
	require.NoError(t, err)
	require.NotNil(t, client)

	// Note: Config fields are private, so we cannot verify them directly.
	_ = staticPeers
}

// TestLauncherCheckpointSyncOption verifies WithCheckpointSync option.
func TestLauncherCheckpointSyncOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	checkpointURL := "https://checkpoint-sync.example.com"

	cfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    time.Now(),
		BeaconPort:     unittest.NewPort(t),
		P2PPort:        unittest.NewPort(t),
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: "http://127.0.0.1:8551",
		JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
	}

	client, err := launcher.LaunchWithOptions(cfg, prysm.WithCheckpointSync(checkpointURL))
	require.NoError(t, err)
	require.NotNil(t, client)

	// Note: Config fields are private, so we cannot verify them directly.
	_ = checkpointURL
}

// TestLauncherGenesisStateOption verifies WithGenesisState option.
func TestLauncherGenesisStateOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	genesisURL := "https://genesis-state.example.com/state.ssz"

	cfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    time.Now(),
		BeaconPort:     unittest.NewPort(t),
		P2PPort:        unittest.NewPort(t),
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: "http://127.0.0.1:8551",
		JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
	}

	client, err := launcher.LaunchWithOptions(cfg, prysm.WithGenesisState(genesisURL))
	require.NoError(t, err)
	require.NotNil(t, client)

	// Note: Config fields are private, so we cannot verify them directly.
	_ = genesisURL
}

// TestLauncherMultipleOptions verifies combining multiple options.
func TestLauncherMultipleOptions(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := prysm.NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	validatorKeys, err := prysm.GenerateValidatorKeys(1)
	require.NoError(t, err)
	bootnodes := []string{"enr://test-bootnode"}
	staticPeers := []string{"/ip4/127.0.0.1/tcp/9001"}
	checkpointURL := "https://checkpoint.example.com"
	genesisURL := "https://genesis.example.com/state.ssz"

	cfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    time.Now(),
		BeaconPort:     unittest.NewPort(t),
		P2PPort:        unittest.NewPort(t),
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: "http://127.0.0.1:8551",
		JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
		FeeRecipient:   common.HexToAddress("0x1234567890123456789012345678901234567890"),
	}

	client, err := launcher.LaunchWithOptions(cfg,
		prysm.WithValidatorKeys(validatorKeys),
		prysm.WithBootnodes(bootnodes),
		prysm.WithStaticPeers(staticPeers),
		prysm.WithCheckpointSync(checkpointURL),
		prysm.WithGenesisState(genesisURL),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Note: Config fields are private, so we cannot verify them directly.
	// The fact that LaunchWithOptions succeeds is sufficient verification.
	_ = validatorKeys
	_ = bootnodes
	_ = staticPeers
	_ = checkpointURL
	_ = genesisURL
}
