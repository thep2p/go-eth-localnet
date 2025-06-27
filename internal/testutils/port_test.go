package testutils

import "testing"

// TestPortAssigner_UniqueAcrossInstances ensures ports are unique across assigners.
func TestNewPort_UniqueAcrossCalls(t *testing.T) {
	p1 := NewPort(t)
	p2 := NewPort(t)
	if p1 == p2 {
		t.Fatalf("expected different ports, got %d", p1)
	}
}
