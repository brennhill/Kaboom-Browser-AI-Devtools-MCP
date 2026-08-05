// Purpose: Tests durable hook session history, tracking decisions, and failure boundaries.
// Docs: docs/features/feature/session-tracking/index.md

package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

type faultTouchFile struct {
	kind  statefault.Kind
	steps *[]string
}

func (file *faultTouchFile) Write(data []byte) (int, error) {
	*file.steps = append(*file.steps, "write")
	if file.kind == statefault.PartialWrite {
		return len(data) / 2, nil
	}
	if file.kind == statefault.Write || file.kind == statefault.Quota || file.kind == statefault.Cancellation {
		return 0, statefault.New(file.kind, "private-touch").Error()
	}
	return len(data), nil
}

func (file *faultTouchFile) Sync() error {
	*file.steps = append(*file.steps, "sync")
	if file.kind == statefault.Sync {
		return statefault.New(file.kind, "private-touch").Error()
	}
	return nil
}

func (file *faultTouchFile) Close() error {
	*file.steps = append(*file.steps, "close")
	if file.kind == statefault.Restart {
		return statefault.New(file.kind, "private-touch").Error()
	}
	return nil
}

func TestAppendTouch_And_ReadTouches(t *testing.T) {
	dir := t.TempDir()

	entries := []TouchEntry{
		{Timestamp: time.Now().Add(-2 * time.Minute), Tool: "Read", File: "/project/a.go", Action: "read"},
		{Timestamp: time.Now().Add(-1 * time.Minute), Tool: "Edit", File: "/project/a.go", Action: "edit", Summary: "refactored"},
		{Timestamp: time.Now(), Tool: "Bash", Action: "bash", Summary: "go test ./..."},
	}

	for _, e := range entries {
		if err := AppendTouch(dir, e); err != nil {
			t.Fatalf("AppendTouch: %v", err)
		}
	}

	touches, err := ReadTouches(dir)
	if err != nil {
		t.Fatalf("ReadTouches: %v", err)
	}
	if len(touches) != 3 {
		t.Fatalf("expected 3 touches, got %d", len(touches))
	}

	// Should be newest-first.
	if touches[0].Tool != "Bash" {
		t.Errorf("expected newest first (Bash), got %s", touches[0].Tool)
	}
}

func TestAppendTouchReportsEveryDurabilityFailureWithoutLeakingState(t *testing.T) {
	for _, kind := range []statefault.Kind{
		statefault.Write,
		statefault.Sync,
		statefault.Quota,
		statefault.PartialWrite,
		statefault.Cancellation,
		statefault.Restart,
	} {
		t.Run(string(kind), func(t *testing.T) {
			steps := []string{}
			opener := func(string) (touchAppendFile, error) {
				return &faultTouchFile{kind: kind, steps: &steps}, nil
			}
			err := appendTouchWithOpener(t.TempDir(), TouchEntry{Tool: "Read", Action: "read", Summary: "private-touch"}, opener)
			if err == nil || strings.Contains(err.Error(), "private-touch") {
				t.Fatalf("append error = %v, want stable redacted failure", err)
			}
			if kind == statefault.PartialWrite && !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("partial write error = %v, want io.ErrShortWrite", err)
			}
		})
	}
}

func TestAppendTouchWritesSyncsAndClosesInOrder(t *testing.T) {
	steps := []string{}
	opener := func(string) (touchAppendFile, error) {
		return &faultTouchFile{steps: &steps}, nil
	}
	if err := appendTouchWithOpener(t.TempDir(), TouchEntry{Tool: "Read", Action: "read"}, opener); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(steps, ","); got != "write,sync,close" {
		t.Fatalf("durability order = %q", got)
	}
}

func TestReadTouches_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	touches, err := ReadTouches(dir)
	if err != nil {
		t.Fatalf("ReadTouches: %v", err)
	}
	if len(touches) != 0 {
		t.Fatalf("expected 0 touches, got %d", len(touches))
	}
}

func TestFilesEdited(t *testing.T) {
	dir := t.TempDir()
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Read", File: "/a.go", Action: "read"})
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Edit", File: "/b.go", Action: "edit"})
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Write", File: "/c.go", Action: "write"})
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Edit", File: "/b.go", Action: "edit"})

	files := FilesEdited(dir)
	if len(files) != 2 {
		t.Fatalf("expected 2 unique edited files, got %d: %v", len(files), files)
	}
}

