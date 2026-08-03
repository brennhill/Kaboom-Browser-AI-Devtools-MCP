// Purpose: Tests for recording manager start, stop, and session lifecycle.
// Docs: docs/features/feature/playback-engine/index.md

// manager_test.go — Tests for RecordingManager lifecycle, validation, and actions.
package recording

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

type faultRecordingFilesystem struct {
	recordingFilesystem
	readErr    error
	readDirErr error
	writeErr   error
	statErr    error
	removeErr  error
	walkErr    error
}

func (files *faultRecordingFilesystem) ReadFile(path string) ([]byte, error) {
	if files.readErr != nil {
		return nil, files.readErr
	}
	return files.recordingFilesystem.ReadFile(path)
}

func (files *faultRecordingFilesystem) ReadDir(path string) ([]os.DirEntry, error) {
	if files.readDirErr != nil {
		return nil, files.readDirErr
	}
	return files.recordingFilesystem.ReadDir(path)
}

func (files *faultRecordingFilesystem) WriteFile(path string, data []byte, permissions os.FileMode) error {
	if files.writeErr != nil {
		return files.writeErr
	}
	return files.recordingFilesystem.WriteFile(path, data, permissions)
}

func (files *faultRecordingFilesystem) Stat(path string) (os.FileInfo, error) {
	if files.statErr != nil {
		return nil, files.statErr
	}
	return files.recordingFilesystem.Stat(path)
}

func (files *faultRecordingFilesystem) RemoveAll(path string) error {
	if files.removeErr != nil {
		return files.removeErr
	}
	return files.recordingFilesystem.RemoveAll(path)
}

func (files *faultRecordingFilesystem) Walk(root string, walkFn filepath.WalkFunc) error {
	if files.walkErr != nil {
		return files.walkErr
	}
	return files.recordingFilesystem.Walk(root, walkFn)
}

// ============================================
// NewRecordingManager Tests
// ============================================

func TestNewNewRecordingManager_Initialization(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	if mgr.recordings == nil {
		t.Fatal("recordings map should be initialized")
	}
	if len(mgr.recordings) != 0 {
		t.Errorf("recordings len = %d, want 0", len(mgr.recordings))
	}
	if mgr.activeRecordingID != "" {
		t.Errorf("activeRecordingID = %q, want empty", mgr.activeRecordingID)
	}
	if mgr.recordingStorageUsed != 0 {
		t.Errorf("recordingStorageUsed = %d, want 0", mgr.recordingStorageUsed)
	}
}

// ============================================
// ValidateRecordingID Tests
// ============================================

func TestNewValidateRecordingID_ValidID(t *testing.T) {
	t.Parallel()

	validIDs := []string{
		"my-recording-20240115T103000-123456789Z",
		"recording-20240101T000000-000000000Z",
		"test",
		"a",
		"recording-with-dashes",
	}

	for _, id := range validIDs {
		if err := ValidateRecordingID(id); err != nil {
			t.Errorf("ValidateRecordingID(%q) = %v, want nil", id, err)
		}
	}
}

func TestNewValidateRecordingID_EmptyID(t *testing.T) {
	t.Parallel()

	err := ValidateRecordingID("")
	if err == nil {
		t.Fatal("ValidateRecordingID('') should return error")
	}
	if !strings.Contains(err.Error(), "recording_id_empty") {
		t.Errorf("error = %q, want recording_id_empty prefix", err.Error())
	}
}

func TestNewValidateRecordingID_PathTraversal(t *testing.T) {
	t.Parallel()

	dangerous := []string{
		"../etc/passwd",
		"recording/../secret",
		"..\\windows\\system32",
		"recording/subdir",
		"recording\\subdir",
	}

	for _, id := range dangerous {
		err := ValidateRecordingID(id)
		if err == nil {
			t.Errorf("ValidateRecordingID(%q) should return error for path traversal", id)
			continue
		}
		if !strings.Contains(err.Error(), "recording_id_invalid") {
			t.Errorf("ValidateRecordingID(%q) error = %q, want recording_id_invalid", id, err.Error())
		}
	}
}

// ============================================
// StartRecording Tests
// ============================================

