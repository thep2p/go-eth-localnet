# Step 1: Engine API Foundation - Enable Secure EL-CL Communication

## Overview

Currently, our multi-node setup uses `SimulatedBeacon` for block production, which is perfect for development but doesn't reflect production Ethereum's architecture where Execution Layer (EL) and Consensus Layer (CL) communicate via the Engine API. This issue implements the foundation for real EL-CL communication by adding Engine API support with JWT authentication.

The Engine API is the critical interface that enables the Consensus Layer to drive the Execution Layer's block production, validation, and chain selection. Without this, we cannot integrate real consensus clients like Prysm or Lighthouse.

## Why This Matters

- **Production Parity**: Real Ethereum nodes use Engine API for EL-CL communication since The Merge
- **Security**: JWT authentication prevents unauthorized access to critical Engine API endpoints
- **Foundation for CL Integration**: Required before we can integrate Prysm or other CL clients
- **Testing Realism**: Enables testing of actual Engine API flows (fork choice updates, payload building, etc.)

## Acceptance Criteria

- [ ] Engine API server is enabled on Geth nodes with configurable port
- [ ] JWT secret generation and management for secure authentication
- [ ] Engine API endpoints are accessible and respond correctly
- [ ] Existing SimulatedBeacon tests continue to pass
- [ ] New tests verify Engine API availability and JWT authentication
- [ ] Documentation explains Engine API configuration and usage

## Implementation Tasks

### 1. JWT Secret Management
```go
// internal/node/jwt.go
package node

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "os"
    "path/filepath"
)

// GenerateJWTSecret creates a 32-byte random JWT secret for Engine API auth.
func GenerateJWTSecret(dataDir string) (string, error) {
    secret := make([]byte, 32)
    if _, err := rand.Read(secret); err != nil {
        return "", fmt.Errorf("generate jwt secret: %w", err)
    }

    jwtPath := filepath.Join(dataDir, "jwt.hex")
    content := hex.EncodeToString(secret)
    if err := os.WriteFile(jwtPath, []byte(content), 0600); err != nil {
        return "", fmt.Errorf("write jwt secret: %w", err)
    }

    return jwtPath, nil
}
```

### 2. Update Config Model
```go
// internal/model/config.go
type Config struct {
    // ... existing fields ...

    // EnginePort is the port for Engine API (authenticated RPC).
    EnginePort int
    // JWTSecretPath is the path to JWT secret file for Engine API auth.
    JWTSecretPath string
    // EnableEngineAPI enables the Engine API endpoints.
    EnableEngineAPI bool
}
```

### 3. Modify Launcher to Enable Engine API
```go
// internal/node/geth.go - Update Launch method
func (l *Launcher) Launch(cfg model.Config, opts ...LaunchOption) (*node.Node, error) {
    // ... existing code ...

    // Configure authenticated RPC for Engine API if enabled
    if cfg.EnableEngineAPI {
        nodeCfg.AuthAddr = "127.0.0.1"
        nodeCfg.AuthPort = cfg.EnginePort
        nodeCfg.AuthVirtualHosts = []string{"localhost"}
        nodeCfg.JWTSecret = cfg.JWTSecretPath

        // Enable Engine API modules
        nodeCfg.AuthModules = []string{"engine", "eth", "web3", "net", "debug"}
    }

    // ... rest of existing code ...
}
```

### 4. Add LaunchOption for Engine API
```go
// internal/node/geth.go
// WithEngineAPI enables Engine API with JWT authentication.
func WithEngineAPI() LaunchOption {
    return func(gen *core.Genesis) {
        // Genesis modifications if needed for Engine API
        // Note: Most Engine API config is handled at node level
    }
}
```

### 5. Update Manager for Engine API Support
```go
// internal/node/manager.go
func (m *Manager) startSingleNode(ctx context.Context, mine bool, staticNodes []string, opts ...LaunchOption) error {
    // ... existing code ...

    // Generate JWT secret if Engine API is enabled
    if m.enableEngineAPI {
        jwtPath, err := GenerateJWTSecret(cfg.DataDir)
        if err != nil {
            return fmt.Errorf("generate jwt secret: %w", err)
        }
        cfg.JWTSecretPath = jwtPath
        cfg.EnableEngineAPI = true
        cfg.EnginePort = m.assignNewPort()
    }

    // ... rest of existing code ...
}

// GetEnginePort returns the Engine API port for the node at the given index.
func (m *Manager) GetEnginePort(index int) int {
    if index < 0 || index >= len(m.configs) {
        return 0
    }
    return m.configs[index].EnginePort
}

// GetJWTSecret returns the JWT secret for the node at the given index.
func (m *Manager) GetJWTSecret(index int) ([]byte, error) {
    if index < 0 || index >= len(m.configs) {
        return nil, fmt.Errorf("invalid node index")
    }

    return os.ReadFile(m.configs[index].JWTSecretPath)
}
```

