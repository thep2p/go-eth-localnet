package consensus

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TestRegistryRegisterAndGet verifies basic registry operations.
func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	launcher := NewMockLauncher(zerolog.Nop())

	// Register launcher
	err := registry.Register("test", launcher)
	require.NoError(t, err, "registration should succeed")

	// Get launcher
	got, err := registry.Get("test")
	require.NoError(t, err, "get should succeed")
	require.Equal(t, launcher, got, "retrieved launcher should match registered launcher")
}

// TestRegistryDuplicateRegistration verifies that duplicate registration fails.
func TestRegistryDuplicateRegistration(t *testing.T) {
	registry := NewRegistry()
	launcher := NewMockLauncher(zerolog.Nop())

	// First registration should succeed
	err := registry.Register("test", launcher)
	require.NoError(t, err, "first registration should succeed")

	// Duplicate registration should fail
	err = registry.Register("test", launcher)
	require.Error(t, err, "duplicate registration should fail")
	require.Contains(t, err.Error(), "already registered",
		"error should indicate launcher is already registered")
}

// TestRegistryGetUnknownLauncher verifies that getting an unknown launcher fails.
func TestRegistryGetUnknownLauncher(t *testing.T) {
	registry := NewRegistry()

	// Get unknown launcher
	_, err := registry.Get("unknown")
	require.Error(t, err, "getting unknown launcher should fail")
	require.Contains(t, err.Error(), "not found",
		"error should indicate launcher was not found")
}

// TestRegistryAvailable verifies listing available launchers.
func TestRegistryAvailable(t *testing.T) {
	registry := NewRegistry()

	// Initially empty
	available := registry.Available()
	require.Empty(t, available, "new registry should be empty")

	// Register multiple launchers
	launcher1 := NewMockLauncher(zerolog.Nop())
	launcher2 := NewMockLauncher(zerolog.Nop())

	require.NoError(t, registry.Register("launcher1", launcher1))
	require.NoError(t, registry.Register("launcher2", launcher2))

	// Check available
	available = registry.Available()
	require.Len(t, available, 2, "should have 2 registered launchers")
	require.Contains(t, available, "launcher1", "should contain launcher1")
	require.Contains(t, available, "launcher2", "should contain launcher2")
}

// TestRegistryUnregister verifies unregistering launchers.
func TestRegistryUnregister(t *testing.T) {
	registry := NewRegistry()
	launcher := NewMockLauncher(zerolog.Nop())

	// Register
	require.NoError(t, registry.Register("test", launcher))

	// Verify it's registered
	_, err := registry.Get("test")
	require.NoError(t, err, "launcher should be registered")

	// Unregister
	err = registry.Unregister("test")
	require.NoError(t, err, "unregistration should succeed")

	// Verify it's gone
	_, err = registry.Get("test")
	require.Error(t, err, "launcher should no longer be registered")
}

// TestRegistryUnregisterUnknown verifies that unregistering unknown launcher fails.
func TestRegistryUnregisterUnknown(t *testing.T) {
	registry := NewRegistry()

	err := registry.Unregister("unknown")
	require.Error(t, err, "unregistering unknown launcher should fail")
	require.Contains(t, err.Error(), "not found",
		"error should indicate launcher was not found")
}

// TestDefaultRegistry verifies that the default registry has mock launcher.
func TestDefaultRegistry(t *testing.T) {
	// Default registry should have mock launcher pre-registered
	launcher, err := DefaultRegistry.Get("mock")
	require.NoError(t, err, "default registry should have mock launcher")
	require.NotNil(t, launcher, "mock launcher should not be nil")
	require.Equal(t, "mock", launcher.Name(), "launcher name should be mock")
}

// TestRegistryConcurrentAccess verifies thread-safe registry operations.
func TestRegistryConcurrentAccess(t *testing.T) {
	registry := NewRegistry()

	// Register a launcher
	launcher := NewMockLauncher(zerolog.Nop())
	require.NoError(t, registry.Register("test", launcher))

	// Concurrently read from registry
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			_, err := registry.Get("test")
			require.NoError(t, err)
			_ = registry.Available()
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestMockLauncherValidation verifies mock launcher configuration validation.
func TestMockLauncherValidation(t *testing.T) {
	launcher := NewMockLauncher(zerolog.Nop())

	tests := []struct {
		name      string
		cfg       Config
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid config",
			cfg: Config{
				Client:     "mock",
				DataDir:    "/tmp/test",
				BeaconPort: 4000,
				P2PPort:    9000,
			},
			shouldErr: false,
		},
		{
			name: "missing beacon port",
			cfg: Config{
				Client:  "mock",
				DataDir: "/tmp/test",
				P2PPort: 9000,
			},
			shouldErr: true,
			errMsg:    "beacon port required",
		},
		{
			name: "missing data dir",
			cfg: Config{
				Client:     "mock",
				BeaconPort: 4000,
				P2PPort:    9000,
			},
			shouldErr: true,
			errMsg:    "data directory required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := launcher.ValidateConfig(tt.cfg)
			if tt.shouldErr {
				require.Error(t, err, "validation should fail")
				require.Contains(t, err.Error(), tt.errMsg,
					"error message should contain expected text")
			} else {
				require.NoError(t, err, "validation should succeed")
			}
		})
	}
}

// TestMockLauncherLaunch verifies launching clients via the launcher.
func TestMockLauncherLaunch(t *testing.T) {
	launcher := NewMockLauncher(zerolog.Nop())

	cfg := Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	// Launch should succeed with valid config
	client, err := launcher.Launch(cfg)
	require.NoError(t, err, "launch should succeed")
	require.NotNil(t, client, "client should not be nil")

	// Invalid config should fail
	invalidCfg := Config{
		Client: "mock",
	}
	_, err = launcher.Launch(invalidCfg)
	require.Error(t, err, "launch with invalid config should fail")
}