func TestWasFileRead(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	_ = AppendTouch(dir, TouchEntry{Timestamp: now, Tool: "Read", File: "/project/a.go", Action: "read"})

	wasRead, when := WasFileRead(dir, "/project/a.go")
	if !wasRead {
		t.Error("expected file to have been read")
	}
	if when.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	wasRead, _ = WasFileRead(dir, "/project/b.go")
	if wasRead {
		t.Error("expected file NOT to have been read")
	}
}

func TestWasFileEdited(t *testing.T) {
	dir := t.TempDir()
	readAt := time.Now().Add(-5 * time.Minute)
	editAt := time.Now().Add(-2 * time.Minute)
	_ = AppendTouch(dir, TouchEntry{Timestamp: readAt, Tool: "Read", File: "/a.go", Action: "read"})
	_ = AppendTouch(dir, TouchEntry{Timestamp: editAt, Tool: "Edit", File: "/a.go", Action: "edit"})

	wasEdited, _ := WasFileEdited(dir, "/a.go", readAt)
	if !wasEdited {
		t.Error("expected file to have been edited after read")
	}

	wasEdited, _ = WasFileEdited(dir, "/a.go", time.Now())
	if wasEdited {
		t.Error("expected file NOT to have been edited after now")
	}
}

func TestSessionSummary(t *testing.T) {
	dir := t.TempDir()
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Read", File: "/a.go", Action: "read"})
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Read", File: "/b.go", Action: "read"})
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Edit", File: "/a.go", Action: "edit"})
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Bash", Action: "bash", Summary: "go test ./... PASS"})

	summary := SessionSummary(dir)
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if !containsStr(summary, "2 files read") {
		t.Errorf("expected '2 files read' in summary: %s", summary)
	}
	if !containsStr(summary, "1 edited") {
		t.Errorf("expected '1 edited' in summary: %s", summary)
	}
	if !containsStr(summary, "1 commands") {
		t.Errorf("expected '1 commands' in summary: %s", summary)
	}
}

func TestSessionSummary_Empty(t *testing.T) {
	dir := t.TempDir()
	summary := SessionSummary(dir)
	if summary != "" {
		t.Errorf("expected empty summary for empty session, got: %s", summary)
	}
}

func TestSessionID_Deterministic(t *testing.T) {
	id1 := SessionID()
	id2 := SessionID()
	if id1 != id2 {
		t.Errorf("SessionID not deterministic: %s != %s", id1, id2)
	}
	if len(id1) != 16 {
		t.Errorf("SessionID should be 16 chars, got %d: %s", len(id1), id1)
	}
}

func TestSessionID_GeminiEnv(t *testing.T) {
	t.Setenv("GEMINI_SESSION_ID", "test-gemini-session-1234567890")
	id := SessionID()
	if id != "test-gemini-sess" {
		t.Errorf("expected truncated Gemini session ID, got: %s", id)
	}
}

func TestSessionID_CodexAndShortIDs(t *testing.T) {
	t.Setenv("GEMINI_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "short")
	if got := SessionID(); got != "short" {
		t.Fatalf("SessionID = %q, want short Codex ID", got)
	}
}

func TestSessionIDHashesUnsafeAgentIdentifier(t *testing.T) {
	t.Setenv("GEMINI_SESSION_ID", "../../private/session")
	id := SessionID()
	if len(id) != 16 || strings.ContainsAny(id, `/\\.`) {
		t.Fatalf("unsafe SessionID() = %q", id)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := SessionDir()
	if err != nil {
		t.Fatal(err)
	}
	wantBase := filepath.Join(home, sessionBaseDir) + string(os.PathSeparator)
	if !strings.HasPrefix(dir, wantBase) {
		t.Fatalf("session directory escaped base: %q", dir)
	}
}

func TestSessionDirCreatesStableMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GEMINI_SESSION_ID", "metadata-session")

	dir, err := SessionDir()
	if err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		t.Fatal(err)
	}
	var meta sessionMeta
	if err := json.Unmarshal(first, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Cwd == "" || meta.Ppid == 0 || meta.StartTime.IsZero() {
		t.Fatalf("incomplete metadata: %+v", meta)
	}
	if _, err := SessionDir(); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("SessionDir rewrote existing metadata")
	}
}

