package testutils

import "testing"

// TestPortAssigner_UniqueAcrossInstances ensures ports are unique across assigners.
func TestPortAssigner_UniqueAcrossInstances(t *testing.T) {
	a1 := NewPortAssigner(t)
	p1 := a1.NewPort()
	a2 := NewPortAssigner(t)
	p2 := a2.NewPort()
	if p1 == p2 {
		t.Fatalf("expected different ports, got %d", p1)
	}
}
