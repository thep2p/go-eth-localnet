package testutils

import "testing"

// TestNewPort_UniqueAcrossCalls ensures ports are unique across calls.
func TestNewPort_UniqueAcrossCalls(t *testing.T) {
	p1 := NewPort(t)
	p2 := NewPort(t)
	if p1 == p2 {
		t.Fatalf("expected different ports, got %d", p1)
	}
}
