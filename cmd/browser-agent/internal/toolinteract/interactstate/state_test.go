// state_test.go — Unit tests for the state-time-travel handler, driven entirely
// through fake Deps and a temp-dir SessionStore. These branches were previously
// only reachable through the daemon's end-to-end interact tests; the narrow Deps
// introduced with this package is what makes them addressable in isolation.

package interactstate

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

func TestMain(m *testing.M) {
	stateRoot, err := os.MkdirTemp("", "kaboom-interactstate-tests-*")
	if err != nil {
		panic("create interactstate test root: " + err.Error())
	}
	if err := os.Setenv(state.StateDirEnv, stateRoot); err != nil {
		panic("set interactstate test root: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(stateRoot)
	os.Exit(code)
}

type fake struct {
	pilot     bool
	extension bool
	tabID     int
	tabURL    string
	tabTitle  string

	enqueued   []queries.PendingQuery
	blockQueue bool

	cmdResult *queries.CommandResult
	cmdFound  bool

	recorded    []string
	redactCalls int
	noStore     bool
}

func newFake() *fake {
	return &fake{pilot: true, extension: true, tabID: 42, tabURL: "https://example.test/", tabTitle: "Example"}
}

func (f *fake) deps() *Deps {
	return &Deps{
		IsPilotActionAllowed: func() bool { return f.pilot },
		IsExtensionConnected: func() bool { return f.extension },
		GetTrackingStatus:    func() (bool, int, string) { return true, f.tabID, f.tabURL },
		GetTrackedTabTitle:   func() string { return f.tabTitle },
		WaitForCommand: func(string, time.Duration) (*queries.CommandResult, bool) {
			return f.cmdResult, f.cmdFound
		},
		EnqueuePendingQuery: func(req mcp.JSONRPCRequest, q queries.PendingQuery, _ time.Duration) (mcp.JSONRPCResponse, bool) {
			if f.blockQueue {
				return mcp.Fail(req, mcp.ErrQueueFull, "queue full", "retry"), true
			}
			f.enqueued = append(f.enqueued, q)
			return mcp.JSONRPCResponse{}, false
		},
		RecordAIAction: func(action, _ string, _ map[string]any) { f.recorded = append(f.recorded, action) },
		RequireSessionStore: func(req mcp.JSONRPCRequest) (mcp.JSONRPCResponse, bool) {
			if f.noStore {
				return mcp.Fail(req, mcp.ErrNotInitialized, "no session store", "start the daemon"), true
			}
			return mcp.JSONRPCResponse{}, false
		},
		DiagnosticHint: func() func(*mcp.StructuredError) { return mcp.WithHint("diag") },
		Redact: func(m map[string]any) map[string]any {
			f.redactCalls++
			m["url"] = "[REDACTED]"
			return m
		},
	}
}

func newHandler(t *testing.T) (*Handler, *fake) {
	t.Helper()
	store, err := persistence.NewSessionStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}
	f := newFake()
	return New(f.deps(), store), f
}

func req() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, ClientID: "client-test"}
}

func payload(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal tool result: %v (raw=%s)", err, string(resp.Result))
	}
	if len(result.Content) == 0 {
		t.Fatal("tool result had no content blocks")
	}
	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatalf("no JSON payload in %q", text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("unmarshal payload: %v (raw=%s)", err, text[idx:])
	}
	return data
}

func TestHandleStateSave_RejectsNameAlias(t *testing.T) {
	h, _ := newHandler(t)
	resp := h.HandleStateSave(req(), json.RawMessage(`{"name":"legacy-snap"}`))
	if !strings.Contains(string(resp.Result), mcp.ErrMissingParam) {
		t.Fatalf("expected %s, got %s", mcp.ErrMissingParam, string(resp.Result))
	}
}

