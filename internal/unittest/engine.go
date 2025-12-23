package unittest

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
)

// DialEngineAPI creates an authenticated RPC client for Engine API.
// It reads the JWT secret from the specified path and uses it to authenticate
// the connection. The JWT file should contain a 32-byte hex-encoded secret.
func DialEngineAPI(ctx context.Context, endpoint string, jwtPath string) (*rpc.Client, error) {
	jwt, err := os.ReadFile(jwtPath)
	if err != nil {
		return nil, fmt.Errorf("read jwt: %w", err)
	}

	// Remove any whitespace/newlines from JWT
	jwtHex := strings.TrimSpace(string(jwt))
	jwtBytes, err := hex.DecodeString(jwtHex)
	if err != nil {
		return nil, fmt.Errorf("decode jwt: %w", err)
	}

	// jwt secret must be exactly 32 bytes
	if len(jwtBytes) != 32 {
		return nil, fmt.Errorf("jwt secret must be 32 bytes, got %d", len(jwtBytes))
	}

	// Convert to [32]byte array
	var secret [32]byte
	copy(secret[:], jwtBytes)

	// Create client with JWT auth
	return rpc.DialOptions(ctx, endpoint,
		rpc.WithHTTPAuth(node.NewJWTAuth(secret)))
}
