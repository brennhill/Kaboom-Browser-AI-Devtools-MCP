// session_store_test.go — Tests for session store operations.

package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestReadTouchesSkipsMalformedAndOversizedRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, touchesFile)
	valid := `{"t":"2026-01-01T00:00:00Z","tool":"Read","action":"read"}`
	content := "{bad json}\n" + strings.Repeat("x", maxTouchLinelen*2) + "\n" + valid + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadTouches(dir)
	if err == nil {
		t.Fatal("expected scanner error for oversized record")
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
