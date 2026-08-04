// Purpose: Tests for tool dispatch and handling.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package screenrec

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

func TestLoadAndFilterRecordingsReportsMalformedMetadata(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "broken_meta.json")
	if err := os.WriteFile(path, []byte(`{"token":"secret"`), 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := statediag.NewCollector()
	recordings, totalSize, _ := loadAndFilterRecordings([]string{path}, "", diagnostics)
	if len(recordings) != 0 || totalSize != 0 {
		t.Fatalf("fallback = %#v, %d; want empty", recordings, totalSize)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "saved_video_state" || got[0].Fix == "" {
		t.Fatalf("diagnostics = %#v, want actionable saved-video warning", got)
	}
	if strings.Contains(got[0].Detail, "secret") {
		t.Fatalf("diagnostic leaked metadata: %#v", got[0])
	}
}

func TestWriteVideoToDiskCleansPairOnCanonicalPersistenceFaults(t *testing.T) {
	const private = "private-video-state"
	for _, kind := range []statefault.Kind{
		statefault.Write, statefault.Sync, statefault.Rename, statefault.DirectorySync,
		statefault.Quota, statefault.PartialWrite, statefault.Cancellation,
	} {
		t.Run(string(kind), func(t *testing.T) {
			dir := t.TempDir()
			ops := defaultVideoDiskOperations()
			if kind == statefault.Rename || kind == statefault.DirectorySync {
				ops.moveFile = func(string, string) error { return statefault.New(kind, private).Error() }
			} else {
				ops.writeMetadata = func(string, []byte, os.FileMode) error { return statefault.New(kind, private).Error() }
			}
			meta := Metadata{Name: "fault-video"}
			path, err := writeVideoToDiskWithOperations(dir, &meta, strings.NewReader(private), ops)
			if path != "" || err == nil || strings.Contains(err.Error(), private) {
				t.Fatalf("write result path=%q err=%v", path, err)
			}
			entries, readErr := os.ReadDir(dir)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("orphaned recording files = %#v err=%v", entries, readErr)
			}
		})
	}
}

func TestHandleSaveReportsAndResolvesPairPersistenceFailure(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	diagnostics := statediag.NewCollector()
	ops := defaultVideoDiskOperations()
	ops.writeMetadata = func(string, []byte, os.FileMode) error {
		return statefault.New(statefault.PartialWrite, "private-video").Error()
	}
	metadata := `{"name":"doctor-video","created_at":"2026-01-01T00:00:00Z"}`
	failedRequest := buildRecordingSaveRequest(t, http.MethodPost, []byte("video"), metadata, "")
	failedResponse := httptest.NewRecorder()
	handleSaveWithOperations(failedResponse, failedRequest, nil, diagnostics, ops)
	if failedResponse.Code != http.StatusInternalServerError {
		t.Fatalf("failure status = %d", failedResponse.Code)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "saved_video_state" || got[0].Lifecycle != statediag.LifecycleActive || strings.Contains(got[0].Detail, "private-video") {
		t.Fatalf("failure diagnostics = %#v", got)
	}

	retryRequest := buildRecordingSaveRequest(t, http.MethodPost, []byte("video"), metadata, "")
	retryResponse := httptest.NewRecorder()
	HandleSave(retryResponse, retryRequest, nil, diagnostics)
	if retryResponse.Code != http.StatusOK {
		t.Fatalf("retry status = %d body=%s", retryResponse.Code, retryResponse.Body.String())
	}
	got = diagnostics.Snapshot()
	if len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
		t.Fatalf("resolved diagnostics = %#v", got)
	}
}

// videoTestEnv drives screenrec against a real capture.Capture through the same
// Deps struct the host builds. It deliberately does NOT construct a Server or a
// ToolHandler: everything these tests assert on (pending queries, command
// results, pilot gating) is capture state, and building the god object only to
// reach it is what kept this feature pinned to package main.
type videoTestEnv struct {
	handler *InteractHandler
	capture *capture.Capture
}

func newVideoTestEnv(t *testing.T) *videoTestEnv {
	t.Helper()

	cap := capture.NewCapture()
	capturefixture.SetPilot(cap, false) // explicit default for pilot-disabled recording tests
	mockConnectedTrackedTab(t, cap)
	return &videoTestEnv{handler: NewInteractHandler(testDeps(cap)), capture: cap}
}

