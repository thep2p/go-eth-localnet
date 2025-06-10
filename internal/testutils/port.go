package testutils

import (
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

// randomPort finds an available TCP port on localhost.
func randomPort(t *testing.T) int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic("failed to find open port: " + err.Error())
	}
	defer func() {
		require.NoError(t, l.Close(), "failed to close listener after finding random port")
	}()
	return l.Addr().(*net.TCPAddr).Port
}

// RandomPort is like randomPort but panics via `t.Fatalf` instead of `panic`.
func RandomPort(t *testing.T) int {
	t.Helper()
	return randomPort(t)
}