func TestNewStartRecording_WithName(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	id, err := mgr.StartRecording("login-flow", "https://example.com/login", false)
	if err != nil {
		t.Fatalf("StartRecording error = %v", err)
	}

	if !strings.HasPrefix(id, "login-flow-") {
		t.Errorf("id = %q, want prefix 'login-flow-'", id)
	}

	if mgr.recordings[id] == nil {
		t.Fatal("recording not stored in manager")
	}

	rec := mgr.recordings[id]
	if rec.ID != id {
		t.Errorf("rec.ID = %q, want %q", rec.ID, id)
	}
	if rec.Name != "login-flow" {
		t.Errorf("rec.Name = %q, want login-flow", rec.Name)
	}
	if rec.StartURL != "https://example.com/login" {
		t.Errorf("rec.StartURL = %q, want https://example.com/login", rec.StartURL)
	}
	if rec.SensitiveDataEnabled {
		t.Error("SensitiveDataEnabled should be false")
	}
	if rec.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}
	if rec.Viewport.Width != 1920 || rec.Viewport.Height != 1080 {
		t.Errorf("Viewport = %+v, want 1920x1080", rec.Viewport)
	}
	if rec.Actions == nil {
		t.Error("Actions should be initialized (not nil)")
	}

	if mgr.activeRecordingID != id {
		t.Errorf("activeRecordingID = %q, want %q", mgr.activeRecordingID, id)
	}
}

func TestNewStartRecording_WithoutName(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	id, err := mgr.StartRecording("", "https://example.com", false)
	if err != nil {
		t.Fatalf("StartRecording error = %v", err)
	}

	if !strings.HasPrefix(id, "recording-") {
		t.Errorf("id = %q, want prefix 'recording-' for auto-name", id)
	}
}

func TestNewStartRecording_SensitiveDataEnabled(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	id, err := mgr.StartRecording("test", "https://example.com", true)
	if err != nil {
		t.Fatalf("StartRecording error = %v", err)
	}

	rec := mgr.recordings[id]
	if !rec.SensitiveDataEnabled {
		t.Error("SensitiveDataEnabled should be true")
	}
}

func TestNewStartRecording_AlreadyRecording(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	_, err := mgr.StartRecording("first", "https://first.com", false)
	if err != nil {
		t.Fatalf("first StartRecording error = %v", err)
	}

	_, err = mgr.StartRecording("second", "https://second.com", false)
	if err == nil {
		t.Fatal("second StartRecording should fail when already recording")
	}
	if !strings.Contains(err.Error(), "already_recording") {
		t.Errorf("error = %q, want already_recording", err.Error())
	}
}

func TestNewStartRecording_StorageFull(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()
	mgr.recordingStorageUsed = RecordingStorageMax

	_, err := mgr.StartRecording("test", "https://example.com", false)
	if err == nil {
		t.Fatal("StartRecording should fail when storage is full")
	}
	if !strings.Contains(err.Error(), "recording_storage_full") {
		t.Errorf("error = %q, want recording_storage_full", err.Error())
	}
}

func TestNewStartRecording_UniqueIDs(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	ids := make(map[string]bool)
	for i := 0; i < 3; i++ {
		id, err := mgr.StartRecording(fmt.Sprintf("test-%d", i), "https://example.com", false)
		if err != nil {
			t.Fatalf("StartRecording[%d] error = %v", i, err)
		}
		if ids[id] {
			t.Fatalf("duplicate recording ID: %q", id)
		}
		ids[id] = true
		mgr.activeRecordingID = ""
	}
}

// ============================================
// StopRecording Tests
// ============================================

func TestNewStopRecording_Success(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	mgr := NewRecordingManager()

	id, err := mgr.StartRecording("test", "https://example.com", false)
	if err != nil {
		t.Fatalf("StartRecording error = %v", err)
	}

	mgr.AddRecordingAction(RecordingAction{Type: "click", Selector: "#btn"})
	mgr.AddRecordingAction(RecordingAction{Type: "type", Text: "hello"})

	actionCount, duration, err := mgr.StopRecording(id)
	if err != nil {
		t.Fatalf("StopRecording error = %v", err)
	}

	if actionCount != 2 {
		t.Errorf("actionCount = %d, want 2", actionCount)
	}
	if duration < 0 {
		t.Errorf("duration = %d, want >= 0", duration)
	}

	if mgr.activeRecordingID != "" {
		t.Errorf("activeRecordingID = %q, want empty after stop", mgr.activeRecordingID)
	}
}

func TestNewStopRecording_NotFound(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	_, _, err := mgr.StopRecording("nonexistent")
	if err == nil {
		t.Fatal("StopRecording should fail for nonexistent recording")
	}
	if !strings.Contains(err.Error(), "recording_not_found") {
		t.Errorf("error = %q, want recording_not_found", err.Error())
	}
}

// ============================================
// AddRecordingAction Tests
// ============================================

