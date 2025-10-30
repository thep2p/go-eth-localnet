package node

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/testutils"
)

// TestGenerateJWTSecret_Success validates that JWT secret generation works
// correctly when the data directory exists. This tests the happy path and
// ensures the JWT file is created with proper content and permissions.
func TestGenerateJWTSecret_Success(t *testing.T) {
	tempDir := testutils.NewTempDir(t)
	defer tempDir.Remove()

	jwtPath, err := GenerateJWTSecret(tempDir.Path())
	require.NoError(t, err, "GenerateJWTSecret should succeed")
	require.NotEmpty(t, jwtPath, "JWT path should not be empty")

	// Verify the JWT file exists
	require.FileExists(t, jwtPath, "JWT file should exist")

	// Verify file permissions (0600)
	fileInfo, err := os.Stat(jwtPath)
	require.NoError(t, err, "Should be able to stat JWT file")
	require.Equal(t, os.FileMode(0600), fileInfo.Mode().Perm(), "JWT file should have 0600 permissions")

	// Verify content is valid hex-encoded 32-byte secret (64 hex characters)
	content, err := os.ReadFile(jwtPath)
	require.NoError(t, err, "Should be able to read JWT file")
	require.Len(t, content, 64, "JWT secret should be 64 hex characters (32 bytes)")

	// Verify content is valid hex
	_, err = hex.DecodeString(string(content))
	require.NoError(t, err, "JWT content should be valid hex")

	// Verify the path points to jwt.hex
	require.Equal(t, JWTFileName, filepath.Base(jwtPath), "JWT file should be named jwt.hex")
}

// TestGenerateJWTSecret_CreatesDirectory validates that GenerateJWTSecret
// creates the parent directory if it doesn't exist. This is important for
// ensuring the function can initialize new node data directories.
func TestGenerateJWTSecret_CreatesDirectory(t *testing.T) {
	tempDir := testutils.NewTempDir(t)
	defer tempDir.Remove()
	dataDir := filepath.Join(tempDir.Path(), "nonexistent", "nested", "dir")

	// Verify directory doesn't exist yet
	_, err := os.Stat(dataDir)
	require.True(t, os.IsNotExist(err), "Directory should not exist initially")

	jwtPath, err := GenerateJWTSecret(dataDir)
	require.NoError(t, err, "GenerateJWTSecret should create parent directory")

	// Verify the directory was created
	dirInfo, err := os.Stat(dataDir)
	require.NoError(t, err, "Directory should exist after GenerateJWTSecret")
	require.True(t, dirInfo.IsDir(), "Path should be a directory")

	// Verify JWT file was created in the new directory
	require.FileExists(t, jwtPath, "JWT file should exist in new directory")
}

// TestGenerateJWTSecret_UniqueSecrets validates that multiple calls to
// GenerateJWTSecret produce different secrets. This ensures cryptographic
// randomness and prevents predictable secrets.
func TestGenerateJWTSecret_UniqueSecrets(t *testing.T) {
	tempDir1 := testutils.NewTempDir(t)
	defer tempDir1.Remove()
	tempDir2 := testutils.NewTempDir(t)
	defer tempDir2.Remove()

	jwtPath1, err := GenerateJWTSecret(tempDir1.Path())
	require.NoError(t, err, "First GenerateJWTSecret should succeed")

	jwtPath2, err := GenerateJWTSecret(tempDir2.Path())
	require.NoError(t, err, "Second GenerateJWTSecret should succeed")

	content1, err := os.ReadFile(jwtPath1)
	require.NoError(t, err, "Should read first JWT file")

	content2, err := os.ReadFile(jwtPath2)
	require.NoError(t, err, "Should read second JWT file")

	require.NotEqual(t, content1, content2, "JWT secrets should be unique")
}

// TestGenerateJWTSecret_Idempotency validates that calling GenerateJWTSecret
// multiple times on the same directory overwrites the previous secret.
// This is important for regenerating secrets when needed.
func TestGenerateJWTSecret_Idempotency(t *testing.T) {
	tempDir := testutils.NewTempDir(t)
	defer tempDir.Remove()

	// Generate first JWT
	jwtPath1, err := GenerateJWTSecret(tempDir.Path())
	require.NoError(t, err, "First GenerateJWTSecret should succeed")
	content1, err := os.ReadFile(jwtPath1)
	require.NoError(t, err, "Should read first JWT file")

	// Generate second JWT in same directory
	jwtPath2, err := GenerateJWTSecret(tempDir.Path())
	require.NoError(t, err, "Second GenerateJWTSecret should succeed")
	content2, err := os.ReadFile(jwtPath2)
	require.NoError(t, err, "Should read second JWT file")

	// Paths should be the same
	require.Equal(t, jwtPath1, jwtPath2, "JWT paths should be identical")

	// Content should be different (new random secret)
	require.NotEqual(t, content1, content2, "JWT secrets should be different after regeneration")
}
