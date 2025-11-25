package prysm_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/consensus"
	"github.com/thep2p/go-eth-localnet/internal/consensus/prysm"
	"github.com/thep2p/go-eth-localnet/internal/unittest"
	"github.com/thep2p/skipgraph-go/modules/throwable"
	skipgraphtest "github.com/thep2p/skipgraph-go/unittest"
)

// TestClientLifecycle verifies basic lifecycle management of Prysm client.
func TestClientLifecycle(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
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

	client, err := prysm.NewClient(logger, cfg)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Create cancellable context for component lifecycle
	mockCtx := skipgraphtest.NewMockThrowableContext(t)
	ctx := throwable.NewContext(mockCtx)

	// Start the client - should succeed and become ready
	client.Start(ctx)

	// Wait for client to become ready
	skipgraphtest.RequireAllReady(t, client)

	// Cancel context to trigger graceful shutdown
	mockCtx.Cancel()

	// Wait for client to finish shutdown
	skipgraphtest.RequireAllDone(t, client)
}

// TestClientValidation verifies configuration validation.
func TestClientValidation(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)

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
			wantError: "DataDir",
		},
		{
			name: "missing beacon port",
			cfg: consensus.Config{
				DataDir:        "/tmp/prysm",
				P2PPort:        9000,
				EngineEndpoint: "http://127.0.0.1:8551",
				JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
			},
			wantError: "BeaconPort",
		},
		{
			name: "missing p2p port",
			cfg: consensus.Config{
				DataDir:        "/tmp/prysm",
				BeaconPort:     4000,
				EngineEndpoint: "http://127.0.0.1:8551",
				JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
			},
			wantError: "P2PPort",
		},
		{
			name: "missing engine endpoint",
			cfg: consensus.Config{
				DataDir:    "/tmp/prysm",
				BeaconPort: 4000,
				P2PPort:    9000,
				JWTSecret:  []byte("test-jwt-secret-32-bytes-long!!"),
			},
			wantError: "EngineEndpoint",
		},
		{
			name: "missing jwt secret",
			cfg: consensus.Config{
				DataDir:        "/tmp/prysm",
				BeaconPort:     4000,
				P2PPort:        9000,
				EngineEndpoint: "http://127.0.0.1:8551",
			},
			wantError: "JWTSecret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := prysm.NewClient(logger, tt.cfg)
			require.Error(t, err, "Expected validation error from NewClient")
			require.Nil(t, client, "Client should be nil when validation fails")
			require.Contains(t, err.Error(), tt.wantError, "Error should contain expected message")
		})
	}
}

// TestClientMultipleStarts verifies that starting an already-started client panics.
// This is enforced by the Component pattern - Start must only be called once.
func TestClientMultipleStarts(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
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

	client, err := prysm.NewClient(logger, cfg)
	require.NoError(t, err)

	errThrown := make(chan interface{})
	ctx := throwable.NewContext(skipgraphtest.NewMockThrowableContext(t, skipgraphtest.WithThrowLogic(func(_ error) {
		close(errThrown)
	})))

	client.Start(ctx)
	select {
	case <-errThrown:
		t.Fatal("Unexpected error during first start")
	case <-time.After(100 * time.Millisecond):
		// No error thrown, proceed
	}

	// Second start should panic
	client.Start(ctx)
	unittest.ChannelMustCloseWithinTimeout(t, errThrown, 100*time.Millisecond, "Expected error on second start")
}

// TestClientAPIs verifies API URL generation.
func TestClientAPIs(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	beaconPort := unittest.NewPort(t)
	p2pPort := unittest.NewPort(t)

	cfg := consensus.Config{
		DataDir:        filepath.Join(tmp.Path(), "prysm"),
		ChainID:        1337,
		GenesisTime:    time.Now(),
		BeaconPort:     beaconPort,
		P2PPort:        p2pPort,
		RPCPort:        unittest.NewPort(t),
		EngineEndpoint: "http://127.0.0.1:8551",
		JWTSecret:      []byte("test-jwt-secret-32-bytes-long!!"),
	}

	client, err := prysm.NewClient(logger, cfg)
	require.NoError(t, err)

	// Verify API URLs are correctly formatted with the configured ports
	expectedBeaconAPI := fmt.Sprintf("http://127.0.0.1:%d", beaconPort)
	expectedP2P := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", p2pPort)

	require.Equal(t, expectedBeaconAPI, client.BeaconAPIURL(), "beacon api url should match expected format")
	require.Equal(t, expectedP2P, client.P2PAddress(), "p2p address should match expected format")
}

// TestClientWithValidators verifies validator initialization.
func TestClientWithValidators(t *testing.T) {
	t.Parallel()

	logger := unittest.Logger(t)
	tmp := unittest.NewTempDir(t)
	t.Cleanup(tmp.Remove)

	validatorKeys := []string{
		"test-key-1",
		"test-key-2",
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
		ValidatorKeys:  validatorKeys,
		FeeRecipient:   common.HexToAddress("0x1234567890123456789012345678901234567890"),
	}

	client, err := prysm.NewClient(logger, cfg)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Create cancellable context for component lifecycle
	mockCtx := skipgraphtest.NewMockThrowableContext(t)
	ctx := throwable.NewContext(mockCtx)

	// Start the client with validators - should succeed
	client.Start(ctx)

	// Wait for client to become ready
	skipgraphtest.RequireAllReady(t, client)

	// Cancel context to trigger graceful shutdown
	mockCtx.Cancel()

	// Wait for client to finish shutdown
	skipgraphtest.RequireAllDone(t, client)
}
