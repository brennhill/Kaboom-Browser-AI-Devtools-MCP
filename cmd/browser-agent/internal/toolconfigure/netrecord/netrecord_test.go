// netrecord_test.go — Unit tests for network recording state, filters, and the handler.

package netrecord

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// fakeBodyProvider implements NetworkBodyProvider.
type fakeBodyProvider struct {
	networkBodies []types.NetworkBody
}

func (f *fakeBodyProvider) GetNetworkBodies() []types.NetworkBody { return f.networkBodies }

// parseResp decodes an MCP tool result into (isError, text).
func parseResp(t *testing.T, resp mcp.JSONRPCResponse) (bool, string) {
	t.Helper()
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("failed to unmarshal result: %v (raw=%s)", err, string(resp.Result))
	}
	text := ""
	if len(r.Content) > 0 {
		text = r.Content[0].Text
	}
	return r.IsError, text
}

func newReq() mcp.JSONRPCRequest { return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1} }

// ---------------------------------------------------------------------------
// NetworkRecordingState
// ---------------------------------------------------------------------------

func TestNetworkRecordingState_TryStart(t *testing.T) {
	state := &NetworkRecordingState{}

	startTime, ok := state.TryStart("example.com", "POST")
	if !ok {
		t.Fatal("first TryStart should succeed")
	}
	if startTime.IsZero() {
		t.Error("startTime should not be zero")
	}

	// Second start should fail while active.
	_, ok2 := state.TryStart("other.com", "GET")
	if ok2 {
		t.Error("second TryStart should fail while already recording")
	}
}

func TestNetworkRecordingState_Stop(t *testing.T) {
	state := &NetworkRecordingState{}

	// Stop when not active should return false.
	_, ok := state.Stop()
	if ok {
		t.Error("Stop on inactive state should return false")
	}

	state.TryStart("example.com", "POST")
	snap, ok := state.Stop()
	if !ok {
		t.Fatal("Stop should succeed after TryStart")
	}
	if !snap.Active {
		t.Error("snapshot should show Active=true")
	}
	if snap.Domain != "example.com" {
		t.Errorf("Domain: want example.com, got %s", snap.Domain)
	}
	if snap.Method != "POST" {
		t.Errorf("Method: want POST, got %s", snap.Method)
	}

	// After stop, Info should show inactive.
	info := state.Info()
	if info.Active {
		t.Error("Info should show Active=false after Stop")
	}
}

func TestNetworkRecordingState_Info(t *testing.T) {
	state := &NetworkRecordingState{}
	info := state.Info()
	if info.Active {
		t.Error("new state should be inactive")
	}

	state.TryStart("test.com", "GET")
	info = state.Info()
	if !info.Active {
		t.Error("should be active after TryStart")
	}
	if info.Domain != "test.com" {
		t.Errorf("Domain: want test.com, got %s", info.Domain)
	}
}

// ---------------------------------------------------------------------------
// RecordingSnapshot
// ---------------------------------------------------------------------------

func TestRecordingSnapshot_Construction(t *testing.T) {
	snap := RecordingSnapshot{
		Active:    true,
		StartTime: time.Now(),
		Domain:    "api.example.com",
		Method:    "POST",
	}
	if !snap.Active {
		t.Error("expected Active=true")
	}
	if snap.Domain != "api.example.com" {
		t.Errorf("Domain: want api.example.com, got %s", snap.Domain)
	}
}

// ---------------------------------------------------------------------------
// Network recording filters
// ---------------------------------------------------------------------------

