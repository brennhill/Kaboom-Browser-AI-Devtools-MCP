// Purpose: Tests for connection diagnostic reporting.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsync"
)

func freePortForTest(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestGatherConnectionDiagnosticsNotListening(t *testing.T) {
	t.Parallel()

	port := freePortForTest(t)
	serverURL := "http://127.0.0.1:" + strconv.Itoa(port)
	healthURL := serverURL + "/health"

	diag := gatherConnectionDiagnostics(port, serverURL, healthURL)
	if got, _ := diag["port_status"].(string); got != "not listening" {
		t.Fatalf("port_status = %q, want not listening", got)
	}
	if got, _ := diag["diagnosis"].(string); got != "No server running on port" {
		t.Fatalf("diagnosis = %q, want 'No server running on port'", got)
	}
}

func TestGatherConnectionDiagnosticsHealthyServer(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok","name":"kaboom-browser-devtools"}`)
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      0,
			Result:  json.RawMessage(`{}`),
		})
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	serverURL := "http://127.0.0.1:" + strconv.Itoa(port)
	healthURL := serverURL + "/health"
	diag := gatherConnectionDiagnostics(port, serverURL, healthURL)

	if got, _ := diag["port_status"].(string); got != "listening" {
		t.Fatalf("port_status = %q, want listening", got)
	}
	if got, _ := diag["health_check"].(string); got != "passed" {
		t.Fatalf("health_check = %q, want passed", got)
	}
	if got, _ := diag["mcp_status"].(string); got != "responsive" {
		t.Fatalf("mcp_status = %q, want responsive", got)
	}
}

func TestRunStopModeRemovesStalePIDAndHandlesNoServer(t *testing.T) {
	// Do not run in parallel; uses Setenv and captures stdout.
	stateRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	port := freePortForTest(t)

	pidPath := filepath.Join(stateRoot, "run", "kaboom-"+strconv.Itoa(port)+".pid")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(pidPath), err)
	}
	if err := os.WriteFile(pidPath, []byte("999999"), 0o600); err != nil {
		t.Fatalf("WriteFile(pid) error = %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	oldOut := os.Stdout
	os.Stdout = w
	runStopMode(port)
	os.Stdout = oldOut
	_ = w.Close()
	out, _ := io.ReadAll(r)
	_ = r.Close()

	if _, err := os.Stat(pidPath); err == nil {
		t.Fatalf("expected stale pid file %q to be removed", pidPath)
	}
	if !strings.Contains(string(out), "Stopping kaboom server") {
		t.Fatalf("runStopMode output missing startup line: %q", string(out))
	}
}

func TestRunStopModeHTTPShutdownPath(t *testing.T) {
	// Do not run in parallel; captures stdout.
	port := freePortForTest(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"stopping":true}`)
	})

	srv := &http.Server{
		Addr:              "127.0.0.1:" + strconv.Itoa(port),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() { _ = srv.ListenAndServe() }()
	t.Cleanup(func() {
		_ = srv.Close()
	})
	// ListenAndServe binds asynchronously; wait for a real response instead of
	// assuming 75ms is always enough.
	testsync.Eventually(t, testsync.DefaultTimeout, "the diagnostic test server to accept requests", func() bool {
		resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health")
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return true
	})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	oldOut := os.Stdout
	os.Stdout = w
	runStopMode(port)
	os.Stdout = oldOut
	_ = w.Close()
	out, _ := io.ReadAll(r)
	_ = r.Close()

	if !strings.Contains(string(out), "Server stopped via HTTP endpoint") {
		t.Fatalf("runStopMode output = %q, expected HTTP shutdown success path", string(out))
	}
}
