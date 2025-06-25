package testutils

import (
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

var GlobalPortAssigner *PortAssigner

func init() {
	// Initialize a global port assigner for use in tests
	// This is useful for tests that need to allocate ports without conflicts
	GlobalPortAssigner = &PortAssigner{
		assignedPorts: make(map[int]struct{}),
	}
}

// PortAssigner provides deterministic port allocation for tests.
// It ensures each assigned port is unique to avoid conflicts when starting multiple nodes.
type PortAssigner struct {
	assignedPorts map[int]struct{}
}

// NewPort returns a free TCP port that has not been handed out previously.
// It loops until it finds an unused port, ensuring tests never allocate the
// same port twice within a single run.
func (p *PortAssigner) NewPort(t *testing.T) int {
	for {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			panic("failed to find open port: " + err.Error())
		}

		port := l.Addr().(*net.TCPAddr).Port

		require.NoError(t, l.Close(), "failed to close port listener")

		if _, taken := p.assignedPorts[port]; taken {
			continue
		}
		p.assignedPorts[port] = struct{}{}
		return port
	}
}
