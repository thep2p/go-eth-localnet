package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TestClientLifecycle verifies the complete lifecycle of a CL client.
//
// This test ensures that clients can be started, become ready, and
// be stopped gracefully following the expected lifecycle pattern.
func TestClientLifecycle(t *testing.T) {
	cfg := Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := NewMockClient(zerolog.Nop(), cfg)
	ctx := context.Background()

	// Test Start
	require.NoError(t, client.Start(ctx), "client should start successfully")
	require.True(t, client.IsRunning(), "client should be running after start")

	// Test readiness - should become ready within 2 seconds
	require.Eventually(t, client.Ready, 2*time.Second, 100*time.Millisecond,
		"client should become ready within timeout")

	// Test BeaconEndpoint
	endpoint := client.BeaconEndpoint()
	require.Contains(t, endpoint, "http://127.0.0.1:4000",
		"beacon endpoint should match configured port")

	// Test Metrics
	metrics, err := client.Metrics()
	require.NoError(t, err, "metrics should be retrievable")
	require.NotNil(t, metrics, "metrics should not be nil")
	require.Greater(t, metrics.CurrentSlot, uint64(0),
		"current slot should be greater than 0")

	// Test Stop
	require.NoError(t, client.Stop(), "client should stop successfully")
	require.NoError(t, client.Wait(), "wait should complete successfully")
}

// TestClientStartAlreadyRunning verifies that starting an already running
// client returns an error.
func TestClientStartAlreadyRunning(t *testing.T) {
	cfg := Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := NewMockClient(zerolog.Nop(), cfg)
	ctx := context.Background()

	require.NoError(t, client.Start(ctx), "first start should succeed")

	// Attempt to start again
	err := client.Start(ctx)
	require.Error(t, err, "second start should fail")
	require.Contains(t, err.Error(), "already running",
		"error should indicate client is already running")

	// Cleanup
	require.NoError(t, client.Stop())
	require.NoError(t, client.Wait())
}

// TestClientStopNotRunning verifies that stopping a non-running client
// is safe and returns no error.
func TestClientStopNotRunning(t *testing.T) {
	cfg := Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := NewMockClient(zerolog.Nop(), cfg)

	// Stop without starting should be safe
	require.NoError(t, client.Stop(), "stopping non-running client should succeed")
}

// TestClientMetricsProgression verifies that metrics update over time.
func TestClientMetricsProgression(t *testing.T) {
	cfg := Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := NewMockClient(zerolog.Nop(), cfg)
	ctx := context.Background()

	require.NoError(t, client.Start(ctx))
	require.Eventually(t, client.Ready, 2*time.Second, 100*time.Millisecond)

	// Get initial metrics
	metrics1, err := client.Metrics()
	require.NoError(t, err)
	initialSlot := metrics1.CurrentSlot

	// Get metrics again - current slot should have incremented
	metrics2, err := client.Metrics()
	require.NoError(t, err)
	require.Greater(t, metrics2.CurrentSlot, initialSlot,
		"current slot should increment between metric calls")

	// Cleanup
	require.NoError(t, client.Stop())
	require.NoError(t, client.Wait())
}

// TestClientMetricsWhenNotRunning verifies that metrics cannot be retrieved
// when the client is not running.
func TestClientMetricsWhenNotRunning(t *testing.T) {
	cfg := Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := NewMockClient(zerolog.Nop(), cfg)

	// Attempt to get metrics without starting
	_, err := client.Metrics()
	require.Error(t, err, "metrics should fail when client is not running")
	require.Contains(t, err.Error(), "not running",
		"error should indicate client is not running")
}

// TestClientValidatorKeys verifies that validator keys are correctly stored
// and retrieved.
func TestClientValidatorKeys(t *testing.T) {
	expectedKeys := []string{"key1", "key2", "key3"}
	cfg := Config{
		Client:        "mock",
		DataDir:       t.TempDir(),
		BeaconPort:    4000,
		P2PPort:       9000,
		ValidatorKeys: expectedKeys,
	}

	client := NewMockClient(zerolog.Nop(), cfg)

	keys := client.ValidatorKeys()
	require.Equal(t, expectedKeys, keys,
		"validator keys should match configuration")
}

// TestClientContextCancellation verifies that client stops when context is cancelled.
func TestClientContextCancellation(t *testing.T) {
	cfg := Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := NewMockClient(zerolog.Nop(), cfg)
	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, client.Start(ctx))
	require.Eventually(t, client.Ready, 2*time.Second, 100*time.Millisecond)

	// Cancel context
	cancel()

	// Stop and wait should succeed
	require.NoError(t, client.Stop())
	require.NoError(t, client.Wait())
}
