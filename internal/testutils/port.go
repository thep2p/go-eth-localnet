package testutils

import (
	"github.com/stretchr/testify/require"
	"net"
	"sync"
	"testing"
)

var (
	globalPortsMu sync.Mutex
	globalPorts   = make(map[int]struct{})
)

// NewPort returns a free TCP port that has not been handed out previously.
// It loops until it finds an unused port, ensuring tests never allocate the
// same port twice within a single run.
func NewPort(t *testing.T) int {
	for {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatalf("failed to find open port: %v", err)
		}

		port := l.Addr().(*net.TCPAddr).Port
		require.NoError(t, l.Close(), "failed to close port listener")

		globalPortsMu.Lock()
		_, taken := globalPorts[port]
		if !taken {
			globalPorts[port] = struct{}{}
		}
		globalPortsMu.Unlock()
		if taken {
			continue
		}
		return port
	}
}
