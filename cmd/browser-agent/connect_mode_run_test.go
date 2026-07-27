// Purpose: Tests for connect-mode daemon run lifecycle.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
)

type connectModeServerState struct {
	mu               sync.Mutex
	registerCalls    int
	unregisterCalls  int
	mcpCalls         int
	lastClientHeader string
	lastRegisterBody string
}

func TestRunConnectModeHappyPath(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	state := &connectModeServerState{}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/clients", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		state.mu.Lock()
		state.registerCalls++
		state.lastClientHeader = r.Header.Get("X-Kaboom-Client")
		state.lastRegisterBody = string(body)
		state.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		var req JSONRPCRequest
		_ = json.Unmarshal(body, &req)
		state.mu.Lock()
		state.mcpCalls++
		state.lastClientHeader = r.Header.Get("X-Kaboom-Client")
		state.mu.Unlock()
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/clients/client-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		state.mu.Lock()
		state.unregisterCalls++
		state.lastClientHeader = r.Header.Get("X-Kaboom-Client")
		state.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdin) error = %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stderr) error = %v", err)
	}

	oldIn := os.Stdin
	oldOut := os.Stdout
	oldErr := os.Stderr
	oldSink := diag.Sink()
	os.Stdin = inR
	os.Stdout = outW
	os.Stderr = errW
	diag.SetSink(errW)

	_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":99,"method":"ping","params":{}}`+"\n")
	_ = inW.Close()

	runConnectMode(port, "client-1", "/tmp/project")

	os.Stdin = oldIn
	os.Stdout = oldOut
	os.Stderr = oldErr
	diag.SetSink(oldSink)
	_ = inR.Close()
	_ = outW.Close()
	_ = errW.Close()

	stdoutBytes, _ := io.ReadAll(outR)
	stderrBytes, _ := io.ReadAll(errR)
	_ = outR.Close()
	_ = errR.Close()

	stdout := strings.TrimSpace(string(stdoutBytes))
	if stdout == "" {
		t.Fatal("expected JSON-RPC output from forwarded /mcp response")
	}
	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(strings.Split(stdout, "\n")[0]), &resp); err != nil {
		t.Fatalf("stdout first line invalid JSON-RPC: %v (%q)", err, stdout)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected protocol error in stdout: %+v", resp.Error)
	}

	stderr := string(stderrBytes)
	if !strings.Contains(stderr, "Connected to") || !strings.Contains(stderr, "Disconnected from") {
		t.Fatalf("stderr missing connect/disconnect messages: %q", stderr)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.registerCalls != 1 {
		t.Fatalf("register calls = %d, want 1", state.registerCalls)
	}
	if state.unregisterCalls != 1 {
		t.Fatalf("unregister calls = %d, want 1", state.unregisterCalls)
	}
	if state.mcpCalls != 1 {
		t.Fatalf("mcp calls = %d, want 1", state.mcpCalls)
	}
	if state.lastClientHeader != "client-1" {
		t.Fatalf("X-Kaboom-Client header = %q, want client-1", state.lastClientHeader)
	}
	if !strings.Contains(state.lastRegisterBody, `"cwd":"/tmp/project"`) {
		t.Fatalf("register body = %q, want cwd payload", state.lastRegisterBody)
	}
}

func TestRunConnectModeMCPForwardErrorResponse(t *testing.T) {
	// Do not run in parallel; test redirects process stdio.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdin) error = %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error = %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stderr) error = %v", err)
	}

	oldIn := os.Stdin
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdin = inR
	os.Stdout = outW
	os.Stderr = errW

	_, _ = io.WriteString(inW, `{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`+"\n")
	_ = inW.Close()

	// Health check fails because no server is listening; runConnectMode calls os.Exit(1),
	// so execute it in a subprocess-style branch by checking env and invoking current test binary.
	// Here, we only assert sendMCPError behavior by calling directly.
	sendMCPError(1, -32603, "Server connection error: dial tcp 127.0.0.1:"+strconv.Itoa(port))

	os.Stdin = oldIn
	os.Stdout = oldOut
	os.Stderr = oldErr
	_ = inR.Close()
	_ = outW.Close()
	_ = errW.Close()

	out, _ := io.ReadAll(outR)
	_ = outR.Close()
	_, _ = io.ReadAll(errR)
	_ = errR.Close()
	if !strings.Contains(string(out), `"code":-32603`) {
		t.Fatalf("expected -32603 in output, got %q", string(out))
	}
}