func TestNewAddRecordingAction_Success(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	id, err := mgr.StartRecording("test", "https://example.com", true)
	if err != nil {
		t.Fatalf("StartRecording error = %v", err)
	}

	err = mgr.AddRecordingAction(RecordingAction{
		Type:     "click",
		Selector: "#submit-btn",
		X:        100,
		Y:        200,
	})
	if err != nil {
		t.Fatalf("AddRecordingAction error = %v", err)
	}

	rec := mgr.recordings[id]
	if len(rec.Actions) != 1 {
		t.Fatalf("actions len = %d, want 1", len(rec.Actions))
	}

	action := rec.Actions[0]
	if action.Type != "click" {
		t.Errorf("Type = %q, want click", action.Type)
	}
	if action.Selector != "#submit-btn" {
		t.Errorf("Selector = %q, want #submit-btn", action.Selector)
	}
	if action.X != 100 {
		t.Errorf("X = %d, want 100", action.X)
	}
	if action.Y != 200 {
		t.Errorf("Y = %d, want 200", action.Y)
	}
}

func TestNewAddRecordingAction_NoActiveRecording(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()

	err := mgr.AddRecordingAction(RecordingAction{Type: "click"})
	if err == nil {
		t.Fatal("AddRecordingAction should fail when not recording")
	}
	if !strings.Contains(err.Error(), "not_recording") {
		t.Errorf("error = %q, want not_recording", err.Error())
	}
}

func TestNewAddRecordingAction_SensitiveDataRedaction(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()
	id, _ := mgr.StartRecording("test", "https://example.com", false)

	mgr.AddRecordingAction(RecordingAction{Type: "type", Text: "my-secret-password"})

	rec := mgr.recordings[id]
	if rec.Actions[0].Text != "[redacted]" {
		t.Errorf("Text = %q, want [redacted] (sensitive data disabled)", rec.Actions[0].Text)
	}
}

func TestNewAddRecordingAction_SensitiveDataPreserved(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()
	id, _ := mgr.StartRecording("test", "https://example.com", true)

	mgr.AddRecordingAction(RecordingAction{Type: "type", Text: "my-secret-password"})

	rec := mgr.recordings[id]
	if rec.Actions[0].Text != "my-secret-password" {
		t.Errorf("Text = %q, want my-secret-password (sensitive data enabled)", rec.Actions[0].Text)
	}
}

func TestNewAddRecordingAction_SetsTimestampIfMissing(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()
	id, _ := mgr.StartRecording("test", "https://example.com", true)

	before := time.Now().UnixMilli()
	mgr.AddRecordingAction(RecordingAction{Type: "click", TimestampMs: 0})
	after := time.Now().UnixMilli()

	rec := mgr.recordings[id]
	ts := rec.Actions[0].TimestampMs
	if ts < before || ts > after {
		t.Errorf("TimestampMs = %d, want between %d and %d", ts, before, after)
	}
}

func TestNewAddRecordingAction_PreservesExplicitTimestamp(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()
	id, _ := mgr.StartRecording("test", "https://example.com", true)

	mgr.AddRecordingAction(RecordingAction{Type: "click", TimestampMs: 1700000000000})

	rec := mgr.recordings[id]
	if rec.Actions[0].TimestampMs != 1700000000000 {
		t.Errorf("TimestampMs = %d, want 1700000000000 (preserved)", rec.Actions[0].TimestampMs)
	}
}

func TestNewAddRecordingAction_NonTypeActionNotRedacted(t *testing.T) {
	t.Parallel()

	mgr := NewRecordingManager()
	id, _ := mgr.StartRecording("test", "https://example.com", false)

	mgr.AddRecordingAction(RecordingAction{Type: "click", Text: "Click Me"})

	rec := mgr.recordings[id]
	if rec.Actions[0].Text != "Click Me" {
		t.Errorf("Text = %q, want 'Click Me' (click actions not redacted)", rec.Actions[0].Text)
	}
}

// ============================================
// CalculateRecordingSize Tests
// ============================================

func TestNewCalculateRecordingSize_EmptyRecording(t *testing.T) {
	t.Parallel()

	rec := &Recording{Name: "", Actions: []RecordingAction{}}

	size := CalculateRecordingSize(rec)
	if size < 500 {
		t.Errorf("size = %d, want >= 500 (base overhead)", size)
	}
}

func TestNewCalculateRecordingSize_WithActions(t *testing.T) {
	t.Parallel()

	rec := &Recording{
		Name:     "test-recording",
		StartURL: "https://example.com/page",
		TestID:   "test-123",
		Actions:  []RecordingAction{{Type: "click"}, {Type: "type"}, {Type: "navigate"}},
	}

	size := CalculateRecordingSize(rec)
	expectedMin := int64(len(rec.Name) + len(rec.StartURL) + len(rec.TestID) + 500 + 3*200)
	if size != expectedMin {
		t.Errorf("size = %d, want %d", size, expectedMin)
	}
}