func TestStateSnapshotHandlers_RequireSnapshotName(t *testing.T) {
	t.Parallel()
	for name, call := range map[string]func(*Handler) mcp.JSONRPCResponse{
		"save": func(h *Handler) mcp.JSONRPCResponse {
			return h.HandleStateSave(req(), json.RawMessage(`{"name":"old"}`))
		},
		"load": func(h *Handler) mcp.JSONRPCResponse {
			return h.HandleStateLoad(req(), json.RawMessage(`{"name":"old"}`))
		},
		"delete": func(h *Handler) mcp.JSONRPCResponse {
			return h.HandleStateDelete(req(), json.RawMessage(`{"name":"old"}`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, _ := newHandler(t)
			resp := call(h)
			if !strings.Contains(string(resp.Result), mcp.ErrMissingParam) {
				t.Fatalf("expected %s, got %s", mcp.ErrMissingParam, string(resp.Result))
			}
		})
	}
}

func TestStateSnapshotHandlers_RejectInvalidJSON(t *testing.T) {
	t.Parallel()
	for name, call := range map[string]func(*Handler) mcp.JSONRPCResponse{
		"save": func(h *Handler) mcp.JSONRPCResponse {
			return h.HandleStateSave(req(), json.RawMessage(`bad`))
		},
		"load": func(h *Handler) mcp.JSONRPCResponse {
			return h.HandleStateLoad(req(), json.RawMessage(`bad`))
		},
		"delete": func(h *Handler) mcp.JSONRPCResponse {
			return h.HandleStateDelete(req(), json.RawMessage(`bad`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, _ := newHandler(t)
			resp := call(h)
			if !strings.Contains(string(resp.Result), mcp.ErrInvalidJSON) {
				t.Fatalf("expected %s, got %s", mcp.ErrInvalidJSON, string(resp.Result))
			}
		})
	}
}

func TestHandleStateSave_AcceptsSnapshotName(t *testing.T) {
	h, _ := newHandler(t)
	got := payload(t, h.HandleStateSave(req(), json.RawMessage(`{"snapshot_name":"new"}`)))
	if got["snapshot_name"] != "new" {
		t.Fatalf("snapshot_name = %v, want new", got["snapshot_name"])
	}
}

func TestHandleStateSave_MissingName(t *testing.T) {
	h, _ := newHandler(t)
	resp := h.HandleStateSave(req(), json.RawMessage(`{}`))
	if !strings.Contains(string(resp.Result), mcp.ErrMissingParam) {
		t.Fatalf("expected %s, got %s", mcp.ErrMissingParam, string(resp.Result))
	}
}

func TestHandleStateSave_RedactsBeforePersisting(t *testing.T) {
	h, f := newHandler(t)
	h.HandleStateSave(req(), json.RawMessage(`{"snapshot_name":"snap"}`))
	if f.redactCalls != 1 {
		t.Fatalf("Redact called %d times, want exactly 1 — snapshots must never reach disk unscrubbed", f.redactCalls)
	}

	loaded := payload(t, h.HandleStateLoad(req(), json.RawMessage(`{"snapshot_name":"snap"}`)))
	state, _ := loaded["state"].(map[string]any)
	if state["url"] != "[REDACTED]" {
		t.Fatalf("persisted url = %v, want the redacted value", state["url"])
	}
}

func TestHandleStateSave_CaptureStatusSurfacesPilotDisabled(t *testing.T) {
	h, f := newHandler(t)
	f.pilot = false
	got := payload(t, h.HandleStateSave(req(), json.RawMessage(`{"snapshot_name":"snap"}`)))
	if got["state_capture"] != act.StateCaptureStatusPilotDisabled {
		t.Fatalf("state_capture = %v, want %s", got["state_capture"], act.StateCaptureStatusPilotDisabled)
	}
}

func TestHandleStateSave_CaptureStatusSurfacesExtensionDown(t *testing.T) {
	h, f := newHandler(t)
	f.extension = false
	got := payload(t, h.HandleStateSave(req(), json.RawMessage(`{"snapshot_name":"snap"}`)))
	if got["state_capture"] != act.StateCaptureStatusExtensionDisconnected {
		t.Fatalf("state_capture = %v, want %s", got["state_capture"], act.StateCaptureStatusExtensionDisconnected)
	}
}

func TestCaptureState_StatusMapping(t *testing.T) {
	for _, tc := range []struct {
		name   string
		setup  func(f *fake)
		want   string
		queued bool
	}{
		{"no answer is a timeout", func(f *fake) { f.cmdFound = false }, act.StateCaptureStatusTimeout, true},
		{"still pending is a timeout", func(f *fake) {
			f.cmdFound, f.cmdResult = true, &queries.CommandResult{Status: "pending"}
		}, act.StateCaptureStatusTimeout, true},
		{"extension error", func(f *fake) {
			f.cmdFound, f.cmdResult = true, &queries.CommandResult{Status: "complete", Error: "boom"}
		}, act.StateCaptureStatusError, true},
		{"empty result", func(f *fake) {
			f.cmdFound, f.cmdResult = true, &queries.CommandResult{Status: "complete"}
		}, act.StateCaptureStatusError, true},
		{"unparseable payload", func(f *fake) {
			f.cmdFound, f.cmdResult = true, &queries.CommandResult{Status: "complete", Result: json.RawMessage(`not-json`)}
		}, act.StateCaptureStatusError, true},
		{"enqueue blocked never waits", func(f *fake) { f.blockQueue = true }, act.StateCaptureStatusError, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, f := newHandler(t)
			tc.setup(f)
			if got := h.CaptureState(req()); got.Status != tc.want {
				t.Fatalf("CaptureState().Status = %q, want %q", got.Status, tc.want)
			}
			if queued := len(f.enqueued) > 0; queued != tc.queued {
				t.Fatalf("enqueued = %v, want %v", queued, tc.queued)
			}
		})
	}
}

func TestHandleStateLoad_NotFound(t *testing.T) {
	h, _ := newHandler(t)
	resp := h.HandleStateLoad(req(), json.RawMessage(`{"snapshot_name":"absent"}`))
	if !strings.Contains(string(resp.Result), mcp.ErrNoData) {
		t.Fatalf("expected %s, got %s", mcp.ErrNoData, string(resp.Result))
	}
}

func TestHandleStateLoad_RestoreStatusBranches(t *testing.T) {
	withState := `{"form_values":{"#a":"x"}}`
	for _, tc := range []struct {
		name  string
		state string
		setup func(f *fake)
		want  string
	}{
		{"no restorable data", `{"url":"https://x.test/"}`, func(*fake) {}, act.StateRestoreStatusNoData},
		{"pilot disabled", withState, func(f *fake) { f.pilot = false }, act.StateRestoreStatusPilotDisabled},
		{"extension down", withState, func(f *fake) { f.extension = false }, act.StateRestoreStatusExtensionDown},
		{"queued", withState, func(*fake) {}, act.StateRestoreStatusQueued},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, err := persistence.NewSessionStore(t.TempDir(), nil)
			if err != nil {
				t.Fatalf("NewSessionStore() error = %v", err)
			}
			f := newFake()
			tc.setup(f)
			h := New(f.deps(), store)
			if err := store.Save(act.StateNamespace, "snap", []byte(tc.state)); err != nil {
				t.Fatalf("seed Save() error = %v", err)
			}

			got := payload(t, h.HandleStateLoad(req(), json.RawMessage(`{"snapshot_name":"snap"}`)))
			if got["state_restore"] != tc.want {
				t.Fatalf("state_restore = %v, want %v", got["state_restore"], tc.want)
			}
		})
	}
}

