// Purpose: Detects daemon connection errors and provides HTTP request helpers with health-check fallback.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// IsConnectionError returns true if the error indicates the daemon is unreachable.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// Prefer typed error checks over string matching
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	// Fallback: string check for wrapped errors that lose type info
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host")
}

// IsServerRunning checks if a server is healthy on the given port via HTTP health check.
func IsServerRunning(port int) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port)) // #nosec G704 -- localhost-only health probe
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// WaitForServer waits for the server to start accepting connections.
func WaitForServer(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsServerRunning(port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// DoHTTP sends a raw JSON-RPC payload to the daemon and returns the HTTP response.
// The caller must provide a context that outlives the response body read.
func DoHTTP(ctx context.Context, client *http.Client, endpoint string, line []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(line)) // #nosec G704 -- endpoint is localhost-only
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return client.Do(httpReq)
}