func TestNewCalculateRecordingSize_LargeRecording(t *testing.T) {
	t.Parallel()

	actions := make([]RecordingAction, 100)
	for i := range actions {
		actions[i] = RecordingAction{Type: "click"}
	}

	rec := &Recording{Name: "large-test", StartURL: "https://example.com", Actions: actions}

	size := CalculateRecordingSize(rec)
	if size < 20000 {
		t.Errorf("size = %d, want >= 20000 for 100 actions", size)
	}
}

// ============================================
// GetRecording / LookupRecording Tests
// ============================================

// TestGetRecording_ValidatesIDAndReadsDisk covers the disk read path the
// log-diff engine reaches through its RecordingSource seam. It used to be
// covered incidentally by the log-diff tests, which now run against a fake.
func TestGetRecording_ValidatesIDAndReadsDisk(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())

	mgr := NewRecordingManager()

	if _, err := mgr.GetRecording("../escape"); err == nil ||
		!strings.Contains(err.Error(), "recording_id_invalid") {
		t.Fatalf("GetRecording(traversal) error = %v, want recording_id_invalid", err)
	}
	if _, err := mgr.GetRecording(""); err == nil ||
		!strings.Contains(err.Error(), "recording_id_empty") {
		t.Fatalf("GetRecording(empty) error = %v, want recording_id_empty", err)
	}
	if _, err := mgr.GetRecording("never-recorded"); err == nil ||
		!strings.Contains(err.Error(), "read_failed") {
		t.Fatalf("GetRecording(missing) error = %v, want read_failed", err)
	}

	id, err := mgr.StartRecording("disk-flow", "https://app.example.com", true)
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	if err := mgr.AddRecordingAction(RecordingAction{Type: "click", Selector: "#go", TimestampMs: 7}); err != nil {
		t.Fatalf("AddRecordingAction() error = %v", err)
	}
	if _, _, err := mgr.StopRecording(id); err != nil {
		t.Fatalf("StopRecording() error = %v", err)
	}

	got, err := mgr.GetRecording(id)
	if err != nil {
		t.Fatalf("GetRecording(%q) error = %v", id, err)
	}
	if got.ID != id || len(got.Actions) != 1 || got.Actions[0].Selector != "#go" {
		t.Fatalf("GetRecording(%q) = %+v, want the persisted single-click recording", id, got)
	}
}

func TestListRecordingsReportsMalformedMetadataAndKeepsValidSiblings(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	manager := NewRecordingManager()
	diagnostics := statediag.NewCollector()
	manager.SetDiagnostics(diagnostics)

	validID, err := manager.StartRecording("valid", "https://example.com", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.StopRecording(validID); err != nil {
		t.Fatal(err)
	}

	recordingsDir, err := state.RecordingsDir()
	if err != nil {
		t.Fatal(err)
	}
	brokenDir := filepath.Join(recordingsDir, "broken")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, recordingMetadataFile), []byte(`{"token":"secret"`), 0o600); err != nil {
		t.Fatal(err)
	}

	recordings, err := manager.ListRecordings(0)
	if err != nil {
		t.Fatalf("malformed sibling must not fail listing: %v", err)
	}
	if len(recordings) != 1 || recordings[0].ID != validID {
		t.Fatalf("recordings = %#v, want valid sibling only", recordings)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "event_recording_state" || got[0].Fix == "" {
		t.Fatalf("diagnostics = %#v, want actionable event-recording warning", got)
	}
	if strings.Contains(got[0].Detail, "secret") {
		t.Fatalf("diagnostic leaked metadata: %#v", got[0])
	}
	if got[0].Lifecycle != statediag.LifecycleActive {
		t.Fatalf("diagnostic lifecycle = %q, want active while malformed sibling remains", got[0].Lifecycle)
	}
	if err := os.RemoveAll(brokenDir); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ListRecordings(0); err != nil {
		t.Fatal(err)
	}
	got = diagnostics.Snapshot()
	if len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
		t.Fatalf("diagnostics = %#v, want recovered after a clean listing", got)
	}
}

