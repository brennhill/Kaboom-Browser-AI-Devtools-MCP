// upload_test.go — Unit tests for upload request validation and queueing. The
// validation branches (missing params, relative path, directory, unreadable file)
// were previously reachable only through the daemon's end-to-end interact tests.

package interactupload

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

type fake struct {
	blockPilot   bool
	blockExt     bool
	blockTab     bool
	blockQueue   bool
	enqueued     []queries.PendingQuery
	recorded     []string
	armed        []string
	armedActions []string
}

func (f *fake) ArmEvidenceForCommand(correlationID, action string, _ json.RawMessage, _ string) {
	f.armed = append(f.armed, correlationID)
	f.armedActions = append(f.armedActions, action)
}

func guard(block *bool, code string) Guard {
	return func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
		if *block {
			return mcp.Fail(req, code, "blocked", "unblock it", opts...), true
		}
		return mcp.JSONRPCResponse{}, false
	}
}

func newHandler() (*Handler, *fake) {
	f := &fake{}
	deps := &Deps{
		RequirePilot:       guard(&f.blockPilot, mcp.ErrCodePilotDisabled),
		RequireExtension:   guard(&f.blockExt, mcp.ErrNotInitialized),
		RequireTabTracking: guard(&f.blockTab, mcp.ErrNoData),
		EnqueuePendingQuery: func(req mcp.JSONRPCRequest, q queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
			if f.blockQueue {
				return mcp.Fail(req, mcp.ErrQueueFull, "queue full", "retry later"), true
			}
			f.enqueued = append(f.enqueued, q)
			return mcp.JSONRPCResponse{}, false
		},
		RecordAIAction: func(action, _ string, _ map[string]any) { f.recorded = append(f.recorded, action) },
	}
	return New(deps, f), f
}

func req() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, ClientID: "client-test"}
}

func tempFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func mustContain(t *testing.T, resp mcp.JSONRPCResponse, want string) {
	t.Helper()
	if !strings.Contains(string(resp.Result), want) {
		t.Fatalf("response %s does not contain %q", string(resp.Result), want)
	}
}

func TestHandleUpload_ValidationRejections(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "nope.bin")
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		args string
		want string
	}{
		{"missing file_path", `{"selector":"#f"}`, mcp.ErrMissingParam},
		{"neither selector nor api_endpoint", `{"file_path":"/tmp/a.bin"}`, mcp.ErrMissingParam},
		{"relative path", `{"selector":"#f","file_path":"rel/a.bin"}`, mcp.ErrPathNotAllowed},
		{"file not found", `{"selector":"#f","file_path":"` + absent + `"}`, mcp.ErrInvalidParam},
		{"path is a directory", `{"selector":"#f","file_path":"` + dir + `"}`, mcp.ErrInvalidParam},
		{"invalid json", `not-json`, mcp.ErrInvalidJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, f := newHandler()
			mustContain(t, h.HandleUpload(req(), json.RawMessage(tc.args)), tc.want)
			if len(f.enqueued) != 0 {
				t.Fatalf("a rejected upload must not be queued, got %+v", f.enqueued)
			}
			if len(f.armed) != 0 {
				t.Fatalf("a rejected upload must not arm evidence, got %v", f.armed)
			}
		})
	}
}

func TestHandleUpload_APIEndpointSatisfiesSelectorRequirement(t *testing.T) {
	h, f := newHandler()
	path := tempFile(t)
	resp := h.HandleUpload(req(), json.RawMessage(`{"api_endpoint":"https://x.test/u","file_path":"`+path+`"}`))
	mustContain(t, resp, "queued")
	if len(f.enqueued) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(f.enqueued))
	}
	var payload map[string]any
	if err := json.Unmarshal(f.enqueued[0].Params, &payload); err != nil {
		t.Fatalf("unmarshal queued params: %v", err)
	}
	if payload["api_endpoint"] != "https://x.test/u" {
		t.Fatalf("api_endpoint = %v, want it forwarded to the extension", payload["api_endpoint"])
	}
}

