// convention_detect_test.go — Tests for convention detection in quality gate hooks.

package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func setupConventionProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create a project with existing patterns.
	sub := filepath.Join(dir, "internal", "server")
	os.MkdirAll(sub, 0755)

	// File with http.Client pattern.
	os.WriteFile(filepath.Join(sub, "client.go"), []byte(`package server

import (
	"net/http"
	"time"
)

func newClient() *http.Client {
	client := &http.Client{Timeout: 5 * time.Second}
	return client
}
`), 0644)

	// Another file with http.Client pattern.
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import (
	"net/http"
	"time"
)

func healthCheck() {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	_ = client
}
`), 0644)

	// File with map[string]func pattern.
	os.WriteFile(filepath.Join(sub, "dispatch.go"), []byte(`package server

var handlers = map[string]func() int{
	"start": runStart,
	"stop":  runStop,
}
`), 0644)

	// File with type declaration.
	os.WriteFile(filepath.Join(sub, "types.go"), []byte(`package server

type ServerConfig struct {
	Port    int
	LogFile string
}
`), 0644)

	return dir
}

func TestDetectConventions_FindsHTTPClient(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "new_file.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	newContent := `client := &http.Client{Timeout: 1e9}`

	matches := DetectConventions(editedFile, dir, newContent)
	if len(matches) == 0 {
		t.Fatal("expected convention match for http.Client{")
	}

	found := false
	for _, m := range matches {
		if m.Pattern == "http.Client{" {
			found = true
			if len(m.Examples) < 2 {
				t.Errorf("expected at least 2 examples, got %d", len(m.Examples))
			}
		}
	}
	if !found {
		t.Error("expected http.Client{ pattern in matches")
	}
}

func TestDetectConventions_FindsHandlerMap(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "new_handler.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	newContent := `var routes = map[string]func()`

	matches := DetectConventions(editedFile, dir, newContent)
	found := false
	for _, m := range matches {
		if m.Pattern == "map[string]func" {
			found = true
		}
	}
	if !found {
		t.Error("expected map[string]func pattern in matches")
	}
}

func TestDetectConventions_DetectsDuplicateType(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "config.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	// Declare same type name that exists in types.go
	newContent := `type ServerConfig struct {
	Host string
}`

	matches := DetectConventions(editedFile, dir, newContent)
	found := false
	for _, m := range matches {
		if strings.Contains(m.Pattern, "ServerConfig") {
			found = true
			if len(m.Examples) == 0 {
				t.Error("expected example showing existing ServerConfig")
			}
		}
	}
	if !found {
		t.Error("expected duplicate type detection for ServerConfig")
	}
}

func TestDetectConventions_NoMatchesForNewPattern(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "new.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	newContent := `x := 1 + 2`

	matches := DetectConventions(editedFile, dir, newContent)
	if len(matches) != 0 {
		t.Errorf("expected no matches for plain arithmetic, got %d", len(matches))
	}
}

func TestDetectConventions_EmptyContent(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "new.go")

	matches := DetectConventions(editedFile, dir, "")
	if matches != nil {
		t.Error("expected nil for empty content")
	}
}

func TestDetectConventions_ExcludesEditedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Only one file in project, and it's the edited file.
	editedFile := filepath.Join(dir, "solo.go")
	os.WriteFile(editedFile, []byte(`package main
client := &http.Client{Timeout: time.Second}
`), 0644)

	newContent := `client := &http.Client{Timeout: time.Second}`

	matches := DetectConventions(editedFile, dir, newContent)
	if len(matches) != 0 {
		t.Error("should not match against the edited file itself")
	}
}

func TestDetectConventions_SkipsGeneratedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Generated file with pattern.
	os.WriteFile(filepath.Join(dir, "bundle.bundled.js"), []byte(`var client = http.Client{}`), 0644)
	// Edited file.
	editedFile := filepath.Join(dir, "new.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	newContent := `http.Client{`

	matches := DetectConventions(editedFile, dir, newContent)
	if len(matches) != 0 {
		t.Error("should not match generated/bundled files")
	}
}

func TestFormatConventions_HelperSuggestion(t *testing.T) {
	t.Parallel()
	matches := []ConventionMatch{
		{
			Pattern: "http.Client{",
			Examples: []string{
				"  internal/server/client.go:9: client := &http.Client{Timeout: 5 * time.Second}",
				"  main.go:9: client := &http.Client{Timeout: 500 * time.Millisecond}",
			},
		},
	}

	result := FormatConventions(matches)

	if !strings.Contains(result, "CODEBASE CONVENTIONS") {
		t.Error("missing header")
	}
	if !strings.Contains(result, "http.Client{") {
		t.Error("missing pattern name")
	}
	if !strings.Contains(result, "SUGGESTION") {
		t.Error("missing helper extraction suggestion for 2+ instances")
	}
	if !strings.Contains(result, "2 files") {
		t.Error("should reference the number of files")
	}
}

func TestFormatConventions_NoSuggestionForSingleInstance(t *testing.T) {
	t.Parallel()
	matches := []ConventionMatch{
		{
			Pattern:  "exec.Command(",
			Examples: []string{"  cmd/run.go:5: exec.Command(\"ls\")"},
		},
	}

	result := FormatConventions(matches)

	if strings.Contains(result, "SUGGESTION") {
		t.Error("should NOT suggest helper extraction for single instance")
	}
}

func TestFormatConventions_Empty(t *testing.T) {
	t.Parallel()
	result := FormatConventions(nil)
	if result != "" {
		t.Errorf("expected empty string for nil matches, got %q", result)
	}
}

func TestExtensionFamily(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ext  string
		want int
	}{
		{".go", 1},
		{".ts", 4},
		{".tsx", 4},
		{".js", 4},
		{".py", 1},
		{".rs", 1},
		{".rb", 1}, // unknown — returns same ext
	}
	for _, tt := range tests {
		exts := extensionFamily(tt.ext)
		if len(exts) != tt.want {
			t.Errorf("extensionFamily(%q) returned %d extensions, want %d", tt.ext, len(exts), tt.want)
		}
	}
}

func TestRunQualityGate_WithConventions(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)

	// Create .kaboom.json and standards doc.
	os.WriteFile(filepath.Join(dir, ".kaboom.json"), []byte(`{"code_standards":"standards.md","file_size_limit":800}`), 0644)
	os.WriteFile(filepath.Join(dir, "standards.md"), []byte("# Standards\n"), 0644)

	// Create a file that introduces http.Client{
	editedFile := filepath.Join(dir, "new_service.go")
	os.WriteFile(editedFile, []byte("package main\nimport \"net/http\"\nvar c = &http.Client{}\n"), 0644)

	// Simulate Edit tool input with new_string containing the pattern.
	input := Input{
		ToolName: "Edit",
		ToolInput: mustMarshal(map[string]string{
			"file_path":  editedFile,
			"new_string": `client := &http.Client{Timeout: 1e9}`,
		}),
	}

	result := RunQualityGate(input)
	if result == nil {
		t.Fatal("expected quality gate result")
	}
	if !strings.Contains(result.Context, "CODEBASE CONVENTIONS") {
		t.Error("expected convention detection in quality gate output")
	}
	if !strings.Contains(result.Context, "http.Client{") {
		t.Error("expected http.Client{ convention in output")
	}
}
