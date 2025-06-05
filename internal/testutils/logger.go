// Package testutils provides testing utilities and helper functions for Ethereum node testing.
// It includes functionality for managing test node instances, configuring static peer connections,
// and setting up test network environments. This package is intended for testing purposes only
// and should not be used in production environments.
package testutils

import (
	"github.com/rs/zerolog"
	"os"
	"testing"
)

// Logger returns a zerolog.Logger configured for testing.
func Logger(t *testing.T) zerolog.Logger {
	t.Helper()
	return zerolog.New(os.Stdout).Level(zerolog.DebugLevel)
}