func TestHandleStateLoad_IncludeURLQueuesNavigation(t *testing.T) {
	h, f := newHandler(t)
	h.HandleStateSave(req(), json.RawMessage(`{"snapshot_name":"snap"}`))
	f.enqueued = nil

	got := payload(t, h.HandleStateLoad(req(), json.RawMessage(`{"snapshot_name":"snap","include_url":true}`)))
	state, _ := got["state"].(map[string]any)
	if state["navigation_queued"] != true {
		t.Fatalf("navigation_queued = %v, want true", state["navigation_queued"])
	}
	if len(f.enqueued) == 0 || f.enqueued[0].Type != "browser_action" {
		t.Fatalf("expected a browser_action navigation to be queued, got %+v", f.enqueued)
	}
}

func TestHandleStateLoad_WithoutIncludeURLDoesNotNavigate(t *testing.T) {
	h, f := newHandler(t)
	h.HandleStateSave(req(), json.RawMessage(`{"snapshot_name":"snap"}`))
	f.enqueued = nil

	got := payload(t, h.HandleStateLoad(req(), json.RawMessage(`{"snapshot_name":"snap"}`)))
	state, _ := got["state"].(map[string]any)
	if _, present := state["navigation_queued"]; present {
		t.Fatal("navigation_queued must be absent unless include_url was requested")
	}
}

