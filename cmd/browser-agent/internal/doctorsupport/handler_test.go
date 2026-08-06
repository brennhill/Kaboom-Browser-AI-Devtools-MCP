// handler_test.go — Verifies explicit preview approval and redacted local export.

package doctorsupport

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func responsePayload(t *testing.T, response mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil || len(result.Content) == 0 {
		t.Fatalf("decode result: %v", err)
	}
	parts := strings.SplitN(result.Content[0].Text, "\n", 2)
	if len(parts) != 2 {
		t.Fatalf("response has no payload: %q", result.Content[0].Text)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(parts[1]), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload
}

func TestHandleRequiresCurrentPreviewBeforeExport(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	views := []incident.DoctorView{{Code: incident.CodeQueueSaturated, Fingerprint: "0123456789abcdef", State: incident.StateDetected}}
	preview, handled := Handle(req, json.RawMessage(`{"doctor_action":"preview_support_bundle"}`), views, "0.9.0", "test-platform", nil)
	if !handled {
		t.Fatal("preview was not handled")
	}
	token, _ := responsePayload(t, preview)["confirmation_token"].(string)
	if token == "" {
		t.Fatal("preview omitted confirmation token")
	}
	var written []byte
	writer := func(_ string, data []byte) error { written = append([]byte(nil), data...); return nil }
	bad, _ := Handle(req, json.RawMessage(`{"doctor_action":"export_support_bundle","confirmation_token":"wrong","output_path":"bundle.json"}`), views, "0.9.0", "test-platform", writer)
	if strings.Contains(string(bad.Result), `"artifact"`) || len(written) != 0 {
		t.Fatal("mismatched preview wrote a bundle")
	}
	args, _ := json.Marshal(arguments{Action: "export_support_bundle", ConfirmationToken: token, OutputPath: "bundle.json"})
	good, _ := Handle(req, args, views, "0.9.0", "test-platform", writer)
	if len(written) == 0 || responsePayload(t, good)["artifact"] != string(written) {
		t.Fatal("export did not return the exact written artifact")
	}
}

func TestHandleRedactsWriterFailuresAndRejectsMalformedInput(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	malformed, handled := Handle(req, json.RawMessage(`{"doctor_action":`), nil, "0.9.0", "test-platform", nil)
	if !handled || !strings.Contains(string(malformed.Result), "Invalid Doctor support arguments") {
		t.Fatal("malformed support arguments fell through")
	}
	views := []incident.DoctorView{{Code: incident.CodeQueueSaturated, Fingerprint: "0123456789abcdef", State: incident.StateDetected}}
	preview, _ := Handle(req, json.RawMessage(`{"doctor_action":"preview_support_bundle"}`), views, "0.9.0", "test-platform", nil)
	token := responsePayload(t, preview)["confirmation_token"].(string)
	args, _ := json.Marshal(arguments{Action: "export_support_bundle", ConfirmationToken: token, OutputPath: "private/path.json"})
	failed, _ := Handle(req, args, views, "0.9.0", "test-platform", func(string, []byte) error {
		return errors.New("private/path.json: secret failure")
	})
	text := string(failed.Result)
	if !strings.Contains(text, "export failed") || strings.Contains(text, "private/path") || strings.Contains(text, "secret failure") {
		t.Fatalf("failure was absent or leaked private detail: %s", text)
	}
}
