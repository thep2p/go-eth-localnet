package unittest

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

// NewTempDir creates a temporary directory for tests. Call Remove
// after all resources referencing the directory have been closed.
func NewTempDir(t *testing.T) *TempDir {
	t.Helper()
	path, err := os.MkdirTemp("", "geth-localnet-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return &TempDir{t: t, path: path}
}

// Path returns the path of the temporary directory.
func (td *TempDir) Path() string {
	return td.path
}

// Remove deletes the temporary directory.
func (td *TempDir) Remove() {
	td.t.Helper()
	require.NoError(td.t, os.RemoveAll(td.path), "failed to remove temp dir: "+td.path)
}
