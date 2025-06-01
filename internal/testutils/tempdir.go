package testutils

import (
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
	t.Cleanup(func() { os.RemoveAll(path) })
	return &TempDir{t: t, path: path}
}

func (td *TempDir) Path() string {
	return td.path
}