func TestSessionDirRejectsCorruptMetadataWithStableRedactedError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GEMINI_SESSION_ID", "corrupt-session")
	dir := filepath.Join(home, sessionBaseDir, SessionID())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, metaFile), []byte(`{"private":"must-not-leak"`), 0o600); err != nil {
		t.Fatal(err)
	}

	gotDir, err := SessionDir()
	if gotDir != "" || err == nil || err.Error() != "session_metadata_corrupt" || strings.Contains(err.Error(), "must-not-leak") {
		t.Fatalf("SessionDir() = %q, %v", gotDir, err)
	}
}

func TestSessionDirReportsMetadataWriteFailure(t *testing.T) {
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
			home := t.TempDir()
			ops := defaultSessionDirectoryOperations()
			ops.userHomeDir = func() (string, error) { return home, nil }
			ops.writeFile = func(string, []byte, os.FileMode) error {
				return statefault.New(kind, "private-metadata").Error()
			}

			dir, err := sessionDirWithOperations(ops)
			if dir != "" || err == nil || err.Error() != "session_metadata_write_failed" || strings.Contains(err.Error(), "private-metadata") {
				t.Fatalf("sessionDirWithOperations() = %q, %v", dir, err)
			}
		})
	}
}

func TestSessionDirClassifiesEveryPersistenceBoundary(t *testing.T) {
	privateErr := errors.New("private persisted value")
	tests := []struct {
		name string
		want string
		edit func(*sessionDirectoryOperations, string)
	}{
		{name: "home", want: "session_home_directory_failed", edit: func(ops *sessionDirectoryOperations, _ string) {
			ops.userHomeDir = func() (string, error) { return "", privateErr }
		}},
		{name: "mkdir", want: "session_directory_create_failed", edit: func(ops *sessionDirectoryOperations, _ string) {
			ops.mkdirAll = func(string, os.FileMode) error { return privateErr }
		}},
		{name: "stat", want: "session_metadata_stat_failed", edit: func(ops *sessionDirectoryOperations, _ string) {
			ops.stat = func(string) (os.FileInfo, error) { return nil, privateErr }
		}},
		{name: "read", want: "session_metadata_read_failed", edit: func(ops *sessionDirectoryOperations, home string) {
			ops.stat = func(string) (os.FileInfo, error) { return os.Stat(home) }
			ops.readFile = func(string) ([]byte, error) { return nil, privateErr }
		}},
		{name: "working directory", want: "session_working_directory_failed", edit: func(ops *sessionDirectoryOperations, _ string) {
			ops.getwd = func() (string, error) { return "", privateErr }
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			ops := defaultSessionDirectoryOperations()
			ops.userHomeDir = func() (string, error) { return home, nil }
			tc.edit(&ops, home)
			path, err := sessionDirWithOperations(ops)
			if path != "" || err == nil || err.Error() != tc.want || strings.Contains(err.Error(), "private") {
				t.Fatalf("sessionDirWithOperations() = %q, %v", path, err)
			}
		})
	}
}

func TestReadTouchesRejectsMalformedAndOversizedRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, touchesFile)
	valid := `{"t":"2026-01-01T00:00:00Z","tool":"Read","action":"read"}`
	content := "{bad json}\n" + strings.Repeat("x", maxTouchLinelen*2) + "\n" + valid + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadTouches(dir)
	if err == nil || err.Error() != "session_touch_log_corrupt" {
		t.Fatalf("ReadTouches() error = %v, want stable corruption error", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %v, want none before scanner failure", entries)
	}
}