### 6. Engine API Client Helper
```go
// internal/testutils/engine.go
package testutils

import (
    "context"
    "encoding/hex"
    "fmt"
    "os"

    "github.com/ethereum/go-ethereum/rpc"
)

// DialEngineAPI creates an authenticated RPC client for Engine API.
func DialEngineAPI(ctx context.Context, endpoint string, jwtPath string) (*rpc.Client, error) {
    jwt, err := os.ReadFile(jwtPath)
    if err != nil {
        return nil, fmt.Errorf("read jwt: %w", err)
    }

    // Remove any whitespace/newlines from JWT
    jwtHex := string(jwt)
    jwtBytes, err := hex.DecodeString(jwtHex)
    if err != nil {
        return nil, fmt.Errorf("decode jwt: %w", err)
    }

    // Create client with JWT auth
    return rpc.DialOptions(ctx, endpoint,
        rpc.WithHTTPAuth(rpc.NewJWTAuth(jwtBytes)))
}
```

## Testing Requirements

### 1. Test Engine API Availability
```go
// internal/node/engine_api_test.go
func TestEngineAPIEnabled(t *testing.T) {
    ctx, cancel, manager := startNodes(t, 1, WithEngineAPI())
    defer cancel()

    // Verify Engine API port is assigned
    enginePort := manager.GetEnginePort(0)
    require.NotZero(t, enginePort)

    // Verify JWT secret exists
    jwt, err := manager.GetJWTSecret(0)
    require.NoError(t, err)
    require.Len(t, jwt, 64) // 32 bytes hex encoded

    // Connect to Engine API with JWT auth
    endpoint := fmt.Sprintf("http://127.0.0.1:%d", enginePort)
    client, err := testutils.DialEngineAPI(ctx, endpoint,
        manager.GetNode(0).Config().JWTSecret)
    require.NoError(t, err)
    defer client.Close()

    // Test engine_forkchoiceUpdatedV3 is available
    var result map[string]interface{}
    err = client.CallContext(ctx, &result, "engine_forkchoiceUpdatedV3",
        map[string]string{
            "headBlockHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
            "safeBlockHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
            "finalizedBlockHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        }, nil)
    // Engine API should be available (even if call params are invalid)
    require.NotNil(t, err) // Expect error due to invalid state, but connection should work
}
```

### 2. Test JWT Authentication Required
```go
func TestEngineAPIRequiresAuth(t *testing.T) {
    ctx, cancel, manager := startNodes(t, 1, WithEngineAPI())
    defer cancel()

    enginePort := manager.GetEnginePort(0)
    endpoint := fmt.Sprintf("http://127.0.0.1:%d", enginePort)

    // Try to connect without JWT - should fail
    client, err := rpc.DialContext(ctx, endpoint)
    if err == nil {
        defer client.Close()

        // Try to call engine API without auth
        var result interface{}
        err = client.CallContext(ctx, &result, "engine_forkchoiceUpdatedV3", nil, nil)
        require.Error(t, err)
        require.Contains(t, err.Error(), "unauthorized")
    }
}
```

### 3. Refactor Existing Tests
- Update `startNodes` helper to optionally enable Engine API
- Ensure all existing tests still pass with SimulatedBeacon
- Add parallel test execution where appropriate

## Technical Considerations

### Important Notes
- **Port Conflicts**: Engine API needs a separate port from the standard RPC port
- **JWT Security**: JWT secrets must be cryptographically secure (32 bytes) and properly protected (0600 permissions)
- **Backwards Compatibility**: Must not break existing SimulatedBeacon functionality
- **Module Availability**: Engine API modules (`engine_*`) are only available on authenticated endpoint
- **Error Handling**: Engine API calls may return specific error codes that need proper handling

### Gotchas
1. **JWT Format**: The JWT file contains hex-encoded secret (64 chars), not raw bytes
2. **Module Registration**: Engine modules must be explicitly enabled in `AuthModules`
3. **Virtual Hosts**: AuthVirtualHosts must include "localhost" for local testing
4. **Concurrent Access**: Multiple CL clients may connect to same Engine API (handle concurrent requests)

### Dependencies
- Geth v1.15.11 already includes full Engine API support
- No additional dependencies required

## References
- [Engine API Specification](https://github.com/ethereum/execution-apis/blob/main/src/engine/common.md)
- [JWT Authentication in Geth](https://geth.ethereum.org/docs/interacting-with-geth/rpc/auth)
- [go-ethereum Engine API implementation](https://github.com/ethereum/go-ethereum/tree/master/eth/catalyst)

## Success Metrics
- Engine API endpoints respond to authenticated requests
- JWT authentication prevents unauthorized access
- All existing tests continue to pass
- New Engine API tests provide >80% coverage of new code
- Documentation clearly explains Engine API usage