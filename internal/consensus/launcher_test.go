package consensus

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// TestLaunchOptionWithValidatorKeys verifies WithValidatorKeys option.
func TestLaunchOptionWithValidatorKeys(t *testing.T) {
	cfg := Config{
		Client: "mock",
	}

	expectedKeys := []string{"key1", "key2", "key3"}
	opt := WithValidatorKeys(expectedKeys)
	opt(&cfg)

	require.Equal(t, expectedKeys, cfg.ValidatorKeys,
		"validator keys should be set correctly")
}

// TestLaunchOptionWithCheckpointSync verifies WithCheckpointSync option.
func TestLaunchOptionWithCheckpointSync(t *testing.T) {
	cfg := Config{
		Client: "mock",
	}

	expectedURL := "http://checkpoint.example.com"
	opt := WithCheckpointSync(expectedURL)
	opt(&cfg)

	require.Equal(t, expectedURL, cfg.CheckpointSyncURL,
		"checkpoint sync URL should be set correctly")
}

// TestLaunchOptionWithBootnodes verifies WithBootnodes option.
func TestLaunchOptionWithBootnodes(t *testing.T) {
	cfg := Config{
		Client: "mock",
	}

	expectedBootnodes := []string{"enr:node1", "enr:node2"}
	opt := WithBootnodes(expectedBootnodes)
	opt(&cfg)

	require.Equal(t, expectedBootnodes, cfg.Bootnodes,
		"bootnodes should be set correctly")
}

// TestLaunchOptionWithStaticPeers verifies WithStaticPeers option.
func TestLaunchOptionWithStaticPeers(t *testing.T) {
	cfg := Config{
		Client: "mock",
	}

	expectedPeers := []string{"peer1", "peer2"}
	opt := WithStaticPeers(expectedPeers)
	opt(&cfg)

	require.Equal(t, expectedPeers, cfg.StaticPeers,
		"static peers should be set correctly")
}

// TestLaunchOptionWithEngineEndpoint verifies WithEngineEndpoint option.
func TestLaunchOptionWithEngineEndpoint(t *testing.T) {
	cfg := Config{
		Client: "mock",
	}

	expectedEndpoint := "http://localhost:8551"
	opt := WithEngineEndpoint(expectedEndpoint)
	opt(&cfg)

	require.Equal(t, expectedEndpoint, cfg.EngineEndpoint,
		"engine endpoint should be set correctly")
}

// TestLaunchOptionWithJWTSecret verifies WithJWTSecret option.
func TestLaunchOptionWithJWTSecret(t *testing.T) {
	cfg := Config{
		Client: "mock",
	}

	expectedSecret := []byte("test-secret")
	opt := WithJWTSecret(expectedSecret)
	opt(&cfg)

	require.Equal(t, expectedSecret, cfg.JWTSecret,
		"JWT secret should be set correctly")
}

