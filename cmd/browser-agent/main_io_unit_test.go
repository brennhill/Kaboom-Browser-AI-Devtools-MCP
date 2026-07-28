// Purpose: Unit tests for browser-agent main io logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"bytes"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"

	cmbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func captureDiagnostics(t *testing.T, run func()) string {
	t.Helper()
	previous := diag.Sink()
	var output bytes.Buffer
	diag.SetSink(&output)
	defer diag.SetSink(previous)
	run()
	return output.String()
}

func TestSendStartupErrorWritesJSONRPCError(t *testing.T) {
	output := captureStdout(t, func() {
		cmbridge.SendStartupError("boom")
	})
	line := strings.TrimSpace(output)
	if line == "" {
		t.Fatal("sendStartupError produced empty output")
	}

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("json.Unmarshal(output) error = %v; output=%q", err, line)
	}
	if resp.ID != "startup" {
		t.Fatalf("resp.ID = %v, want %q", resp.ID, "startup")
	}
	if resp.Error == nil || resp.Error.Code != -32603 {
		t.Fatalf("resp.Error = %+v, want code -32603", resp.Error)
	}
	if !strings.Contains(resp.Error.Message, "boom") {
		t.Fatalf("resp.Error.Message = %q, want to contain %q", resp.Error.Message, "boom")
	}
}

func TestPrintHelpIncludesKeySections(t *testing.T) {
	output := captureDiagnostics(t, printHelp)
	if !strings.Contains(output, "Usage: kaboom [options]") {
		t.Fatalf("help output missing usage header: %q", output)
	}
	if !strings.Contains(output, "--state-dir <path>") {
		t.Fatalf("help output missing --state-dir docs")
	}
	if !strings.Contains(output, "--parallel") {
		t.Fatalf("help output missing --parallel docs")
	}
	if !strings.Contains(output, "--doctor") {
		t.Fatalf("help output missing --doctor docs")
	}
	if strings.Contains(output, "--check") {
		t.Fatalf("help output retains removed --check compatibility facade")
	}
	if !strings.Contains(output, "CLI Mode (direct tool access):") {
		t.Fatalf("help output missing CLI mode section")
	}
}

func TestRunSetupCheckPortInUseBranch(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	output := captureDiagnostics(t, func() {
		runSetupCheckWithOptions(port, setupCheckOptions{})
	})
	if !strings.Contains(output, "Checking port availability... FAILED") {
		t.Fatalf("setup check output missing failed-port branch: %q", output)
	}
	if !strings.Contains(output, "Port "+strconv.Itoa(port)+" is already in use.") {
		t.Fatalf("setup check output missing in-use message: %q", output)
	}
	if !strings.Contains(output, "Quick stop (Kaboom): kaboom --stop --port "+strconv.Itoa(port)) {
		t.Fatalf("setup check output missing quick-stop guidance: %q", output)
	}
	if !strings.Contains(output, "Suggested free port: --port ") {
		t.Fatalf("setup check output missing suggested free port guidance: %q", output)
	}
}
