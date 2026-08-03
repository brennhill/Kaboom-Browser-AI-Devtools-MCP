// session_track_test.go — Tests for session tracking hook.

package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

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
