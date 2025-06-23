package testutils

import (
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

// PortAssigner provides deterministic port allocation for tests.
// It ensures each assigned port is unique to avoid conflicts when starting multiple nodes.
type PortAssigner struct {
	t             *testing.T
	assignedPorts map[int]struct{}
}

// NewPortAssigner initializes a PortAssigner bound to the given test instance.
// The returned assigner should only be used within the lifetime of the test.
func NewPortAssigner(t *testing.T) *PortAssigner {
	return &PortAssigner{
		t:             t,
		assignedPorts: make(map[int]struct{}),
	}
}

// NewPort returns a free TCP port that has not been handed out previously.
// It loops until it finds an unused port, ensuring tests never allocate the
// same port twice within a single run.
func (p *PortAssigner) NewPort() int {
	for {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			panic("failed to find open port: " + err.Error())
		}

		port := l.Addr().(*net.TCPAddr).Port
		require.NoError(p.t, l.Close(), "failed to close port listener")

		if _, taken := p.assignedPorts[port]; taken {
			continue
		}
		p.assignedPorts[port] = struct{}{}
		return port
	}
}
