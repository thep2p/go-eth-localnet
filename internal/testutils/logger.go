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
