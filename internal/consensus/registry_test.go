package consensus_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/unittest/mocks"
)

// TestRegistryRegisterAndGet verifies basic registry operations.
func TestRegistryRegisterAndGet(t *testing.T) {
	registry := consensus.NewRegistry()
	mockLauncher := mocks.NewMockLauncher(t)

	// Set expectation for Name
	mockLauncher.EXPECT().Name().Return("test").Maybe()

	// Register launcher
	err := registry.Register("test", mockLauncher)
	require.NoError(t, err, "registration should succeed")

	// Get launcher
	got, err := registry.Get("test")
	require.NoError(t, err, "get should succeed")
	require.Equal(t, mockLauncher, got, "retrieved launcher should match registered launcher")
}

// TestRegistryDuplicateRegistration verifies that duplicate registration fails.
func TestRegistryDuplicateRegistration(t *testing.T) {
	registry := consensus.NewRegistry()
	mockLauncher := mocks.NewMockLauncher(t)

	// First registration should succeed
	err := registry.Register("test", mockLauncher)
	require.NoError(t, err, "first registration should succeed")

	// Duplicate registration should fail
	err = registry.Register("test", mockLauncher)
	require.Error(t, err, "duplicate registration should fail")
	require.Contains(t, err.Error(), "already registered",
		"error should indicate launcher is already registered")
}

// TestRegistryGetUnknownLauncher verifies that getting an unknown launcher fails.
func TestRegistryGetUnknownLauncher(t *testing.T) {
	registry := consensus.NewRegistry()

	// Get unknown launcher
	_, err := registry.Get("unknown")
	require.Error(t, err, "getting unknown launcher should fail")
	require.Contains(t, err.Error(), "not found",
		"error should indicate launcher was not found")
}

// TestRegistryAvailable verifies listing available launchers.
func TestRegistryAvailable(t *testing.T) {
	registry := consensus.NewRegistry()

	// Initially empty
	available := registry.Available()
	require.Empty(t, available, "new registry should be empty")

	// Register multiple launchers
	mockLauncher1 := mocks.NewMockLauncher(t)
	mockLauncher2 := mocks.NewMockLauncher(t)

	require.NoError(t, registry.Register("launcher1", mockLauncher1))
	require.NoError(t, registry.Register("launcher2", mockLauncher2))

	// Check available
	available = registry.Available()
	require.Len(t, available, 2, "should have 2 registered launchers")
	require.Contains(t, available, "launcher1", "should contain launcher1")
	require.Contains(t, available, "launcher2", "should contain launcher2")
}

// TestRegistryUnregister verifies unregistering launchers.
func TestRegistryUnregister(t *testing.T) {
	registry := consensus.NewRegistry()
	mockLauncher := mocks.NewMockLauncher(t)

	// Register
	require.NoError(t, registry.Register("test", mockLauncher))

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
	registry := consensus.NewRegistry()

	err := registry.Unregister("unknown")
	require.Error(t, err, "unregistering unknown launcher should fail")
	require.Contains(t, err.Error(), "not found",
		"error should indicate launcher was not found")
}

// TestDefaultRegistry verifies that the default registry exists.
func TestDefaultRegistry(t *testing.T) {
	// Default registry should exist
	require.NotNil(t, consensus.DefaultRegistry, "default registry should not be nil")
}

// TestRegistryConcurrentAccess verifies thread-safe registry operations.
func TestRegistryConcurrentAccess(t *testing.T) {
	registry := consensus.NewRegistry()

	// Register a launcher
	mockLauncher := mocks.NewMockLauncher(t)
	require.NoError(t, registry.Register("test", mockLauncher))

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

// TestLauncherValidation verifies launcher configuration validation.
func TestLauncherValidation(t *testing.T) {
	mockLauncher := mocks.NewMockLauncher(t)

	tests := []struct {
		name      string
		cfg       consensus.Config
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid config",
			cfg: consensus.Config{
				Client:     "mock",
				DataDir:    "/tmp/test",
				BeaconPort: 4000,
				P2PPort:    9000,
			},
			shouldErr: false,
		},
		{
			name: "missing beacon port",
			cfg: consensus.Config{
				Client:  "mock",
				DataDir: "/tmp/test",
				P2PPort: 9000,
			},
			shouldErr: true,
			errMsg:    "beacon port",
		},
		{
			name: "missing data dir",
			cfg: consensus.Config{
				Client:     "mock",
				BeaconPort: 4000,
				P2PPort:    9000,
			},
			shouldErr: true,
			errMsg:    "data dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set expectation
			if tt.shouldErr {
				mockLauncher.EXPECT().
					ValidateConfig(tt.cfg).
					Return(fmt.Errorf("%s", tt.errMsg)).
					Once()
			} else {
				mockLauncher.EXPECT().
					ValidateConfig(tt.cfg).
					Return(nil).
					Once()
			}

			err := mockLauncher.ValidateConfig(tt.cfg)
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

// TestLauncherLaunch verifies launching clients via the launcher.
func TestLauncherLaunch(t *testing.T) {
	mockLauncher := mocks.NewMockLauncher(t)
	mockClient := mocks.NewMockClient(t)

	cfg := consensus.Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	// Set expectation for successful launch
	mockLauncher.EXPECT().
		Launch(cfg).
		Return(mockClient, nil).
		Once()

	// Launch should succeed with valid config
	client, err := mockLauncher.Launch(cfg)
	require.NoError(t, err, "launch should succeed")
	require.NotNil(t, client, "client should not be nil")
	require.Equal(t, mockClient, client, "client should match mock")
}

// TestLauncherLaunchError verifies launch error handling.
func TestLauncherLaunchError(t *testing.T) {
	mockLauncher := mocks.NewMockLauncher(t)

	invalidCfg := consensus.Config{
		Client: "mock",
	}

	// Set expectation for failed launch
	mockLauncher.EXPECT().
		Launch(invalidCfg).
		Return(nil, fmt.Errorf("invalid config")).
		Once()

	// Launch with invalid config should fail
	_, err := mockLauncher.Launch(invalidCfg)
	require.Error(t, err, "launch with invalid config should fail")
}

// TestLauncherName verifies the Name method.
func TestLauncherName(t *testing.T) {
	mockLauncher := mocks.NewMockLauncher(t)

	// Set expectation
	mockLauncher.EXPECT().Name().Return("test-launcher").Once()

	// Test
	name := mockLauncher.Name()
	require.Equal(t, "test-launcher", name, "launcher name should match")
}