func TestTouchQueryErrorPathsAndSummaries(t *testing.T) {
	badDir := filepath.Join(t.TempDir(), "missing")
	if files := FilesEdited(badDir); files != nil {
		t.Fatalf("FilesEdited = %v, want nil", files)
	}
	if _, _, found := LastBashResult(badDir); found {
		t.Fatal("LastBashResult unexpectedly found a command")
	}
	if found, _ := WasFileRead(badDir, "x"); found {
		t.Fatal("WasFileRead unexpectedly found a read")
	}
	if found, _ := WasFileEdited(badDir, "x", time.Time{}); found {
		t.Fatal("WasFileEdited unexpectedly found an edit")
	}

	dir := t.TempDir()
	if err := AppendTouch(dir, TouchEntry{Tool: "run_shell_command", Action: "bash", Summary: "tests FAIL " + strings.Repeat("x", 120)}); err != nil {
		t.Fatal(err)
	}
	command, summary, found := LastBashResult(dir)
	if !found || !strings.Contains(command, "FAIL") || summary != "" {
		t.Fatalf("LastBashResult = %q, %q, %v", command, summary, found)
	}
	got := SessionSummary(dir)
	if !strings.Contains(got, "Last test: FAIL") || !strings.Contains(got, "...") {
		t.Fatalf("SessionSummary = %q", got)
	}
}

func TestCleanStaleSessionsRemovesOnlyExpiredValidDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	base := filepath.Join(home, sessionBaseDir)
	for name, contents := range map[string]string{
		"stale":   `{"start_time":"` + time.Now().Add(-24*time.Hour).Format(time.RFC3339) + `"}`,
		"current": `{"start_time":"` + time.Now().Format(time.RFC3339) + `"}`,
		"invalid": `{`,
	} {
		dir := filepath.Join(base, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, metaFile), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "not-a-directory"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	CleanStaleSessions()
	if _, err := os.Stat(filepath.Join(base, "stale")); !os.IsNotExist(err) {
		t.Fatalf("stale session still exists: %v", err)
	}
	for _, name := range []string{"current", "invalid"} {
		if _, err := os.Stat(filepath.Join(base, name)); err != nil {
			t.Fatalf("%s session removed: %v", name, err)
		}
	}
}

func TestCleanStaleSessionsBoundsEachScan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	base := filepath.Join(home, sessionBaseDir)
	stale := `{"start_time":"` + time.Now().Add(-24*time.Hour).Format(time.RFC3339) + `"}`
	for index := 0; index < maxCleanupScan+5; index++ {
		dir := filepath.Join(base, fmt.Sprintf("session-%03d", index))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, metaFile), []byte(stale), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	CleanStaleSessions()
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("remaining sessions = %d, want 5 after bounded scan", len(entries))
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRunSessionTrack_FirstRead(t *testing.T) {
	dir := t.TempDir()
	input := Input{
		ToolName:  "Read",
		ToolInput: json.RawMessage(`{"file_path":"/project/foo.go"}`),
	}
	result := RunSessionTrack(input, dir)
	if result != nil {
		t.Errorf("expected nil result on first read, got: %s", result.FormatContext())
	}

	// Verify the touch was recorded.
	touches, _ := ReadTouches(dir)
	if len(touches) != 1 {
		t.Fatalf("expected 1 touch, got %d", len(touches))
	}
	if touches[0].File != "/project/foo.go" {
		t.Errorf("expected file /project/foo.go, got %s", touches[0].File)
	}
}

