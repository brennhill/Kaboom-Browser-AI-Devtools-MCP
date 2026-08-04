// Purpose: Additional tests for main helper functions.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/configdiscovery"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestFindMCPConfigResolution(t *testing.T) {
	// Do not run in parallel; uses Setenv.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	home := os.Getenv("HOME")
	continuePath := filepath.Join(home, ".continue", "config.json")
	if err := os.MkdirAll(filepath.Dir(continuePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(continuePath), err)
	}
	if err := os.WriteFile(
		continuePath,
		[]byte(`{"mcpServers":{"kaboom-browser-devtools":{"command":"kaboom-agentic-browser"}}}`),
		0o600,
	); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", continuePath, err)
	}
	if got := configdiscovery.Find(); got != continuePath {
		t.Fatalf("configdiscovery.Find() = %q, want %q", got, continuePath)
	}
}

func TestFindMCPConfigResolutionClaudePath(t *testing.T) {
	// Do not run in parallel; uses Setenv.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	home := os.Getenv("HOME")
	claudePath := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(claudePath, []byte(`{"mcpServers":{"kaboom-browser-devtools":{"command":"kaboom-agentic-browser"}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", claudePath, err)
	}
	if got := configdiscovery.Find(); got != claudePath {
		t.Fatalf("configdiscovery.Find() = %q, want %q", got, claudePath)
	}
}

func TestRunSetupCheckPrintsDiagnostics(t *testing.T) {
	// Do not run in parallel; test redirects the process-wide diagnostic sink.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	text := captureDiagnostics(t, func() {
		runSetupCheckWithOptions(port, setupCheckOptions{})
	})
	if !strings.Contains(text, "KABOOM SETUP CHECK") || !strings.Contains(text, "Next steps:") {
		t.Fatalf("runSetupCheck output missing expected sections:\n%s", text)
	}
	if !strings.Contains(text, "Port:    "+strconv.Itoa(port)) {
		t.Fatalf("runSetupCheck output missing port %d", port)
	}
}

func TestEvaluateFastPathFailureThreshold(t *testing.T) {
	t.Parallel()

	summary := health.FastPathTelemetrySummary{
		Total:      10,
		Success:    9,
		Failure:    1,
		ErrorCodes: map[int]int{-32002: 1},
		Methods:    map[string]int{"resources/read": 10},
	}
	if err := health.EvaluateFastPathFailureThreshold(summary, 5, 0.2); err != nil {
		t.Fatalf("expected threshold pass, got err=%v", err)
	}
	if err := health.EvaluateFastPathFailureThreshold(summary, 5, 0.05); err == nil {
		t.Fatal("expected threshold failure error, got nil")
	}
	if err := health.EvaluateFastPathFailureThreshold(summary, 20, 0.2); err == nil {
		t.Fatal("expected insufficient samples error, got nil")
	}
}

func TestRunSetupCheckIncludesFastPathTelemetrySummary(t *testing.T) {
	// Do not run in parallel; test redirects diagnostics and uses Setenv.
	t.Setenv(state.StateDirEnv, t.TempDir())
	bridge.ResetFastPathCounters()
	bridgeRuntime().RecordFastPathEvent("resources/read", true, 0)
	bridgeRuntime().RecordFastPathEvent("resources/read", false, -32002)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	var ok bool
	text := captureDiagnostics(t, func() {
		ok = runSetupCheckWithOptions(port, setupCheckOptions{
			minSamples:      2,
			maxFailureRatio: 0.1,
		})
	})
	if ok {
		t.Fatal("runSetupCheckWithOptions should fail threshold check")
	}
	if !strings.Contains(text, "Checking bridge fast-path telemetry...") {
		t.Fatalf("expected fast-path telemetry diagnostics, got:\n%s", text)
	}
	if !strings.Contains(text, "Checking fast-path failure threshold... FAILED") {
		t.Fatalf("expected threshold failure output, got:\n%s", text)
	}
}
