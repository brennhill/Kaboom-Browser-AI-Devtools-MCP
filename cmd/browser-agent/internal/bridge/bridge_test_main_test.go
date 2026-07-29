// bridge_test_main_test.go — Shared constructed bridge runner for package tests.
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
