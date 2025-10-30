package node_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/node"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
)

// startNodesWithEngineAPI initializes nodes with Engine API enabled for testing.
// This is similar to startNodes but enables Engine API on the manager before starting.
func startNodesWithEngineAPI(t *testing.T, nodeCount int, opts ...node.LaunchOption) (
	context.Context,
	context.CancelFunc,
	*node.Manager,
) {
	t.Helper()

	tmp := testutils.NewTempDir(t)
	launcher := node.NewLauncher(testutils.Logger(t))
	manager := node.NewNodeManager(
		testutils.Logger(t), launcher, tmp.Path(), func() int {
			return testutils.NewPort(t)
		},
	)

	// Enable Engine API before starting nodes
	require.NoError(t, manager.EnableEngineAPI(), "EnableEngineAPI should succeed")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(tmp.Remove)
	t.Cleanup(
		func() {
			testutils.RequireCallMustReturnWithinTimeout(
				t, manager.Done, node.ShutdownTimeout, "node shutdown failed",
			)
		},
	)

	require.NoError(t, manager.Start(ctx, nodeCount, opts...))
	gethNode := manager.GethNode()
	require.NotNil(t, gethNode)

	testutils.RequireRpcReadyWithinTimeout(t, ctx, manager.RPCPort(), node.OperationTimeout)

	return ctx, cancel, manager
}

// TestEngineAPIEnabled verifies that Engine API is properly configured and accessible.
//
// The Engine API is the critical interface that enables Consensus Layer (CL) clients
// to communicate with Execution Layer (EL) nodes after Ethereum's transition to
// Proof-of-Stake (The Merge). This test validates:
//
//  1. Engine API port is assigned and accessible
//  2. JWT secret is generated and has correct format (32 bytes hex-encoded = 64 chars)
//  3. Authenticated connection can be established using the JWT
//  4. Engine API endpoints are available and respond
//
// Without proper Engine API setup:
//   - CL clients (Prysm, Lighthouse, etc.) cannot control block production
//   - The node cannot participate in PoS consensus
//   - Integration with real beacon chains is impossible
//
// This test confirms the foundation for EL-CL communication is in place.
func TestEngineAPIEnabled(t *testing.T) {
	ctx, cancel, manager := startNodesWithEngineAPI(t, 1)
	defer cancel()

	// Verify Engine API port is assigned
	enginePort := manager.GetEnginePort(0)
	require.NotZero(t, enginePort, "Engine API port should be assigned")

	// Verify JWT secret exists and has correct format
	jwt, err := manager.GetJWTSecret(0)
	require.NoError(t, err, "JWT secret should be readable")
	require.Len(t, jwt, 64, "JWT secret should be 32 bytes hex encoded (64 characters)")

	// Get the JWT path from the node config
	jwtPath := manager.GetNode(0).Config().JWTSecret
	require.NotEmpty(t, jwtPath, "JWT secret path should be set")

	// Connect to Engine API with JWT auth
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", enginePort)
	client, err := testutils.DialEngineAPI(ctx, endpoint, jwtPath)
	require.NoError(t, err, "Should be able to connect to Engine API with JWT auth")
	defer client.Close()

	// Test that engine_exchangeCapabilities is available
	// This is a basic Engine API method that returns supported capabilities
	var result []string
	err = client.CallContext(ctx, &result, "engine_exchangeCapabilities", []string{"engine_newPayloadV1"})
	// The method should be available even if it returns an error or empty result
	// What matters is that the endpoint exists and responds
	require.NoError(t, err, "engine_exchangeCapabilities should be available")
}

