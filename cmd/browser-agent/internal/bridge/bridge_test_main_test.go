// bridge_test_main_test.go -- TestMain for bridge package tests.
package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

func TestMain(m *testing.M) {
	Init(Deps{
		Version:            "0.0.0-test",
		MaxPostBodySize:    10 * 1024 * 1024,
		MCPServerName:      "kaboom",
		ServerInstructions: "test instructions",
		Stderrf:            func(format string, args ...any) { fmt.Fprintf(os.Stderr, format, args...) },
		Debugf:             func(format string, args ...any) {},
		WriteMCPPayload: func(payload []byte, framing internbridge.StdioFraming) {
			out := ActiveMCPTransportWriter()
			if framing == internbridge.StdioFramingContentLength {
				_, _ = fmt.Fprintf(out, "Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(payload), payload)
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
		NegotiateProtocolVersion: func(rawParams json.RawMessage) string { return "2025-06-18" },
		MCPResources: func() []mcp.MCPResource {
			return []mcp.MCPResource{
				{URI: "kaboom://capabilities", Name: "capabilities", MimeType: "text/markdown"},
			}
		},
		MCPResourceTemplates: func() []any { return nil },
		ResolveResourceContent: func(uri string) (string, string, bool) {
			if uri == "kaboom://capabilities" {
				return "kaboom://capabilities", "# Capabilities\nTest content", true
			}
			// Handle known playbook URIs and aliases
			knownPlaybooks := map[string]string{
				"kaboom://playbook/security":             "kaboom://playbook/security/quick",
				"kaboom://playbook/security/quick":       "kaboom://playbook/security/quick",
				"kaboom://playbook/security_audit/quick": "kaboom://playbook/security/quick",
			}
			if canonical, ok := knownPlaybooks[uri]; ok {
				return canonical, "# Playbook\nTest playbook content", true
			}
			return "", "", false
		},
		DaemonProcessArgv0:   func(exePath string) string { return exePath },
		StopServerForUpgrade: func(port int) bool { return false },
		FindProcessOnPort:    func(port int) ([]int, error) { return nil, nil },
		IsProcessAlive: func(pid int) bool {
			if pid <= 0 {
				return false
			}
			// On Unix, FindProcess always succeeds. Use kill -0 to check.
			p, err := os.FindProcess(pid)
			if err != nil {
				return false
			}
			// Signal(syscall.Signal(0)) checks existence without killing.
			return p.Signal(nil) == nil || pid == os.Getpid()
		},
		AppendExitDiagnostic: func(event string, extra map[string]any) string { return "" },
	})

	os.Exit(m.Run())
}
