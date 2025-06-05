package testutils

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

// TempDir is a utility struct for managing temporary directories in tests.
type TempDir struct {
	t    *testing.T
	path string
}

// NewTempDir creates a temporary directory and registers cleanup with the test.
func NewTempDir(t *testing.T) *TempDir {
	t.Helper()
	path, err := os.MkdirTemp("", "geth-localnet-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(path), "failed to remove temp dir: "+path)
	})
	return &TempDir{t: t, path: path}
}

// Path returns the path of the temporary directory.
func (td *TempDir) Path() string {
	return td.path
}
