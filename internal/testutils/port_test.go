package testutils

import "testing"

// TestNewPort_UniqueAcrossCalls ensures ports are unique across calls.
func TestNewPort_UniqueAcrossCalls(t *testing.T) {
	p1 := NewPort(t)
	p2 := NewPort(t)
	require.NotEqual(t, p1, p2, "expected different ports, but got the same: %d", p1)
}
