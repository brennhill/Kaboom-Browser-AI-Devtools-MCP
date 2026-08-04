// Purpose: Unit tests for browser-agent bridge runmode logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestRunBridgeModeWithExistingServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`+"\n")
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	output := captureStdout(t, func() {
		withTestStdin(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n", func() {
			bridgeRuntime().RunMode(port, "", 0)
		})
	})
	if !strings.Contains(output, `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("runBridgeMode output missing initialize response: %q", output)
	}
}
