package consensus_test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/skipgraph-go/modules"
	"github.com/thep2p/skipgraph-go/modules/component"
	"github.com/thep2p/skipgraph-go/modules/throwable"
)

// TestBaseClientLifecycle verifies the component.Manager lifecycle pattern.
func TestBaseClientLifecycle(t *testing.T) {
	cfg := consensus.Config{
		Client:     "test",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	// Track startup and shutdown execution
	startupCalled := false
	shutdownCalled := false

	// Create BaseClient with startup/shutdown logic
	client := consensus.NewBaseClient(
		zerolog.Nop(),
		cfg,
		component.WithStartupLogic(func(ctx modules.ThrowableContext) {
			startupCalled = true
		}),
		component.WithShutdownLogic(func() {
			shutdownCalled = true
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tctx := throwable.NewContext(ctx)

	// Start the client
	client.Start(tctx)

	// Verify startup was called
	require.True(t, startupCalled, "startup logic should be executed")

	// Wait for Ready - should be immediate since we have no blocking startup logic
	select {
	case <-client.Ready():
		// Client is ready
	case <-time.After(2 * time.Second):
		t.Fatal("client should become ready within timeout")
	}

	// Trigger shutdown
	cancel()

	// Wait for Done
	select {
	case <-client.Done():
		// Client stopped successfully
	case <-time.After(2 * time.Second):
		t.Fatal("client should complete shutdown within timeout")
	}

	// Verify shutdown was called
	require.True(t, shutdownCalled, "shutdown logic should be executed")
}

// TestBaseClientBeaconEndpoint verifies BeaconEndpoint returns correct URL.
func TestBaseClientBeaconEndpoint(t *testing.T) {
	cfg := consensus.Config{
		Client:     "test",
		DataDir:    t.TempDir(),
		BeaconPort: 5051,
		P2PPort:    9000,
	}

	client := consensus.NewBaseClient(zerolog.Nop(), cfg)

	endpoint := client.BeaconEndpoint()
	require.Equal(t, "http://127.0.0.1:5051", endpoint,
		"beacon endpoint should match configured port")
}

// TestBaseClientConfigAccess verifies Config() returns the correct configuration.
func TestBaseClientConfigAccess(t *testing.T) {
	cfg := consensus.Config{
		Client:     "test",
		DataDir:    "/path/to/data",
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := consensus.NewBaseClient(zerolog.Nop(), cfg)

	retrievedCfg := client.Config()
	require.Equal(t, cfg.Client, retrievedCfg.Client)
	require.Equal(t, cfg.DataDir, retrievedCfg.DataDir)
	require.Equal(t, cfg.BeaconPort, retrievedCfg.BeaconPort)
	require.Equal(t, cfg.P2PPort, retrievedCfg.P2PPort)
}

// TestBaseClientWithChildComponents demonstrates managing child components.
func TestBaseClientWithChildComponents(t *testing.T) {
	cfg := consensus.Config{
		Client:     "test",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	// Create a child component
	childReady := false
	childComponent := component.NewManager(
		zerolog.Nop(),
		component.WithStartupLogic(func(ctx modules.ThrowableContext) {
			// Simulate async initialization
			time.Sleep(100 * time.Millisecond)
			childReady = true
		}),
	)

	// Create BaseClient that manages the child
	client := consensus.NewBaseClient(
		zerolog.Nop(),
		cfg,
		component.WithComponent(childComponent),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tctx := throwable.NewContext(ctx)

	// Start the client
	client.Start(tctx)

	// The parent should only become ready after the child is ready
	select {
	case <-client.Ready():
		// Client is ready - child must be ready too
		require.True(t, childReady, "child should be ready when parent is ready")
	case <-time.After(2 * time.Second):
		t.Fatal("client should become ready within timeout")
	}

	// Cleanup
	cancel()
	<-client.Done()
}

// TestBaseClientMultipleStarts verifies that calling Start twice panics.
//
// This is the expected behavior from component.Manager to prevent
// programming errors.
func TestBaseClientMultipleStarts(t *testing.T) {
	cfg := consensus.Config{
		Client:     "test",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	client := consensus.NewBaseClient(zerolog.Nop(), cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tctx := throwable.NewContext(ctx)

	// First start should succeed
	client.Start(tctx)

	// Component pattern throws an irrecoverable error (panic) on duplicate Start
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

// TestBaseClientStartupLogicOrder verifies startup logic executes before readiness.
func TestBaseClientStartupLogicOrder(t *testing.T) {
	cfg := consensus.Config{
		Client:     "test",
		DataDir:    t.TempDir(),
		BeaconPort: 4000,
		P2PPort:    9000,
	}

	executionOrder := make([]string, 0)

	client := consensus.NewBaseClient(
		zerolog.Nop(),
		cfg,
		component.WithStartupLogic(func(ctx modules.ThrowableContext) {
			executionOrder = append(executionOrder, "startup")
			// Simulate some initialization work
			time.Sleep(50 * time.Millisecond)
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tctx := throwable.NewContext(ctx)

	client.Start(tctx)

	<-client.Ready()
	executionOrder = append(executionOrder, "ready")

	require.Equal(t, []string{"startup", "ready"}, executionOrder,
		"startup logic should execute before ready signal")

	cancel()
	<-client.Done()
}