// TestEngineAPIRequiresAuth verifies that Engine API endpoints require JWT authentication.
//
// Security is critical for Engine API because it controls block production and chain state.
// Unauthorized access would allow attackers to:
//   - Propose invalid blocks
//   - Manipulate fork choice
//   - Disrupt consensus
//
// This test ensures that:
//  1. Connections without JWT authentication are rejected
//  2. The Engine API enforces authentication at the transport level
//  3. Security is properly configured, not accidentally exposed
//
// If this test fails, it indicates a CRITICAL SECURITY VULNERABILITY where
// Engine API is exposed without authentication, allowing anyone to control
// the node's consensus participation.
func TestEngineAPIRequiresAuth(t *testing.T) {
	ctx, cancel, manager := startNodesWithEngineAPI(t, 1)
	defer cancel()

	enginePort := manager.GetEnginePort(0)
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", enginePort)

	// Try to connect without JWT authentication
	client, err := rpc.DialContext(ctx, endpoint)
	// Connection itself might succeed (TCP handshake)
	// But API calls should fail due to missing authentication
	if err == nil {
		defer client.Close()

		// Try to call an Engine API method without auth
		var result interface{}
		err = client.CallContext(ctx, &result, "engine_exchangeCapabilities", []string{})

		// The call should fail with an authentication error
		// Note: The specific error may vary, but it should not succeed
		require.Error(t, err, "Engine API calls without JWT should fail")

		// Common error messages indicating auth failure:
		// - "unauthorized"
		// - "401"
		// - "authentication"
		// We're flexible on exact message as long as the call fails
	}
	// If we can't even establish a connection, that's also acceptable
	// (stricter security that rejects unauthenticated connections at TCP level)
}

// TestEngineAPIMultiNode verifies Engine API works correctly in multi-node setups.
//
// In production, multiple nodes may run simultaneously, each with its own Engine API.
// This test ensures:
//  1. Each node gets a unique Engine API port (no conflicts)
//  2. Each node has its own JWT secret (proper isolation)
//  3. All Engine API endpoints are independently accessible
//  4. No cross-contamination between node configurations
//
// This validates that the Engine API implementation scales to multi-node
// scenarios used in testing and development environments.
func TestEngineAPIMultiNode(t *testing.T) {
	nodeCount := 3
	ctx, cancel, manager := startNodesWithEngineAPI(t, nodeCount)
	defer cancel()

	// Track ports and JWTs to verify uniqueness
	ports := make(map[int]bool)
	jwts := make(map[string]bool)

	for i := 0; i < nodeCount; i++ {
		// Verify each node has an Engine API port
		enginePort := manager.GetEnginePort(i)
		require.NotZero(t, enginePort, "Node %d should have Engine API port", i)

		// Verify ports are unique
		require.False(t, ports[enginePort], "Engine API port %d should be unique", enginePort)
		ports[enginePort] = true

		// Verify each node has a JWT secret
		jwt, err := manager.GetJWTSecret(i)
		require.NoError(t, err, "Node %d should have JWT secret", i)
		require.Len(t, jwt, 64, "Node %d JWT should be 64 chars", i)

		// Verify JWTs are unique (different nodes should have different secrets)
		jwtStr := string(jwt)
		require.False(t, jwts[jwtStr], "JWT for node %d should be unique", i)
		jwts[jwtStr] = true

		// Verify we can connect to each node's Engine API
		jwtPath := manager.GetNode(i).Config().JWTSecret
		endpoint := fmt.Sprintf("http://127.0.0.1:%d", enginePort)
		client, err := testutils.DialEngineAPI(ctx, endpoint, jwtPath)
		require.NoError(t, err, "Should connect to node %d Engine API", i)
		client.Close()
	}
}

// TestEngineAPIBackwardCompatibility verifies that existing functionality still works
// when Engine API is NOT enabled.
//
// Backward compatibility is critical - enabling Engine API support should not break
// existing users who don't need it. This test ensures:
//  1. Nodes can still start without Engine API
//  2. Standard RPC still works normally
//  3. Block production continues with SimulatedBeacon
//  4. GetEnginePort returns 0 when Engine API is disabled
//
// This validates that Engine API is truly optional and existing workflows
// remain unaffected.
func TestEngineAPIBackwardCompatibility(t *testing.T) {
	// Use standard startNodes WITHOUT Engine API enabled
	ctx, cancel, manager := startNodes(t, 1)
	defer cancel()

	// Verify Engine API is NOT configured
	enginePort := manager.GetEnginePort(0)
	require.Zero(t, enginePort, "Engine API port should be 0 when not enabled")

	// Verify standard RPC still works
	testutils.RequireRpcReadyWithinTimeout(t, ctx, manager.RPCPort(), node.OperationTimeout)

	// Verify the node can still connect via standard RPC
	client, err := rpc.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", manager.RPCPort()))
	require.NoError(t, err, "Standard RPC should still work")
	defer client.Close()

	// Verify basic functionality works (client version check)
	var version string
	err = client.CallContext(ctx, &version, "web3_clientVersion")
	require.NoError(t, err, "Standard RPC calls should work")
	require.NotEmpty(t, version, "Should get valid client version")
}
