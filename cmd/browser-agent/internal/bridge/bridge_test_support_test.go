// bridge_test_support_test.go -- Test support helpers for bridge package tests.
package bridge

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// initTestDeps sets up minimal bridge deps for testing.
//
// It restores the previous package-global deps (installed by TestMain, or by an
// enclosing test) via t.Cleanup so this stub cannot leak into tests that run after
// this one. Without the restore, the minimal stub below (protocol 2024-11-05, no real
// resources, a spaces-only Content-Length writer) leaks and breaks every later test
// that relies on the real deps — the FastPath failures + framed-init hang we hit once.
func initTestDeps(t *testing.T) {
	t.Helper()
	prev := deps
	t.Cleanup(func() { deps = prev })
	Init(Deps{
		Version:            "0.0.0-test",
		MaxPostBodySize:    10 * 1024 * 1024,
		MCPServerName:      "kaboom",
		ServerInstructions: "test instructions",
		Stderrf:            func(format string, args ...any) {},
		Debugf:             func(format string, args ...any) {},
		WriteMCPPayload: func(payload []byte, framing internbridge.StdioFraming) {
			out := ActiveMCPTransportWriter()
			if framing == internbridge.StdioFramingContentLength {
				_, _ = out.Write([]byte("Content-Length: "))
				_, _ = out.Write([]byte(strings.Repeat(" ", len(payload))))
				_, _ = out.Write([]byte("\r\nContent-Type: application/json\r\n\r\n"))
				_, _ = out.Write(payload)
			} else {
				_, _ = out.Write(payload)
				_, _ = out.Write([]byte("\n"))
			}
		},
		SyncStdoutBestEffort:      func() {},
		SetStderrSink:             func(w io.Writer) {},
		GetBridgeFraming:          func() internbridge.StdioFraming { return internbridge.StdioFramingLine },
		StoreBridgeFraming:        func(f internbridge.StdioFraming) {},
		SetPushClientCapabilities: func(caps push.ClientCapabilities) {},
		ExtractClientCapabilities: func(rawParams json.RawMessage) push.ClientCapabilities {
			return push.ClientCapabilities{}
		},
		NegotiateProtocolVersion: func(rawParams json.RawMessage) string { return "2024-11-05" },
		MCPResources:             func() []mcp.MCPResource { return nil },
		MCPResourceTemplates:     func() []any { return nil },
		ResolveResourceContent:   func(uri string) (string, string, bool) { return "", "", false },
		DaemonProcessArgv0:       func(exePath string) string { return exePath },
		StopServerForUpgrade:     func(port int) bool { return false },
		FindProcessOnPort:        func(port int) ([]int, error) { return nil, nil },
		IsProcessAlive:           func(pid int) bool { return false },
		AppendExitDiagnostic:     func(event string, extra map[string]any) string { return "" },
	})
}

// Note: resetFastPathResourceReadCounters, resetFastPathCounters,
// captureBridgeIO, and parseJSONLines are defined in the test files that were moved from main.

// fastPathTelemetrySummary is a test-local summary type for telemetry log parsing.
type fastPathTelemetrySummary struct {
	total      int
	success    int
	failure    int
	errorCodes map[int]int
	methods    map[string]int
}

// summarizeFastPathTelemetryLog parses fast-path telemetry from a log file (test-only copy).
func summarizeFastPathTelemetryLog(path string, maxLines int) fastPathTelemetrySummary {
	summary := fastPathTelemetrySummary{
		errorCodes: map[int]int{},
		methods:    map[string]int{},
	}
	if maxLines <= 0 {
		return summary
	}

	f, err := os.Open(path)
	if err != nil {
		return summary
	}
	defer func() { _ = f.Close() }()

	lines := make([]string, 0, maxLines)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) > maxLines {
			lines = lines[1:]
		}
	}

	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		event, _ := entry["event"].(string)
		if event != "bridge_fastpath_method" {
			continue
		}
		summary.total++
		if ok, _ := entry["success"].(bool); ok {
			summary.success++
		} else {
			summary.failure++
		}
		if method, _ := entry["method"].(string); method != "" {
			summary.methods[method]++
		}
		if code, ok := entry["error_code"].(float64); ok {
			codeInt := int(code)
			if codeInt != 0 {
				summary.errorCodes[codeInt]++
			}
		}
	}
	return summary
}

// setStderrSink is a test helper that delegates to deps.SetStderrSink.
func setStderrSink(w io.Writer) {
	if deps.SetStderrSink != nil {
		deps.SetStderrSink(w)
	}
}
