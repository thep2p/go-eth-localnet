package testutils

import (
	"net"
	"testing"
)

// RandomPort finds an available TCP port on localhost.
func randomPort() int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic("failed to find open port: " + err.Error())
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// RandomPortT is like RandomPort but panics via `t.Fatalf` instead of `panic`.
func RandomPortT(t *testing.T) int {
	t.Helper()
	return randomPort()
}
