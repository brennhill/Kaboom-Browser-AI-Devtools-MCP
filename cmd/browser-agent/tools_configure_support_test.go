// tools_configure_support_test.go — Tests explicit Doctor support-bundle approval and export.
package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func supportResponseText(t *testing.T, response mcp.JSONRPCResponse) string {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Fatal("empty support response")
	}
	return result.Content[0].Text
}

func supportPayload(t *testing.T, response mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	text := supportResponseText(t, response)
	newline := strings.IndexByte(text, '\n')
	if newline < 0 {
		t.Fatalf("support response has no payload: %q", text)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text[newline+1:]), &payload); err != nil {
		t.Fatalf("decode support payload: %v", err)
	}
	return payload
}

func TestDoctorSupportExportRequiresMatchingPreview(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	views := []incident.DoctorView{{Code: incident.CodeQueueSaturated, Fingerprint: "0123456789abcdef", State: incident.StateDetected}}
	preview, handled := handleDoctorSupportAction(req, json.RawMessage(`{"doctor_action":"preview_support_bundle"}`), views)
	if !handled {
		t.Fatal("preview was not handled")
	}
	payload := supportPayload(t, preview)
	token, ok := payload["confirmation_token"].(string)
	if !ok || token == "" {
		t.Fatalf("preview missing token: %#v", payload)
	}

	oldWrite := writeDoctorSupportBundle
	t.Cleanup(func() { writeDoctorSupportBundle = oldWrite })
	var written []byte
	writeDoctorSupportBundle = func(_ string, data []byte) error { written = append([]byte(nil), data...); return nil }
	bad, _ := handleDoctorSupportAction(req, json.RawMessage(`{"doctor_action":"export_support_bundle","confirmation_token":"wrong","output_path":"bundle.json"}`), views)
	if !strings.Contains(supportResponseText(t, bad), "does not match") || len(written) != 0 {
		t.Fatal("mismatched preview wrote a bundle")
	}
	args, _ := json.Marshal(doctorSupportArgs{Action: "export_support_bundle", ConfirmationToken: token, OutputPath: "bundle.json"})
	good, _ := handleDoctorSupportAction(req, args, views)
	if len(written) == 0 || supportPayload(t, good)["artifact"] != string(written) {
		t.Fatalf("export did not return the exact written artifact: response=%q artifact=%q", supportResponseText(t, good), written)
	}
}

func TestDoctorSupportExportRejectsStalePreviewAndReportsWriteFailure(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	initial := []incident.DoctorView{{Code: incident.CodeQueueSaturated, Fingerprint: "0123456789abcdef", State: incident.StateDetected}}
	preview, _ := handleDoctorSupportAction(req, json.RawMessage(`{"doctor_action":"preview_support_bundle"}`), initial)
	token := supportPayload(t, preview)["confirmation_token"].(string)

	changed := []incident.DoctorView{{Code: incident.CodeQueueSaturated, Fingerprint: "0123456789abcdef", State: incident.StateRetrying, Attempts: 1}}
	args, _ := json.Marshal(doctorSupportArgs{Action: "export_support_bundle", ConfirmationToken: token, OutputPath: "private/path.json"})
	stale, _ := handleDoctorSupportAction(req, args, changed)
	if !strings.Contains(supportResponseText(t, stale), "does not match") {
		t.Fatalf("stale preview accepted: %s", supportResponseText(t, stale))
	}

	current, _ := handleDoctorSupportAction(req, json.RawMessage(`{"doctor_action":"preview_support_bundle"}`), changed)
	currentToken := supportPayload(t, current)["confirmation_token"].(string)
	oldWrite := writeDoctorSupportBundle
	t.Cleanup(func() { writeDoctorSupportBundle = oldWrite })
	writeDoctorSupportBundle = func(_ string, _ []byte) error { return errors.New("private/path.json: secret failure") }
	args, _ = json.Marshal(doctorSupportArgs{Action: "export_support_bundle", ConfirmationToken: currentToken, OutputPath: "private/path.json"})
	failed, _ := handleDoctorSupportAction(req, args, changed)
	text := supportResponseText(t, failed)
	if !strings.Contains(text, "export failed") || strings.Contains(text, "private/path") || strings.Contains(text, "secret failure") {
		t.Fatalf("write failure was absent or leaked private detail: %s", text)
	}
}

func TestDoctorSupportActionRejectsMalformedArguments(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	response, handled := handleDoctorSupportAction(req, json.RawMessage(`{"doctor_action":`), nil)
	if !handled || !strings.Contains(supportResponseText(t, response), "Invalid Doctor support arguments") {
		t.Fatalf("malformed support arguments fell through: handled=%v response=%s", handled, supportResponseText(t, response))
	}
}
