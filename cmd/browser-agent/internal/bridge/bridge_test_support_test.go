// bridge_test_support_test.go -- Test support helpers for bridge package tests.
package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/startuplock"
	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

var testRunner *Runner

func newTestRunner() *Runner {
	return NewRunner(
		Identity{Version: "0.0.0-test", ServerName: "kaboom", ServerInstructions: "test instructions"},
		Transport{
			MaxBodySize: 10 * 1024 * 1024,
			Stderrf:     func(format string, args ...any) { _, _ = fmt.Fprintf(os.Stderr, format, args...) },
			Debugf:      func(string, ...any) {},
			Write: func(payload []byte, framing internbridge.StdioFraming) {
				out := ActiveMCPTransportWriter()
				if framing == internbridge.StdioFramingContentLength {
					_, _ = fmt.Fprintf(out, "Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(payload), payload)
				} else {
					_, _ = out.Write(append(payload, '\n'))
				}
			},
			Sync: func() {}, SetStderr: func(io.Writer) {},
		},
		Protocol{
			GetFraming:          func() internbridge.StdioFraming { return internbridge.StdioFramingLine },
			StoreFraming:        func(internbridge.StdioFraming) {},
			SetCapabilities:     func(push.ClientCapabilities) {},
			ExtractCapabilities: func(json.RawMessage) push.ClientCapabilities { return push.ClientCapabilities{} },
			NegotiateVersion:    func(json.RawMessage) string { return "2025-06-18" },
			Resources: func() []mcp.MCPResource {
				return []mcp.MCPResource{{URI: "kaboom://capabilities", Name: "capabilities", MimeType: "text/markdown"}}
			},
			ResourceTemplates: func() []any { return nil },
			ResolveResource: func(uri string) (string, string, bool) {
				known := map[string]string{
					"kaboom://capabilities":                  "# Capabilities\nTest content",
					"kaboom://playbook/security":             "# Playbook\nTest playbook content",
					"kaboom://playbook/security/quick":       "# Playbook\nTest playbook content",
					"kaboom://playbook/security_audit/quick": "# Playbook\nTest playbook content",
				}
				text, ok := known[uri]
				if !ok {
					return "", "", false
				}
				if uri != "kaboom://capabilities" {
					return "kaboom://playbook/security/quick", text, true
				}
				return uri, text, true
			},
		},
		Lifecycle{
			ProcessArgv0:         func(path string) string { return path },
			StopServerForUpgrade: func(int) bool { return false },
			FindProcessOnPort:    func(int) ([]int, error) { return nil, nil },
			IsProcessAlive:       func(pid int) bool { return pid == os.Getpid() },
			AppendExitDiagnostic: func(string, map[string]any) string { return "" },
		},
	)
}

func TestMain(m *testing.M) {
	testRunner = newTestRunner()
	os.Exit(m.Run())
}

func TestRunnerDependenciesAreInstanceScoped(t *testing.T) {
	first := newTestRunner()
	second := newTestRunner()
	first.identity.Version = "first"
	if second.identity.Version == first.identity.Version {
		t.Fatal("runner identity leaked across constructed instances")
	}
}

// initTestDeps installs a fresh constructed runner for a test.
func initTestDeps(t *testing.T) {
	t.Helper()
	testRunner = newTestRunner()
	testRunner.transport.Debugf = func(string, ...any) {}
	testRunner.protocol.NegotiateVersion = func(json.RawMessage) string { return "2024-11-05" }
	testRunner.protocol.Resources = func() []mcp.MCPResource { return nil }
	testRunner.protocol.ResourceTemplates = func() []any { return nil }
	testRunner.protocol.ResolveResource = func(string) (string, string, bool) { return "", "", false }
}

func testStartupLockManager() startuplock.Manager {
	return startuplock.NewManager(testRunner.identity.Version, testRunner.lifecycle.IsProcessAlive)
}

// Note: captureBridgeIO and parseJSONLines are defined in the tests that own them.

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

// setStderrSink is a test helper that delegates to testRunner.transport.SetStderr.
func setStderrSink(w io.Writer) {
	if testRunner.transport.SetStderr != nil {
		testRunner.transport.SetStderr(w)
	}
}