// testDeps mirrors the host's buildScreenrecDeps wiring, minus the
// cold-start wait: the gates read the same capture flags the real ones read.
func testDeps(cap *capture.Capture) Deps {
	return Deps{
		EnqueuePendingQuery: func(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool) {
			if _, err := cap.Queries().CreatePendingQueryWithTimeout(query, timeout, req.ClientID); err != nil {
				return mcp.Fail(req, mcp.ErrQueueFull, err.Error(), "Wait for in-flight commands to complete, then retry."), true
			}
			return mcp.JSONRPCResponse{}, false
		},
		RequirePilot: func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
			if cap.Extension().IsPilotActionAllowed() {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.Fail(req, mcp.ErrCodePilotDisabled, "AI Web Pilot is explicitly disabled",
				"Enable AI Web Pilot in the extension popup", opts...), true
		},
		RequireExtension: func(req mcp.JSONRPCRequest, opts ...func(*mcp.StructuredError)) (mcp.JSONRPCResponse, bool) {
			if cap.Extension().IsExtensionConnected() {
				return mcp.JSONRPCResponse{}, false
			}
			return mcp.Fail(req, mcp.ErrNotInitialized, "Browser extension is not connected",
				"Open a tab with the Kaboom extension enabled", opts...), true
		},
		RecordAIAction:   func(action, url string, extra map[string]any) {},
		DiagnosticHint:   func() func(*mcp.StructuredError) { return func(*mcp.StructuredError) {} },
		GetCommandResult: cap.Queries().GetCommandResult,
	}
}

func decodeMapResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v; body=%q", err, rr.Body.String())
	}
	return body
}

