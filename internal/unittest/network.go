package unittest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// RequireRpcReadyWithinTimeout is a test helper that fails if the RPC server is not ready within the specified timeout.
func RequireRpcReadyWithinTimeout(t *testing.T, ctx context.Context, port int, timeout time.Duration) {
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	body := []byte(`{"jsonrpc":"2.0","method":"net_version","params":[],"id":1}`)

	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("RPC not ready on port %d within %s", port, timeout)
}

// RequirePortClosesWithinTimeout is a test helper that fails if the specified port does not close within the timeout.
func RequirePortClosesWithinTimeout(t *testing.T, port int, timeout time.Duration) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return // port is closed
		}
		_ = conn.Close()
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("port %d did not close within %s", port, timeout)
}