// TestLookupRecording_PrefersInMemoryOverDisk pins the one behavior that makes
// LookupRecording distinct from GetRecording: the replay engine must see an
// in-flight recording that has not been written to disk yet.
func TestLookupRecording_PrefersInMemoryOverDisk(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())

	mgr := NewRecordingManager()

	id, err := mgr.StartRecording("live-flow", "https://app.example.com", true)
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	if err := mgr.AddRecordingAction(RecordingAction{Type: "type", Selector: "#q", Text: "unsaved"}); err != nil {
		t.Fatalf("AddRecordingAction() error = %v", err)
	}

	// Nothing is on disk yet, so only the in-memory branch can answer this.
	if _, err := mgr.GetRecording(id); err == nil {
		t.Fatal("GetRecording() should not see an unpersisted recording")
	}

	live, err := mgr.LookupRecording(id)
	if err != nil {
		t.Fatalf("LookupRecording(%q) error = %v", id, err)
	}
	if live == nil || len(live.Actions) != 1 || live.Actions[0].Text != "unsaved" {
		t.Fatalf("LookupRecording(%q) = %+v, want the in-memory recording", id, live)
	}

	if _, err := mgr.LookupRecording("never-recorded"); err == nil ||
		!strings.Contains(err.Error(), "read_failed") {
		t.Fatalf("LookupRecording(missing) error = %v, want read_failed from the disk fallback", err)
	}
}

func TestRecordingPersistenceFaultsRetainActiveRecordingUntilRetry(t *testing.T) {
	const private = "private-recording-value"
	for _, kind := range []statefault.Kind{
		statefault.Write,
		statefault.Sync,
		statefault.Rename,
		statefault.DirectorySync,
		statefault.Quota,
		statefault.PartialWrite,
		statefault.Cancellation,
	} {
		t.Run(string(kind), func(t *testing.T) {
			t.Setenv(state.StateDirEnv, t.TempDir())
			diagnostics := statediag.NewCollector()
			manager := NewRecordingManager()
			manager.SetDiagnostics(diagnostics)
			id, err := manager.StartRecording("fault", "https://private.example", false)
			if err != nil {
				t.Fatal(err)
			}
			previousFiles := manager.files
			manager.files = &faultRecordingFilesystem{
				recordingFilesystem: previousFiles,
				writeErr:            statefault.New(kind, private).Error(),
			}

			if _, _, err := manager.StopRecording(id); err == nil || strings.Contains(err.Error(), private) {
				t.Fatalf("StopRecording() error = %v, want redacted failure", err)
			}
			if manager.activeRecordingID != id {
				t.Fatalf("active recording = %q, want retained %q", manager.activeRecordingID, id)
			}
			got := diagnostics.Snapshot()
			if len(got) != 1 || got[0].Name != "event_recording_state" || got[0].Lifecycle != statediag.LifecycleActive || strings.Contains(got[0].Detail, private) {
				t.Fatalf("recording diagnostics = %#v", got)
			}

			manager.files = previousFiles
			if _, _, err := manager.StopRecording(id); err != nil {
				t.Fatal(err)
			}
			got = diagnostics.Snapshot()
			if len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
				t.Fatalf("resolved recording diagnostics = %#v", got)
			}
		})
	}
}

func TestRecordingReadAndListFaultsAreStableAndRecoverable(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	diagnostics := statediag.NewCollector()
	manager := NewRecordingManager()
	manager.SetDiagnostics(diagnostics)
	id, err := manager.StartRecording("read-fault", "https://example.com", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.StopRecording(id); err != nil {
		t.Fatal(err)
	}
	previousFiles := manager.files
	manager.files = &faultRecordingFilesystem{
		recordingFilesystem: previousFiles,
		readErr:             statefault.New(statefault.Read, "private-recording").Error(),
	}
	if _, err := manager.GetRecording(id); err == nil || strings.Contains(err.Error(), "private-recording") {
		t.Fatalf("GetRecording() error = %v", err)
	}
	manager.files = &faultRecordingFilesystem{
		recordingFilesystem: previousFiles,
		readDirErr:          statefault.New(statefault.Read, "private-recording").Error(),
	}
	if _, err := manager.ListRecordings(0); err == nil || strings.Contains(err.Error(), "private-recording") {
		t.Fatalf("ListRecordings() error = %v", err)
	}
	manager.files = previousFiles
	if _, err := manager.ListRecordings(0); err != nil {
		t.Fatal(err)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
		t.Fatalf("resolved read diagnostics = %#v", got)
	}
}

func TestStartRecordingNormalizesUnsafeNameBeforeBuildingStorageID(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	manager := NewRecordingManager()
	id, err := manager.StartRecording("../../private recording", "https://example.com", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRecordingID(id); err != nil {
		t.Fatalf("generated recording ID %q is unsafe: %v", id, err)
	}
	if strings.Contains(id, "private recording") || strings.ContainsAny(id, `/\\`) {
		t.Fatalf("generated recording ID was not normalized: %q", id)
	}
	if _, _, err := manager.StopRecording(id); err != nil {
		t.Fatal(err)
	}
}
