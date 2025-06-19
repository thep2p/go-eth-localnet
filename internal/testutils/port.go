package testutils

import (
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

type PortAssigner struct {
	t             *testing.T
	assignedPorts map[int]struct{}
}

func NewPortAssigner(t *testing.T) *PortAssigner {
	return &PortAssigner{
		t:             t,
		assignedPorts: make(map[int]struct{}),
	}
}

// NewPort returns a new randomly assigned port that is not currently in use.
// It keeps track of the assigned port to avoid conflicts in future calls.
func (p *PortAssigner) NewPort() int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic("failed to find open port: " + err.Error())
	}

	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(p.t, l.Close(), "failed to close port listener")

	return port
}
