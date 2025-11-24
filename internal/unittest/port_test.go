package unittest_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thep2p/go-eth-localnet/internal/unittest"
)

// TestNewPort_UniqueAcrossCalls ensures ports are unique across calls.
func TestNewPort_UniqueAcrossCalls(t *testing.T) {
	p1 := unittest.NewPort(t)
	p2 := unittest.NewPort(t)
	require.NotEqual(t, p1, p2, "expected different ports, but got the same: %d", p1)
}