func TestHandleUpload_OmitsAPIEndpointWhenUnset(t *testing.T) {
	h, f := newHandler()
	path := tempFile(t)
	h.HandleUpload(req(), json.RawMessage(`{"selector":"#f","file_path":"`+path+`"}`))
	var payload map[string]any
	if err := json.Unmarshal(f.enqueued[0].Params, &payload); err != nil {
		t.Fatalf("unmarshal queued params: %v", err)
	}
	if _, present := payload["api_endpoint"]; present {
		t.Fatal("api_endpoint must be absent from the payload when the caller did not set it")
	}
}

func TestHandleUpload_GatesBlockBeforeTouchingTheFilesystem(t *testing.T) {
	// A path that would fail validateUploadFile: the gates must fire first, so a
	// blocked caller learns why it was blocked rather than "file not found".
	absent := filepath.Join(t.TempDir(), "nope.bin")
	for _, tc := range []struct {
		name  string
		set   func(f *fake)
		wants string
	}{
		{"pilot", func(f *fake) { f.blockPilot = true }, mcp.ErrCodePilotDisabled},
		{"extension", func(f *fake) { f.blockExt = true }, mcp.ErrNotInitialized},
		{"tab tracking", func(f *fake) { f.blockTab = true }, mcp.ErrNoData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, f := newHandler()
			tc.set(f)
			mustContain(t, h.HandleUpload(req(), json.RawMessage(`{"selector":"#f","file_path":"`+absent+`"}`)), tc.wants)
		})
	}
}

func TestHandleUpload_QueuesWithDefaultsAndArmsEvidence(t *testing.T) {
	h, f := newHandler()
	path := tempFile(t)
	resp := h.HandleUpload(req(), json.RawMessage(`{"selector":"#f","file_path":"`+path+`","submit":true}`))
	mustContain(t, resp, "queued")

	if len(f.enqueued) != 1 || f.enqueued[0].Type != "upload" {
		t.Fatalf("expected one 'upload' query, got %+v", f.enqueued)
	}
	var payload map[string]any
	if err := json.Unmarshal(f.enqueued[0].Params, &payload); err != nil {
		t.Fatalf("unmarshal queued params: %v", err)
	}
	if payload["escalation_timeout_ms"] != float64(defaultEscalationTimeoutMs) {
		t.Fatalf("escalation_timeout_ms = %v, want the default %d applied", payload["escalation_timeout_ms"], defaultEscalationTimeoutMs)
	}
	if payload["file_size"] != float64(10) {
		t.Fatalf("file_size = %v, want 10 (the real size on disk)", payload["file_size"])
	}
	if payload["submit"] != true {
		t.Fatalf("submit = %v, want it forwarded", payload["submit"])
	}

	// Evidence must be armed against the SAME correlation id that was queued,
	// or the before/after screenshots attach to nothing.
	if len(f.armed) != 1 || f.armed[0] != f.enqueued[0].CorrelationID {
		t.Fatalf("armed evidence for %v, queued %q", f.armed, f.enqueued[0].CorrelationID)
	}
	if f.armedActions[0] != "upload" {
		t.Fatalf("armed action = %q, want upload", f.armedActions[0])
	}
	if len(f.recorded) != 1 || f.recorded[0] != "upload" {
		t.Fatalf("recorded = %v, want one 'upload'", f.recorded)
	}
}

func TestHandleUpload_HonoursExplicitEscalationTimeout(t *testing.T) {
	h, f := newHandler()
	path := tempFile(t)
	h.HandleUpload(req(), json.RawMessage(`{"selector":"#f","file_path":"`+path+`","escalation_timeout_ms":1234}`))
	var payload map[string]any
	if err := json.Unmarshal(f.enqueued[0].Params, &payload); err != nil {
		t.Fatalf("unmarshal queued params: %v", err)
	}
	if payload["escalation_timeout_ms"] != float64(1234) {
		t.Fatalf("escalation_timeout_ms = %v, want the caller's 1234", payload["escalation_timeout_ms"])
	}
}

func TestHandleUpload_BlockedQueueIsNotRecorded(t *testing.T) {
	h, f := newHandler()
	f.blockQueue = true
	path := tempFile(t)
	mustContain(t, h.HandleUpload(req(), json.RawMessage(`{"selector":"#f","file_path":"`+path+`"}`)), mcp.ErrQueueFull)
	if len(f.recorded) != 0 {
		t.Fatalf("a rejected enqueue must not record an AI action, got %v", f.recorded)
	}
}