func TestMatchesRecordingFilter(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Minute)
	future := now.Add(1 * time.Minute)

	tests := []struct {
		name      string
		body      types.NetworkBody
		startTime time.Time
		domain    string
		method    string
		want      bool
	}{
		{
			name:      "matches all",
			body:      types.NetworkBody{Timestamp: future.Format(time.RFC3339Nano), URL: "https://example.com/api", Method: "GET"},
			startTime: now,
			domain:    "example.com",
			method:    "GET",
			want:      true,
		},
		{
			name:      "before start time",
			body:      types.NetworkBody{Timestamp: past.Format(time.RFC3339Nano), URL: "https://example.com/api", Method: "GET"},
			startTime: now,
			domain:    "",
			method:    "",
			want:      false,
		},
		{
			name:      "wrong domain",
			body:      types.NetworkBody{Timestamp: future.Format(time.RFC3339Nano), URL: "https://other.com/api", Method: "GET"},
			startTime: now,
			domain:    "example.com",
			method:    "",
			want:      false,
		},
		{
			name:      "wrong method",
			body:      types.NetworkBody{Timestamp: future.Format(time.RFC3339Nano), URL: "https://example.com/api", Method: "POST"},
			startTime: now,
			domain:    "",
			method:    "GET",
			want:      false,
		},
		{
			name:      "no filters, no timestamp",
			body:      types.NetworkBody{URL: "https://example.com/api", Method: "GET"},
			startTime: now,
			domain:    "",
			method:    "",
			want:      true,
		},
		{
			name:      "case insensitive domain",
			body:      types.NetworkBody{Timestamp: future.Format(time.RFC3339Nano), URL: "https://EXAMPLE.COM/api", Method: "GET"},
			startTime: now,
			domain:    "example.com",
			method:    "",
			want:      true,
		},
		{
			name:      "case insensitive method",
			body:      types.NetworkBody{Timestamp: future.Format(time.RFC3339Nano), URL: "https://example.com/api", Method: "get"},
			startTime: now,
			domain:    "",
			method:    "GET",
			want:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesRecordingFilter(tt.body, tt.startTime, tt.domain, tt.method)
			if got != tt.want {
				t.Errorf("MatchesRecordingFilter = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildRecordedRequestEntry(t *testing.T) {
	body := types.NetworkBody{
		Method:        "POST",
		URL:           "https://example.com/api",
		Status:        200,
		RequestBody:   `{"key":"value"}`,
		ResponseBody:  `{"ok":true}`,
		ContentType:   "application/json",
		Duration:      150,
		HasAuthHeader: true,
		Timestamp:     "2026-03-29T12:00:00Z",
	}
	entry := BuildRecordedRequestEntry(body)

	if entry["method"] != "POST" {
		t.Errorf("method: want POST, got %v", entry["method"])
	}
	if entry["url"] != "https://example.com/api" {
		t.Errorf("url mismatch")
	}
	if entry["status"] != 200 {
		t.Errorf("status: want 200, got %v", entry["status"])
	}
	if entry["request_body"] != `{"key":"value"}` {
		t.Error("request_body mismatch")
	}
	if entry["has_auth_header"] != true {
		t.Error("has_auth_header should be true")
	}
}

func TestBuildRecordedRequestEntry_MinimalFields(t *testing.T) {
	body := types.NetworkBody{Method: "GET", URL: "https://example.com", Status: 404}
	entry := BuildRecordedRequestEntry(body)

	if _, ok := entry["request_body"]; ok {
		t.Error("request_body should be omitted for empty body")
	}
	if _, ok := entry["response_body"]; ok {
		t.Error("response_body should be omitted for empty body")
	}
	if _, ok := entry["content_type"]; ok {
		t.Error("content_type should be omitted for empty body")
	}
	if _, ok := entry["has_auth_header"]; ok {
		t.Error("has_auth_header should be omitted when false")
	}
}

func TestCollectRecordedRequests(t *testing.T) {
	now := time.Now()
	future := now.Add(1 * time.Minute)

	bodies := []types.NetworkBody{
		{Timestamp: future.Format(time.RFC3339Nano), URL: "https://example.com/api", Method: "GET", Status: 200},
		{Timestamp: future.Format(time.RFC3339Nano), URL: "https://other.com/api", Method: "POST", Status: 201},
		{Timestamp: future.Format(time.RFC3339Nano), URL: "https://example.com/data", Method: "GET", Status: 200},
	}
	snap := RecordingSnapshot{
		Active:    true,
		StartTime: now,
		Domain:    "example.com",
		Method:    "",
	}

	result := CollectRecordedRequests(bodies, snap)
	if len(result) != 2 {
		t.Errorf("expected 2 recorded requests, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// HandleNetworkRecording
// ---------------------------------------------------------------------------

func TestHandleNetworkRecording(t *testing.T) {
	t.Run("status when inactive", func(t *testing.T) {
		d := &fakeBodyProvider{}
		state := &NetworkRecordingState{}
		resp := HandleNetworkRecording(d, state, newReq(), json.RawMessage(`{"operation":"status"}`))
		isErr, _ := parseResp(t, resp)
		if isErr {
			t.Fatal("status should not error")
		}
	})

	t.Run("empty operation is status", func(t *testing.T) {
		d := &fakeBodyProvider{}
		state := &NetworkRecordingState{}
		resp := HandleNetworkRecording(d, state, newReq(), nil)
		isErr, _ := parseResp(t, resp)
		if isErr {
			t.Fatal("empty operation should be treated as status")
		}
	})

	t.Run("start then start again errors", func(t *testing.T) {
		d := &fakeBodyProvider{}
		state := &NetworkRecordingState{}
		resp := HandleNetworkRecording(d, state, newReq(), json.RawMessage(`{"operation":"start","domain":"example.com","method":"POST"}`))
		if isErr, text := parseResp(t, resp); isErr {
			t.Fatalf("first start should succeed: %s", text)
		}
		// status while active should report active + filters
		statusResp := HandleNetworkRecording(d, state, newReq(), json.RawMessage(`{"operation":"status"}`))
		if isErr, _ := parseResp(t, statusResp); isErr {
			t.Fatal("status while active should not error")
		}
		resp2 := HandleNetworkRecording(d, state, newReq(), json.RawMessage(`{"operation":"start"}`))
		if isErr, _ := parseResp(t, resp2); !isErr {
			t.Fatal("second start should error (already active)")
		}
	})

	t.Run("stop when active returns recorded requests", func(t *testing.T) {
		future := "2999-01-01T00:00:00Z"
		d := &fakeBodyProvider{networkBodies: []types.NetworkBody{
			{Timestamp: future, URL: "https://example.com/api", Method: "POST", Status: 200},
		}}
		state := &NetworkRecordingState{}
		HandleNetworkRecording(d, state, newReq(), json.RawMessage(`{"operation":"start","domain":"example.com"}`))
		resp := HandleNetworkRecording(d, state, newReq(), json.RawMessage(`{"operation":"stop"}`))
		isErr, text := parseResp(t, resp)
		if isErr {
			t.Fatalf("stop should succeed: %s", text)
		}
	})

	t.Run("stop when inactive errors", func(t *testing.T) {
		d := &fakeBodyProvider{}
		state := &NetworkRecordingState{}
		resp := HandleNetworkRecording(d, state, newReq(), json.RawMessage(`{"operation":"stop"}`))
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("stop when inactive should error")
		}
	})

	t.Run("unknown operation errors", func(t *testing.T) {
		d := &fakeBodyProvider{}
		state := &NetworkRecordingState{}
		resp := HandleNetworkRecording(d, state, newReq(), json.RawMessage(`{"operation":"frobnicate"}`))
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("unknown operation should error")
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		d := &fakeBodyProvider{}
		state := &NetworkRecordingState{}
		resp := HandleNetworkRecording(d, state, newReq(), json.RawMessage(`{not json`))
		if isErr, _ := parseResp(t, resp); !isErr {
			t.Fatal("invalid JSON should error")
		}
	})
}
