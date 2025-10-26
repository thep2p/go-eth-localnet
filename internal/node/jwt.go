package node

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateJWTSecret creates a 32-byte random JWT secret for Engine API auth.
// The secret is written to a file named "jwt.hex" in the specified data directory
// with 0600 permissions for security. Returns the path to the JWT secret file.
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