func buildRecordingSaveRequest(t *testing.T, method string, video []byte, metadata string, queryID string) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if video != nil {
		part, err := writer.CreateFormFile("video", "recording.webm")
		if err != nil {
			t.Fatalf("CreateFormFile() error = %v", err)
		}
		if _, err := part.Write(video); err != nil {
			t.Fatalf("write video part error = %v", err)
		}
	}
	if metadata != "" {
		if err := writer.WriteField("metadata", metadata); err != nil {
			t.Fatalf("WriteField(metadata) error = %v", err)
		}
	}
	if queryID != "" {
		if err := writer.WriteField("query_id", queryID); err != nil {
			t.Fatalf("WriteField(query_id) error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	req := httptest.NewRequest(method, "/recordings/save", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func writeVideoMetadataFile(t *testing.T, dir string, meta Metadata) {
	t.Helper()
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	path := filepath.Join(dir, meta.Name+"_meta.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func TestSanitizeVideoSlug(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"My Recording Name":        "my-recording-name",
		"caps_AND spaces 123":      "caps-and-spaces-123",
		"___":                      "recording",
		"a---b---c":                "a-b-c",
		"  already-safe-value  ":   "already-safe-value",
		"unicode-\u2603-not-allow": "unicode-not-allow",
	}

	for in, want := range cases {
		got := sanitizeVideoSlug(in)
		if got != want {
			t.Fatalf("sanitizeVideoSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPathWithinDir(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(os.PathSeparator), "tmp", "kaboom")

	if !pathWithinDir(filepath.Join(root, "recordings", "a.webm"), root) {
		t.Fatal("expected child path to be within root")
	}
	if pathWithinDir(filepath.Join(root, "..", "outside.webm"), root) {
		t.Fatal("expected parent traversal path to be rejected")
	}
	// Same directory should be within
	if !pathWithinDir(root, root) {
		t.Fatal("expected same dir to be within itself")
	}
	// Direct parent should be rejected
	if pathWithinDir(filepath.Dir(root), root) {
		t.Fatal("expected direct parent to be rejected")
	}
	// Deeply nested child should be within
	if !pathWithinDir(filepath.Join(root, "a", "b", "c", "d.webm"), root) {
		t.Fatal("expected deeply nested child to be within root")
	}
}

func TestRecordingsReadDirsUsesCanonicalDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	primary, err := state.RecordingsDir()
	if err != nil {
		t.Fatalf("state.RecordingsDir() error = %v", err)
	}
	if err := os.MkdirAll(primary, 0o755); err != nil {
		t.Fatalf("MkdirAll(primary) error = %v", err)
	}

	dirs := ReadDirs()
	if len(dirs) != 1 {
		t.Fatalf("ReadDirs() len = %d, want 1", len(dirs))
	}
	if dirs[0] != primary {
		t.Fatalf("ReadDirs()[0] = %q, want primary %q", dirs[0], primary)
	}
}

func TestHandleVideoRecordingSaveValidationAndSuccess(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	env := newVideoTestEnv(t)

	// Method guard.
	methodReq := httptest.NewRequest(http.MethodGet, "/recordings/save", nil)
	methodRR := httptest.NewRecorder()
	HandleSave(methodRR, methodReq, env.capture.Queries(), nil)
	if methodRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method guard status = %d, want 405", methodRR.Code)
	}

	// Missing metadata.
	missingMetaReq := buildRecordingSaveRequest(t, http.MethodPost, []byte("video-bytes"), "", "")
	missingMetaRR := httptest.NewRecorder()
	HandleSave(missingMetaRR, missingMetaReq, env.capture.Queries(), nil)
	if missingMetaRR.Code != http.StatusBadRequest {
		t.Fatalf("missing metadata status = %d, want 400", missingMetaRR.Code)
	}

	// Invalid metadata JSON.
	invalidMetaReq := buildRecordingSaveRequest(t, http.MethodPost, []byte("video-bytes"), "{bad json", "")
	invalidMetaRR := httptest.NewRecorder()
	HandleSave(invalidMetaRR, invalidMetaReq, env.capture.Queries(), nil)
	if invalidMetaRR.Code != http.StatusBadRequest {
		t.Fatalf("invalid metadata status = %d, want 400", invalidMetaRR.Code)
	}

	// Path traversal in name should be rejected.
	traversalMeta := `{"name":"../escape","created_at":"2026-01-01T00:00:00Z"}`
	traversalReq := buildRecordingSaveRequest(t, http.MethodPost, []byte("video-bytes"), traversalMeta, "")
	traversalRR := httptest.NewRecorder()
	HandleSave(traversalRR, traversalReq, env.capture.Queries(), nil)
	if traversalRR.Code != http.StatusBadRequest {
		t.Fatalf("path traversal status = %d, want 400", traversalRR.Code)
	}

	// Successful save with query result callback.
	okMeta := `{"name":"e2e-checkout","display_name":"Checkout","created_at":"2026-01-01T00:00:00Z","duration_seconds":7,"url":"https://app.example.com/checkout"}`
	req := buildRecordingSaveRequest(t, http.MethodPost, []byte("video-bytes-123"), okMeta, "query-1")
	rr := httptest.NewRecorder()
	HandleSave(rr, req, env.capture.Queries(), nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("success status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	resp := decodeMapResponse(t, rr)
	if resp["status"] != "saved" {
		t.Fatalf("response status = %v, want saved", resp["status"])
	}
	responsePath, ok := resp["path"].(string)
	if !ok || responsePath == "" {
		t.Fatalf("response path = %v, want non-empty string", resp["path"])
	}

	Dir, err := state.RecordingsDir()
	if err != nil {
		t.Fatalf("state.RecordingsDir() error = %v", err)
	}
	videoPath := responsePath
	if !pathWithinDir(videoPath, Dir) {
		t.Fatalf("video path %q is outside recordings dir %q", videoPath, Dir)
	}
	metaPath := strings.TrimSuffix(videoPath, ".webm") + "_meta.json"

	videoData, err := os.ReadFile(videoPath) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("video file missing: %v", err)
	}
	if string(videoData) != "video-bytes-123" {
		t.Fatalf("video file content = %q, want %q", string(videoData), "video-bytes-123")
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("metadata file missing: %v", err)
	}

	queryResult, found := env.capture.Queries().TakeQueryResult("query-1")
	if !found {
		t.Fatal("expected query result to be set for query-1")
	}
	var qr map[string]any
	if err := json.Unmarshal(queryResult, &qr); err != nil {
		t.Fatalf("query result JSON parse failed: %v", err)
	}
	if qr["status"] != "saved" || qr["name"] != "e2e-checkout" {
		t.Fatalf("unexpected query result payload: %+v", qr)
	}
}

func TestHandleVideoRecordingSaveRejectsOversizedUpload(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	env := newVideoTestEnv(t)

	originalLimit := MaxUploadSizeBytes
	MaxUploadSizeBytes = 1024
	t.Cleanup(func() { MaxUploadSizeBytes = originalLimit })

	largeVideo := bytes.Repeat([]byte("a"), 2048)
	meta := `{"name":"oversized-recording","created_at":"2026-01-01T00:00:00Z"}`
	req := buildRecordingSaveRequest(t, http.MethodPost, largeVideo, meta, "")
	rr := httptest.NewRecorder()
	HandleSave(rr, req, env.capture.Queries(), nil)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload status = %d, want %d (body=%q)", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
	}
}

func TestHandleRevealRecordingValidation(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	// Method guard.
	req := httptest.NewRequest(http.MethodGet, "/recordings/reveal", nil)
	rr := httptest.NewRecorder()
	HandleReveal(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method guard status = %d, want 405", rr.Code)
	}

	// Invalid JSON.
	invalidReq := httptest.NewRequest(http.MethodPost, "/recordings/reveal", strings.NewReader("{bad"))
	invalidRR := httptest.NewRecorder()
	HandleReveal(invalidRR, invalidReq)
	if invalidRR.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status = %d, want 400", invalidRR.Code)
	}

	// Missing path.
	missingReq := httptest.NewRequest(http.MethodPost, "/recordings/reveal", strings.NewReader(`{}`))
	missingRR := httptest.NewRecorder()
	HandleReveal(missingRR, missingReq)
	if missingRR.Code != http.StatusBadRequest {
		t.Fatalf("missing path status = %d, want 400", missingRR.Code)
	}

	// Forbidden path outside recordings directory.
	forbiddenReq := httptest.NewRequest(http.MethodPost, "/recordings/reveal", strings.NewReader(`{"path":"/tmp/not-allowed.webm"}`))
	forbiddenRR := httptest.NewRecorder()
	HandleReveal(forbiddenRR, forbiddenReq)
	if forbiddenRR.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d, want 403; body=%s", forbiddenRR.Code, forbiddenRR.Body.String())
	}
}

func TestToolObserveSavedVideosListsSortsFiltersAndDedupes(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	primaryDir, err := state.RecordingsDir()
	if err != nil {
		t.Fatalf("state.RecordingsDir() error = %v", err)
	}
	if err := os.MkdirAll(primaryDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(primary) error = %v", err)
	}

	writeVideoMetadataFile(t, primaryDir, Metadata{
		Name:      "alpha",
		CreatedAt: "2026-01-01T00:00:00Z",
		URL:       "https://app.example.com/alpha",
		SizeBytes: 10,
	})
	writeVideoMetadataFile(t, primaryDir, Metadata{
		Name:      "beta",
		CreatedAt: "2026-01-02T00:00:00Z",
		URL:       "https://app.example.com/beta",
		SizeBytes: 20,
	})
	// Malformed file should be skipped.
	if err := os.WriteFile(filepath.Join(primaryDir, "bad_meta.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatalf("WriteFile(bad_meta.json) error = %v", err)
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	resp := HandleObserveSavedVideos(req, json.RawMessage(`{}`), nil)
	toolResult := parseToolResult(t, resp)
	data := parseResponseJSON(t, toolResult)

	if got := int(data["total"].(float64)); got != 2 {
		t.Fatalf("total = %d, want 2 (alpha + beta deduped)", got)
	}
	if got := int(data["storage_used_bytes"].(float64)); got != 30 {
		t.Fatalf("storage_used_bytes = %d, want 30", got)
	}

	recordings, ok := data["recordings"].([]any)
	if !ok || len(recordings) != 2 {
		t.Fatalf("recordings = %#v, want 2 entries", data["recordings"])
	}

	first, ok := recordings[0].(map[string]any)
	if !ok {
		t.Fatalf("recordings[0] type = %T, want map[string]any", recordings[0])
	}
	if first["name"] != "beta" {
		t.Fatalf("first sorted recording name = %v, want beta", first["name"])
	}

	// Filter down to alpha and enforce last_n.
	filteredResp := HandleObserveSavedVideos(req, json.RawMessage(`{"url":"alpha","last_n":1}`), nil)
	filteredResult := parseToolResult(t, filteredResp)
	filtered := parseResponseJSON(t, filteredResult)

	if got := int(filtered["total"].(float64)); got != 1 {
		t.Fatalf("filtered total = %d, want 1", got)
	}
	filteredRecords := filtered["recordings"].([]any)
	rec0 := filteredRecords[0].(map[string]any)
	if rec0["name"] != "alpha" {
		t.Fatalf("filtered recording name = %v, want alpha", rec0["name"])
	}
}

// ============================================
// revealInFileManager
// ============================================
//
// These tests intentionally avoid executing platform commands because invoking
// Finder/Explorer during `go test` is disruptive and unnecessary for logic coverage.

func TestRevealCommandForOS(t *testing.T) {
	t.Parallel()

	path := "/tmp/recordings/demo.webm"
	cases := []struct {
		name     string
		goos     string
		wantCmd  string
		wantArgs []string
	}{
		{
			name:     "darwin",
			goos:     "darwin",
			wantCmd:  "open",
			wantArgs: []string{"-R", path},
		},
		{
			name:     "windows",
			goos:     "windows",
			wantCmd:  "explorer",
			wantArgs: []string{"/select,", path},
		},
		{
			name:     "linux-default",
			goos:     "linux",
			wantCmd:  "xdg-open",
			wantArgs: []string{filepath.Dir(path)},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotCmd, gotArgs := revealCommandForOS(tc.goos, path)
			if gotCmd != tc.wantCmd {
				t.Fatalf("revealCommandForOS(%q) cmd = %q, want %q", tc.goos, gotCmd, tc.wantCmd)
			}
			if len(gotArgs) != len(tc.wantArgs) {
				t.Fatalf("revealCommandForOS(%q) args len = %d, want %d", tc.goos, len(gotArgs), len(tc.wantArgs))
			}
			for i := range gotArgs {
				if gotArgs[i] != tc.wantArgs[i] {
					t.Fatalf("revealCommandForOS(%q) args[%d] = %q, want %q", tc.goos, i, gotArgs[i], tc.wantArgs[i])
				}
			}
		})
	}
}

func TestRevealInFileManagerWithRunner_UsesRunner(t *testing.T) {
	t.Parallel()

	path := "/tmp/recordings/demo.webm"
	var gotCmd string
	var gotArgs []string

	err := revealInFileManagerWithRunner("darwin", path, func(name string, args ...string) error {
		gotCmd = name
		gotArgs = append([]string(nil), args...)
		return nil
	})
	if err != nil {
		t.Fatalf("revealInFileManagerWithRunner() error = %v, want nil", err)
	}
	if gotCmd != "open" {
		t.Fatalf("runner cmd = %q, want open", gotCmd)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "-R" || gotArgs[1] != path {
		t.Fatalf("runner args = %#v, want [-R %q]", gotArgs, path)
	}
}

func TestRevealInFileManagerWithRunner_PropagatesRunnerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("runner failed")
	err := revealInFileManagerWithRunner("darwin", "/tmp/recordings/demo.webm", func(_ string, _ ...string) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("revealInFileManagerWithRunner() error = %v, want %v", err, wantErr)
	}
}

func TestHandleRecordStartAndStop(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	env := newVideoTestEnv(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 99, ClientID: "client-a"}

	disabled := env.handler.HandleRecordStart(req, json.RawMessage(`{"name":"x"}`))
	disabledResult := parseToolResult(t, disabled)
	if !disabledResult.IsError {
		t.Fatal("expected screen_recording_start to fail when pilot is disabled")
	}

	capturefixture.SetPilot(env.capture, true)

	invalidAudio := env.handler.HandleRecordStart(req, json.RawMessage(`{"audio":"speaker"}`))
	invalidAudioResult := parseToolResult(t, invalidAudio)
	if !invalidAudioResult.IsError {
		t.Fatal("expected invalid audio mode to return error")
	}

	startResp := env.handler.HandleRecordStart(req, json.RawMessage(`{"name":"My Video","fps":120,"audio":"tab","tab_id":7}`))
	startResult := parseToolResult(t, startResp)
	startData := parseResponseJSON(t, startResult)

	if startData["status"] != "queued" {
		t.Fatalf("screen_recording_start status = %v, want queued", startData["status"])
	}
	if startData["recording_state"] != recordingStateAwaitingGesture {
		t.Fatalf("screen_recording_start recording_state = %v, want %q", startData["recording_state"], recordingStateAwaitingGesture)
	}
	if startData["requires_user_gesture"] != true {
		t.Fatalf("screen_recording_start requires_user_gesture = %v, want true", startData["requires_user_gesture"])
	}
	userPrompt, _ := startData["user_prompt"].(string)
	if !strings.Contains(strings.ToLower(userPrompt), "open the kaboom popup") {
		t.Fatalf("screen_recording_start user_prompt = %q, want guidance to open popup and approve", userPrompt)
	}
	if int(startData["fps"].(float64)) != 60 {
		t.Fatalf("screen_recording_start fps = %v, want clamped 60", startData["fps"])
	}
	if startData["audio"] != "tab" {
		t.Fatalf("screen_recording_start audio = %v, want tab", startData["audio"])
	}
	if !strings.Contains(startData["name"].(string), "my-video--") {
		t.Fatalf("screen_recording_start name = %q, want sanitized timestamped name", startData["name"])
	}
	if !strings.HasSuffix(startData["path"].(string), ".webm") {
		t.Fatalf("screen_recording_start path = %q, want .webm suffix", startData["path"])
	}

	lastQuery := env.capture.Queries().GetLastPendingQuery()
	if lastQuery == nil {
		t.Fatal("expected pending query for screen_recording_start")
	}
	if lastQuery.Type != "screen_recording_start" || lastQuery.TabID != 7 {
		t.Fatalf("unexpected start query: %+v", *lastQuery)
	}
	paramsJSON, err := io.ReadAll(bytes.NewReader(lastQuery.Params))
	if err != nil {
		t.Fatalf("read start query params error: %v", err)
	}
	if !strings.Contains(string(paramsJSON), `"action":"screen_recording_start"`) {
		t.Fatalf("start query params = %s, want screen_recording_start action", string(paramsJSON))
	}

	stopBeforeReady := env.handler.HandleRecordStop(req, json.RawMessage(`{"tab_id":7}`))
	stopBeforeReadyResult := parseToolResult(t, stopBeforeReady)
	if !stopBeforeReadyResult.IsError {
		t.Fatal("screen_recording_stop should fail fast while screen_recording_start is still awaiting user gesture")
	}
	if !strings.Contains(strings.ToLower(stopBeforeReadyResult.Content[0].Text), recordingStateAwaitingGesture) {
		t.Fatalf("screen_recording_stop error should mention %q state, got: %s", recordingStateAwaitingGesture, stopBeforeReadyResult.Content[0].Text)
	}

	startCorrelationID, _ := startData["correlation_id"].(string)
	if startCorrelationID == "" {
		t.Fatal("screen_recording_start response missing correlation_id")
	}

	env.capture.Queries().ApplyCommandResult(startCorrelationID, "complete", json.RawMessage(`{"status":"recording","name":"My Video"}`), "")

	stopResp := env.handler.HandleRecordStop(req, json.RawMessage(`{"tab_id":7}`))
	stopResult := parseToolResult(t, stopResp)
	stopData := parseResponseJSON(t, stopResult)
	if stopData["status"] != "queued" {
		t.Fatalf("screen_recording_stop status = %v, want queued", stopData["status"])
	}
	if stopData["recording_state"] != recordingStateStopping {
		t.Fatalf("screen_recording_stop recording_state = %v, want %q", stopData["recording_state"], recordingStateStopping)
	}

	stopQuery := env.capture.Queries().GetLastPendingQuery()
	if stopQuery == nil {
		t.Fatal("expected pending query for screen_recording_stop")
	}
	if stopQuery.Type != "screen_recording_stop" || stopQuery.TabID != 7 {
		t.Fatalf("unexpected stop query: %+v", *stopQuery)
	}
}

// ============================================
// Local response assertions
// ============================================
//
// package main's tools_test_helpers_test.go cannot be imported across a package
// boundary, so the two helpers this file used come along as unexported copies.

func mockConnectedTrackedTab(t *testing.T, cap *capture.Capture) {
	t.Helper()
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	capture.NewSyncHandler(cap).HandleSync(httptest.NewRecorder(), httpReq)
	capturefixture.Track(cap, 42, "https://example.com")
}

func parseToolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("parseToolResult: %v; raw=%s", err, string(resp.Result))
	}
	return result
}

func parseResponseJSON(t *testing.T, result mcp.MCPToolResult) map[string]any {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("parseResponseJSON: no content blocks")
	}
	text := result.Content[0].Text
	for i, ch := range text {
		if ch == '{' || ch == '[' {
			text = text[i:]
			break
		}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("parseResponseJSON: %v; text=%q", err, text)
	}
	return data
}
