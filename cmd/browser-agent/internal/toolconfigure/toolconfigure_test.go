// toolconfigure_test.go — Unit tests for the toolconfigure sub-package exported API.

package toolconfigure

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ---------------------------------------------------------------------------
// Sequence types and constants
// ---------------------------------------------------------------------------

func TestSequenceNamePattern_Valid(t *testing.T) {
	valid := []string{"my-sequence", "login_flow", "TestA123", "a", "A-B_c"}
	for _, name := range valid {
		if !SequenceNamePattern.MatchString(name) {
			t.Errorf("expected %q to match SequenceNamePattern", name)
		}
	}
}

func TestSequenceNamePattern_Invalid(t *testing.T) {
	invalid := []string{"", "has space", "special!", "path/name", "tab\there"}
	for _, name := range invalid {
		if SequenceNamePattern.MatchString(name) {
			t.Errorf("expected %q to NOT match SequenceNamePattern", name)
		}
	}
}

func TestSequenceConstants(t *testing.T) {
	if MaxSequenceSteps != 50 {
		t.Errorf("MaxSequenceSteps: want 50, got %d", MaxSequenceSteps)
	}
	if MaxSequenceNameLen != 64 {
		t.Errorf("MaxSequenceNameLen: want 64, got %d", MaxSequenceNameLen)
	}
	if DefaultStepTimeout != 10000 {
		t.Errorf("DefaultStepTimeout: want 10000, got %d", DefaultStepTimeout)
	}
}

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
// Tutorial snippet catalog
// ---------------------------------------------------------------------------

func TestTutorialSnippets_NonEmpty(t *testing.T) {
	snippets := TutorialSnippets()
	if len(snippets) == 0 {
		t.Fatal("TutorialSnippets should return a non-empty slice")
	}
}

func TestTutorialSnippets_NoDuplicateGoals(t *testing.T) {
	snippets := TutorialSnippets()
	seen := make(map[string]bool)
	for _, s := range snippets {
		goal, ok := s["goal"].(string)
		if !ok || goal == "" {
			t.Error("snippet missing 'goal' string field")
			continue
		}
		if seen[goal] {
			t.Errorf("duplicate snippet goal: %s", goal)
		}
		seen[goal] = true
	}
}

func TestTutorialSnippets_RequiredFields(t *testing.T) {
	snippets := TutorialSnippets()
	for i, s := range snippets {
		for _, key := range []string{"tool", "goal", "snippet", "arguments"} {
			if _, ok := s[key]; !ok {
				t.Errorf("snippet %d missing required field %q", i, key)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// NormalizeTelemetryMode
// ---------------------------------------------------------------------------

func TestNormalizeTelemetryMode(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{"off", "off", true},
		{"auto", "auto", true},
		{"full", "full", true},
		{"invalid", "", false},
		{"", "", false},
		{"  off  ", "off", true},   // trims before validating AND before returning
		{"\tfull\n", "full", true}, // any surrounding whitespace, not just spaces
		{"  invalid  ", "", false}, // trimming must not make an invalid mode valid
		{"of f", "", false},        // interior whitespace is still invalid
		{"   ", "", false},         // whitespace-only trims to empty, which is invalid
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := NormalizeTelemetryMode(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok: want %v, got %v", tt.wantOK, ok)
			}
			if got != tt.want {
				t.Errorf("mode: want %q, got %q", tt.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tutorial playbooks
// ---------------------------------------------------------------------------

func TestTutorialSafeAutomationLoop_NonEmpty(t *testing.T) {
	playbook := TutorialSafeAutomationLoop()
	if len(playbook) == 0 {
		t.Fatal("TutorialSafeAutomationLoop should return a non-empty map")
	}
	if _, ok := playbook["title"]; !ok {
		t.Error("playbook should have a 'title' field")
	}
}

func TestTutorialCSPFallbackPlaybook_NonEmpty(t *testing.T) {
	playbook := TutorialCSPFallbackPlaybook()
	if len(playbook) == 0 {
		t.Fatal("TutorialCSPFallbackPlaybook should return a non-empty map")
	}
}
