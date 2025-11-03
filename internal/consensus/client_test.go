package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/skipgraph-go/modules/throwable"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tctx := throwable.NewContext(ctx)

	// Test Start
	client.Start(tctx)

	// Test readiness - should become ready within 2 seconds
	select {
	case <-client.Ready():
		// Client is ready
	case <-time.After(2 * time.Second):
		t.Fatal("client should become ready within timeout")
	}

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

	// Test shutdown
	cancel()
	select {
	case <-client.Done():
		// Client stopped successfully
	case <-time.After(2 * time.Second):
		t.Fatal("client should stop within timeout")
	}
}

// TestClientStartAlreadyRunning verifies that starting a client multiple times
// causes a panic (irrecoverable error) as enforced by the Component pattern.
func TestClientStartAlreadyRunning(t *testing.T) {
	cfg := Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := NewMockClient(zerolog.Nop(), cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tctx := throwable.NewContext(ctx)

	// First start should succeed
	client.Start(tctx)

	// Component pattern throws an irrecoverable error (panic) on duplicate Start
	// This is by design to catch programming errors early
	defer func() {
		if r := recover(); r != nil {
			// Expected panic - test passes
			cancel()
			return
		}
		t.Fatal("expected panic when calling Start twice, but didn't get one")
	}()

	// Second start should panic
	client.Start(tctx)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tctx := throwable.NewContext(ctx)
	client.Start(tctx)

	// Wait for ready
	select {
	case <-client.Ready():
		// Ready
	case <-time.After(2 * time.Second):
		t.Fatal("client should become ready")
	}

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
	cancel()
	<-client.Done()
}

// TestClientMetricsWhenNotReady verifies that metrics cannot be retrieved
// when the client is not ready.
func TestClientMetricsWhenNotReady(t *testing.T) {
	cfg := Config{
		Client:     "mock",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := NewMockClient(zerolog.Nop(), cfg)

	// Attempt to get metrics without starting
	_, err := client.Metrics()
	require.Error(t, err, "metrics should fail when client is not ready")
	require.Contains(t, err.Error(), "not ready",
		"error should indicate client is not ready")
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

	tctx := throwable.NewContext(ctx)
	client.Start(tctx)

	// Wait for ready
	select {
	case <-client.Ready():
		// Ready
	case <-time.After(2 * time.Second):
		t.Fatal("client should become ready")
	}

	// Cancel context
	cancel()

	// Wait for shutdown
	select {
	case <-client.Done():
		// Stopped successfully
	case <-time.After(2 * time.Second):
		t.Fatal("client should stop after context cancellation")
	}
}