func TestHandleStateList_ReportsMetadata(t *testing.T) {
	h, _ := newHandler(t)
	h.HandleStateSave(req(), json.RawMessage(`{"snapshot_name":"snap"}`))

	got := payload(t, h.HandleStateList(req(), nil))
	if got["count"] != float64(1) {
		t.Fatalf("count = %v, want 1", got["count"])
	}
	states, _ := got["states"].([]any)
	entry, _ := states[0].(map[string]any)
	if entry["name"] != "snap" {
		t.Fatalf("entry name = %v, want snap", entry["name"])
	}
	if entry["title"] != "Example" {
		t.Fatalf("entry title = %v, want the persisted title", entry["title"])
	}
	if entry["saved_at"] == nil {
		t.Fatal("entry is missing saved_at metadata")
	}
}

func TestHandleStateList_UnparseableSnapshotStillListed(t *testing.T) {
	store, err := persistence.NewSessionStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}
	f := newFake()
	h := New(f.deps(), store)
	if err := store.Save(act.StateNamespace, "corrupt", []byte(`not-json`)); err != nil {
		t.Fatalf("seed Save() error = %v", err)
	}

	got := payload(t, h.HandleStateList(req(), nil))
	states, _ := got["states"].([]any)
	if len(states) != 1 {
		t.Fatalf("states = %v, want the corrupt snapshot still listed by name", states)
	}
	if entry, _ := states[0].(map[string]any); entry["name"] != "corrupt" {
		t.Fatalf("entry = %v, want name=corrupt", states[0])
	}
}

func TestHandleStateDelete_RoundTrip(t *testing.T) {
	h, f := newHandler(t)
	h.HandleStateSave(req(), json.RawMessage(`{"snapshot_name":"snap"}`))

	got := payload(t, h.HandleStateDelete(req(), json.RawMessage(`{"snapshot_name":"snap"}`)))
	if got["status"] != "deleted" {
		t.Fatalf("status = %v, want deleted", got["status"])
	}

	again := h.HandleStateDelete(req(), json.RawMessage(`{"snapshot_name":"snap"}`))
	if !strings.Contains(string(again.Result), mcp.ErrNoData) {
		t.Fatalf("deleting twice should report %s, got %s", mcp.ErrNoData, string(again.Result))
	}

	wantRecorded := []string{"save_state", "delete_state"}
	if strings.Join(f.recorded, ",") != strings.Join(wantRecorded, ",") {
		t.Fatalf("recorded actions = %v, want %v", f.recorded, wantRecorded)
	}
}

func TestSessionStoreGateBlocksEveryAction(t *testing.T) {
	h, f := newHandler(t)
	f.noStore = true
	for name, resp := range map[string]mcp.JSONRPCResponse{
		"save":   h.HandleStateSave(req(), json.RawMessage(`{"snapshot_name":"s"}`)),
		"load":   h.HandleStateLoad(req(), json.RawMessage(`{"snapshot_name":"s"}`)),
		"list":   h.HandleStateList(req(), nil),
		"delete": h.HandleStateDelete(req(), json.RawMessage(`{"snapshot_name":"s"}`)),
	} {
		if !strings.Contains(string(resp.Result), mcp.ErrNotInitialized) {
			t.Errorf("%s: expected the session-store gate to block, got %s", name, string(resp.Result))
		}
	}
}
