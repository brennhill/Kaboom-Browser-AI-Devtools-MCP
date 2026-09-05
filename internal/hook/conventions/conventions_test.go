// Purpose: Tests project convention discovery, caching, detection, and formatting.
// Docs: docs/features/feature/convention-engine/index.md

package conventions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// TestMain gives the package its own state root. Convention discovery persists
// its results under the Kaboom state directory, and without this every test run
// would deposit cache files in the developer's real ~/.kaboom and read back
// results a previous run had written.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "kaboom-hook-state-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot create test state root: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv(state.StateDirEnv, root); err != nil {
		fmt.Fprintf(os.Stderr, "cannot set test state root: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := os.RemoveAll(root); err != nil {
		fmt.Fprintf(os.Stderr, "cannot remove test state root: %v\n", err)
	}
	os.Exit(code)
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

func TestDetect_FindsHTTPClient(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "new_file.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	newContent := `client := &http.Client{Timeout: 1e9}`

	matches := Detect(editedFile, dir, newContent)
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

func TestDetect_FindsHandlerMap(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "new_handler.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	newContent := `var routes = map[string]func()`

	matches := Detect(editedFile, dir, newContent)
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

func TestDetect_DetectsDuplicateType(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "config.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	// Declare same type name that exists in types.go
	newContent := `type ServerConfig struct {
	Host string
}`

	matches := Detect(editedFile, dir, newContent)
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

func TestDetect_NoMatchesForNewPattern(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "new.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	newContent := `x := 1 + 2`

	matches := Detect(editedFile, dir, newContent)
	if len(matches) != 0 {
		t.Errorf("expected no matches for plain arithmetic, got %d", len(matches))
	}
}

func TestDetect_EmptyContent(t *testing.T) {
	t.Parallel()
	dir := setupConventionProject(t)
	editedFile := filepath.Join(dir, "new.go")

	matches := Detect(editedFile, dir, "")
	if matches != nil {
		t.Error("expected nil for empty content")
	}
}

func TestDetect_ExcludesEditedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Only one file in project, and it's the edited file.
	editedFile := filepath.Join(dir, "solo.go")
	os.WriteFile(editedFile, []byte(`package main
client := &http.Client{Timeout: time.Second}
`), 0644)

	newContent := `client := &http.Client{Timeout: time.Second}`

	matches := Detect(editedFile, dir, newContent)
	if len(matches) != 0 {
		t.Error("should not match against the edited file itself")
	}
}

func TestDetect_SkipsGeneratedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Generated file with pattern.
	os.WriteFile(filepath.Join(dir, "bundle.bundled.js"), []byte(`var client = http.Client{}`), 0644)
	// Edited file.
	editedFile := filepath.Join(dir, "new.go")
	os.WriteFile(editedFile, []byte("package main\n"), 0644)

	newContent := `http.Client{`

	matches := Detect(editedFile, dir, newContent)
	if len(matches) != 0 {
		t.Error("should not match generated/bundled files")
	}
}

func TestReadConventionSource_AppliesCanonicalScanFilters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	validPath := filepath.Join(dir, "valid.go")
	if err := os.WriteFile(validPath, []byte("package valid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	validEntry, err := os.Stat(validPath)
	if err != nil {
		t.Fatal(err)
	}
	if data, ok := readConventionSource(validPath, fileInfoDirEntry{validEntry}, []string{".go"}); !ok || string(data) != "package valid\n" {
		t.Fatalf("valid source = (%q, %v), want canonical file contents", data, ok)
	}

	for _, name := range []string{"wrong.ts", "bundle.bundled.go", "vendor.min.go", "source.go.map"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("ignored"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := readConventionSource(path, fileInfoDirEntry{info}, []string{".go"}); ok {
			t.Errorf("%s should be rejected by canonical convention scan filters", name)
		}
	}
}

type fileInfoDirEntry struct {
	os.FileInfo
}

func (entry fileInfoDirEntry) Type() os.FileMode          { return entry.Mode().Type() }
func (entry fileInfoDirEntry) Info() (os.FileInfo, error) { return entry.FileInfo, nil }

func TestFormat_HelperSuggestion(t *testing.T) {
	t.Parallel()
	matches := []Match{
		{
			Pattern: "http.Client{",
			Examples: []string{
				"  internal/server/client.go:9: client := &http.Client{Timeout: 5 * time.Second}",
				"  main.go:9: client := &http.Client{Timeout: 500 * time.Millisecond}",
			},
		},
	}

	result := Format(matches)

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

func TestFormat_NoSuggestionForSingleInstance(t *testing.T) {
	t.Parallel()
	matches := []Match{
		{
			Pattern:  "exec.Command(",
			Examples: []string{"  cmd/run.go:5: exec.Command(\"ls\")"},
		},
	}

	result := Format(matches)

	if strings.Contains(result, "SUGGESTION") {
		t.Error("should NOT suggest helper extraction for single instance")
	}
}

func TestFormat_Empty(t *testing.T) {
	t.Parallel()
	result := Format(nil)
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