func TestRunSessionTrackSurfacesFailedTouchWithoutClaimingItWasRecorded(t *testing.T) {
	input := Input{ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"/private/file.go"}`)}
	result := runSessionTrack(input, t.TempDir(), ReadTouches, func(string, TouchEntry) error {
		return statefault.New(statefault.Write, "private-touch-value").Error()
	})
	if result == nil || result.Action != "persistence_failed" {
		t.Fatalf("failed touch result = %#v", result)
	}
	if !strings.Contains(result.Context, "session_touch_append_failed") || strings.Contains(result.Context, "private-touch-value") {
		t.Fatalf("failed touch context = %q", result.Context)
	}
}

func TestRunSessionTrackRejectsCorruptHistoryBeforeAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, touchesFile)
	if err := os.WriteFile(path, []byte(`{"private":"must-not-leak"`), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := RunSessionTrack(Input{ToolName: "Read", ToolInput: json.RawMessage(`{"file_path":"/private/file.go"}`)}, dir)
	if result == nil || result.Action != "persistence_failed" || !strings.Contains(result.Context, "session_touch_read_failed") || strings.Contains(result.Context, "must-not-leak") {
		t.Fatalf("corrupt history result = %#v", result)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("corrupt history was modified: before=%q after=%q err=%v", before, after, err)
	}
}

func TestRunSessionTrack_RedundantRead(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate with a prior read.
	_ = AppendTouch(dir, TouchEntry{
		Timestamp: time.Now().Add(-3 * time.Minute),
		Tool:      "Read",
		File:      "/project/foo.go",
		Action:    "read",
	})

	input := Input{
		ToolName:  "Read",
		ToolInput: json.RawMessage(`{"file_path":"/project/foo.go"}`),
	}
	result := RunSessionTrack(input, dir)
	if result == nil {
		t.Fatal("expected redundant read warning")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "[Session]") {
		t.Errorf("expected [Session] prefix in: %s", ctx)
	}
	if !strings.Contains(ctx, "You read this file") {
		t.Errorf("expected 'You read this file' in: %s", ctx)
	}
	if !strings.Contains(ctx, "No edits since") {
		t.Errorf("expected 'No edits since' in: %s", ctx)
	}
}

func TestRunSessionTrack_RedundantReadWithEdit(t *testing.T) {
	dir := t.TempDir()
	readAt := time.Now().Add(-5 * time.Minute)
	editAt := time.Now().Add(-2 * time.Minute)

	_ = AppendTouch(dir, TouchEntry{Timestamp: readAt, Tool: "Read", File: "/project/foo.go", Action: "read"})
	_ = AppendTouch(dir, TouchEntry{Timestamp: editAt, Tool: "Edit", File: "/project/foo.go", Action: "edit", Summary: "refactored"})

	input := Input{
		ToolName:  "Read",
		ToolInput: json.RawMessage(`{"file_path":"/project/foo.go"}`),
	}
	result := RunSessionTrack(input, dir)
	if result == nil {
		t.Fatal("expected redundant read warning with edit info")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "edited it") {
		t.Errorf("expected 'edited it' in: %s", ctx)
	}
}

func TestRunSessionTrack_EditInjectsSummary(t *testing.T) {
	dir := t.TempDir()
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Read", File: "/a.go", Action: "read"})
	_ = AppendTouch(dir, TouchEntry{Timestamp: time.Now(), Tool: "Read", File: "/b.go", Action: "read"})

	input := Input{
		ToolName:  "Edit",
		ToolInput: json.RawMessage(`{"file_path":"/a.go","new_string":"func Foo() {}"}`),
	}
	result := RunSessionTrack(input, dir)
	if result == nil {
		t.Fatal("expected session summary on edit")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "[Session]") {
		t.Errorf("expected [Session] prefix in: %s", ctx)
	}
	if !strings.Contains(ctx, "files read") {
		t.Errorf("expected 'files read' in: %s", ctx)
	}
}

func TestRunSessionTrack_BashRecorded(t *testing.T) {
	dir := t.TempDir()
	input := Input{
		ToolName:  "Bash",
		ToolInput: json.RawMessage(`{"command":"go test ./..."}`),
	}
	result := RunSessionTrack(input, dir)
	// Bash on empty session should not produce output.
	if result != nil {
		t.Errorf("unexpected output on first bash: %s", result.FormatContext())
	}

	// Verify touch was recorded.
	touches, _ := ReadTouches(dir)
	if len(touches) != 1 {
		t.Fatalf("expected 1 touch, got %d", len(touches))
	}
	if touches[0].Action != "bash" {
		t.Errorf("expected action 'bash', got %s", touches[0].Action)
	}
}

func TestClassifyAction(t *testing.T) {
	tests := []struct {
		tool   string
		expect string
	}{
		{"Read", "read"},
		{"read_file", "read"},
		{"Edit", "edit"},
		{"replace_in_file", "edit"},
		{"edit_file", "edit"},
		{"Write", "write"},
		{"write_file", "write"},
		{"Bash", "bash"},
		{"run_shell_command", "bash"},
		{"Unknown", "other"},
	}
	for _, tt := range tests {
		got := classifyAction(tt.tool)
		if got != tt.expect {
			t.Errorf("classifyAction(%q) = %q, want %q", tt.tool, got, tt.expect)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d      time.Duration
		expect string
	}{
		{30 * time.Second, "30 sec"},
		{5 * time.Minute, "5 min"},
		{2 * time.Hour, "2 hr"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.expect {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.expect)
		}
	}
}