// TestMultipleLaunchOptions verifies applying multiple options.
func TestMultipleLaunchOptions(t *testing.T) {
	cfg := Config{
		Client: "mock",
	}

	// Apply multiple options
	opts := []LaunchOption{
		WithValidatorKeys([]string{"key1", "key2"}),
		WithBootnodes([]string{"enr:node1", "enr:node2"}),
		WithCheckpointSync("http://checkpoint.example.com"),
		WithStaticPeers([]string{"peer1"}),
		WithEngineEndpoint("http://localhost:8551"),
		WithJWTSecret([]byte("secret")),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Verify all options were applied
	require.Equal(t, []string{"key1", "key2"}, cfg.ValidatorKeys)
	require.Equal(t, []string{"enr:node1", "enr:node2"}, cfg.Bootnodes)
	require.Equal(t, "http://checkpoint.example.com", cfg.CheckpointSyncURL)
	require.Equal(t, []string{"peer1"}, cfg.StaticPeers)
	require.Equal(t, "http://localhost:8551", cfg.EngineEndpoint)
	require.Equal(t, []byte("secret"), cfg.JWTSecret)
}

// TestLaunchOptionChaining verifies options can be chained.
func TestLaunchOptionChaining(t *testing.T) {
	cfg := Config{
		Client: "mock",
	}

	// Chain options
	WithValidatorKeys([]string{"key1"})(&cfg)
	WithBootnodes([]string{"node1"})(&cfg)

	require.Equal(t, []string{"key1"}, cfg.ValidatorKeys,
		"first option should be applied")
	require.Equal(t, []string{"node1"}, cfg.Bootnodes,
		"second option should be applied")
}

// TestLaunchOptionOverride verifies that options can override previous values.
func TestLaunchOptionOverride(t *testing.T) {
	cfg := Config{
		Client:        "mock",
		ValidatorKeys: []string{"old-key"},
	}

	// Override with new keys
	newKeys := []string{"new-key1", "new-key2"}
	WithValidatorKeys(newKeys)(&cfg)

	require.Equal(t, newKeys, cfg.ValidatorKeys,
		"option should override existing value")
}

// TestCompleteConfigurationWithOptions verifies building a complete config.
func TestCompleteConfigurationWithOptions(t *testing.T) {
	// Start with base config
	cfg := Config{
		Client:     "mock",
		DataDir:    "/tmp/test",
		ChainID:    1337,
		BeaconPort: 4000,
		P2PPort:    9000,
		RPCPort:    5000,
	}

	// Apply options for runtime configuration
	opts := []LaunchOption{
		WithValidatorKeys([]string{"validator-key-1", "validator-key-2"}),
		WithEngineEndpoint("http://localhost:8551"),
		WithJWTSecret([]byte("jwt-secret-123")),
		WithBootnodes([]string{
			"enr:-Ku4QImhMc1z8yCiNJ1TyUxdcfNucje3BGwEHzodEZUan8PherEo4sF7pPHPSIB1NNuSg5fZy7qFsjmUKs2ea1Whi0EBh2F0dG5ldHOIAAAAAAAAAACEZXRoMpD1pf1CAAAAAP__________gmlkgnY0gmlwhBLf22SJc2VjcDI1NmsxoQOVphkDqal4QzPMksc5wnpuC3gvSC8AfbFOnZY_On34wIN1ZHCCIyg",
		}),
		WithStaticPeers([]string{"peer-1", "peer-2"}),
		WithCheckpointSync("http://checkpoint-sync.example.com"),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Verify complete configuration
	require.Equal(t, "mock", cfg.Client)
	require.Equal(t, "/tmp/test", cfg.DataDir)
	require.Equal(t, uint64(1337), cfg.ChainID)
	require.Equal(t, 4000, cfg.BeaconPort)
	require.Equal(t, 9000, cfg.P2PPort)
	require.Equal(t, 5000, cfg.RPCPort)
	require.Len(t, cfg.ValidatorKeys, 2)
	require.Equal(t, "http://localhost:8551", cfg.EngineEndpoint)
	require.Equal(t, []byte("jwt-secret-123"), cfg.JWTSecret)
	require.Len(t, cfg.Bootnodes, 1)
	require.Len(t, cfg.StaticPeers, 2)
	require.Equal(t, "http://checkpoint-sync.example.com", cfg.CheckpointSyncURL)
}

// TestConfigWithFeeRecipient verifies fee recipient configuration.
func TestConfigWithFeeRecipient(t *testing.T) {
	expectedAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	cfg := Config{
		Client:       "mock",
		FeeRecipient: expectedAddress,
	}

	require.Equal(t, expectedAddress, cfg.FeeRecipient,
		"fee recipient should be set correctly")
}

// TestEmptyLaunchOptions verifies that empty options don't cause issues.
func TestEmptyLaunchOptions(t *testing.T) {
	cfg := Config{
		Client: "mock",
	}

	// Apply empty options
	WithValidatorKeys(nil)(&cfg)
	WithBootnodes(nil)(&cfg)
	WithStaticPeers(nil)(&cfg)

	require.Nil(t, cfg.ValidatorKeys, "nil slice should be preserved")
	require.Nil(t, cfg.Bootnodes, "nil slice should be preserved")
	require.Nil(t, cfg.StaticPeers, "nil slice should be preserved")
}
