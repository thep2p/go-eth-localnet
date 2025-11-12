package prysm

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/unittest"
)

// TestLauncherCreation verifies launcher creation.
func TestLauncherCreation(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := NewLauncher(logger)
	require.NotNil(t, launcher)
}

// TestLauncherBasicConfig verifies launching with minimal valid configuration.
func TestLauncherBasicConfig(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := NewLauncher(logger)
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
	launcher := NewLauncher(logger)

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
	launcher := NewLauncher(logger)
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

	validatorKeys := []string{"test-key-1", "test-key-2"}
	bootnodes := []string{"enr://test-bootnode"}
	staticPeers := []string{"/ip4/127.0.0.1/tcp/9001"}

	client, err := launcher.LaunchWithOptions(cfg,
		WithValidatorKeys(validatorKeys),
		WithBootnodes(bootnodes),
		WithStaticPeers(staticPeers),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify options were applied
	assert.Equal(t, validatorKeys, client.config.ValidatorKeys)
	assert.Equal(t, bootnodes, client.config.Bootnodes)
	assert.Equal(t, staticPeers, client.config.StaticPeers)
}

// TestLauncherValidatorKeysOption verifies WithValidatorKeys option.
func TestLauncherValidatorKeysOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	validatorKeys := []string{
		"0x1234567890abcdef",
		"0xfedcba0987654321",
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

	client, err := launcher.LaunchWithOptions(cfg, WithValidatorKeys(validatorKeys))
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, validatorKeys, client.config.ValidatorKeys)
}

// TestLauncherBootnodesOption verifies WithBootnodes option.
func TestLauncherBootnodesOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := NewLauncher(logger)
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

	client, err := launcher.LaunchWithOptions(cfg, WithBootnodes(bootnodes))
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, bootnodes, client.config.Bootnodes)
}

// TestLauncherStaticPeersOption verifies WithStaticPeers option.
func TestLauncherStaticPeersOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := NewLauncher(logger)
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

	client, err := launcher.LaunchWithOptions(cfg, WithStaticPeers(staticPeers))
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, staticPeers, client.config.StaticPeers)
}

// TestLauncherCheckpointSyncOption verifies WithCheckpointSync option.
func TestLauncherCheckpointSyncOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := NewLauncher(logger)
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

	client, err := launcher.LaunchWithOptions(cfg, WithCheckpointSync(checkpointURL))
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, checkpointURL, client.config.CheckpointSyncURL)
}

// TestLauncherGenesisStateOption verifies WithGenesisState option.
func TestLauncherGenesisStateOption(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := NewLauncher(logger)
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

	client, err := launcher.LaunchWithOptions(cfg, WithGenesisState(genesisURL))
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, genesisURL, client.config.GenesisStateURL)
}

// TestLauncherMultipleOptions verifies combining multiple options.
func TestLauncherMultipleOptions(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	launcher := NewLauncher(logger)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	validatorKeys := []string{"test-key-1"}
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
		WithValidatorKeys(validatorKeys),
		WithBootnodes(bootnodes),
		WithStaticPeers(staticPeers),
		WithCheckpointSync(checkpointURL),
		WithGenesisState(genesisURL),
	)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify all options were applied
	assert.Equal(t, validatorKeys, client.config.ValidatorKeys)
	assert.Equal(t, bootnodes, client.config.Bootnodes)
	assert.Equal(t, staticPeers, client.config.StaticPeers)
	assert.Equal(t, checkpointURL, client.config.CheckpointSyncURL)
	assert.Equal(t, genesisURL, client.config.GenesisStateURL)
}
