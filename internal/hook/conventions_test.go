// Purpose: Tests project convention discovery, caching, detection, and formatting.
// Docs: docs/features/feature/convention-engine/index.md

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

func TestDiscoverConventions_GoProject(t *testing.T) {
	t.Parallel()

	// Find the repo root (the real kaboom codebase).
	root := findRepoRoot(t)

	conventions := DiscoverConventions(root, ".go")
	if len(conventions) == 0 {
		t.Fatal("expected discovered conventions for Go codebase, got none")
	}

	t.Logf("discovered %d Go conventions:", len(conventions))
	for _, c := range conventions {
		t.Logf("  %3d files  %s", c.FileCount, c.Pattern)
	}

	// Sanity: should find patterns we know exist in kaboom.
	found := make(map[string]bool)
	for _, c := range conventions {
		found[c.Pattern] = true
	}

	// These are real kaboom patterns that appear in many files.
	wantSome := []string{
		"json.Unmarshal(",
		"json.Marshal(",
	}
	for _, w := range wantSome {
		if !found[w] {
			t.Errorf("expected to discover %q — it's a real pattern in this codebase", w)
		}
	}

	// Noise should be filtered out.
	noisePatterns := []string{
		"t.Fatalf(",
		"t.Errorf(",
		"strings.Contains(",
		"fmt.Sprintf(",
		"filepath.Join(",
		"mu.Lock(",
	}
	for _, n := range noisePatterns {
		if found[n] {
			t.Errorf("noise pattern %q should be filtered, but was discovered", n)
		}
	}
}

func TestDiscoverConventions_TSProject(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)

	conventions := DiscoverConventions(root, ".ts")
	if len(conventions) == 0 {
		t.Fatal("expected discovered conventions for TS files, got none")
	}

	t.Logf("discovered %d TS conventions:", len(conventions))
	for _, c := range conventions {
		t.Logf("  %3d files  %s", c.FileCount, c.Pattern)
	}

	// Noise should be filtered.
	found := make(map[string]bool)
	for _, c := range conventions {
		found[c.Pattern] = true
	}
	tsNoise := []string{"Date.now(", "Math.min(", "JSON.stringify(", "console.log("}
	for _, n := range tsNoise {
		if found[n] {
			t.Errorf("noise pattern %q should be filtered, but was discovered", n)
		}
	}
}

func TestDiscoverConventions_Cache(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)

	// First call populates cache.
	c1 := DiscoverConventions(root, ".go")

	// Second call should return cached result (same slice).
	c2 := DiscoverConventions(root, ".go")

	if len(c1) != len(c2) {
		t.Fatalf("cache miss: first call returned %d, second returned %d", len(c1), len(c2))
	}

	for i := range c1 {
		if c1[i].Pattern != c2[i].Pattern {
			t.Errorf("cache miss at index %d: %q vs %q", i, c1[i].Pattern, c2[i].Pattern)
		}
	}
}

func TestDiscoverConventions_EmptyDir(t *testing.T) {
	t.Parallel()

	empty := t.TempDir()
	conventions := DiscoverConventions(empty, ".go")
	if len(conventions) != 0 {
		t.Errorf("expected no conventions in empty dir, got %d", len(conventions))
	}
}

func TestDiscoverConventions_SmallProject(t *testing.T) {
	t.Parallel()

	// Build a minimal project where `db.Query(` appears in 3 files.
	root := t.TempDir()
	files := map[string]string{
		"a.go": "package main\nfunc a() { db.Query(\"SELECT 1\") }\n",
		"b.go": "package main\nfunc b() { db.Query(\"SELECT 2\") }\n",
		"c.go": "package main\nfunc c() { db.Query(\"SELECT 3\") }\n",
		"d.go": "package main\nfunc d() { db.Query(\"SELECT 4\"); db.Exec(\"INSERT\") }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Clear cache so we don't get stale results.
	discoveryCache.mu.Lock()
	delete(discoveryCache.entries, root+"\x00.go")
	discoveryCache.mu.Unlock()

	conventions := DiscoverConventions(root, ".go")

	found := make(map[string]bool)
	for _, c := range conventions {
		found[c.Pattern] = true
		t.Logf("  %d files  %s", c.FileCount, c.Pattern)
	}

	if !found["db.Query("] {
		t.Error("expected to discover db.Query( in 4 files")
	}
	// db.Exec( only appears in 1 file — below threshold.
	if found["db.Exec("] {
		t.Error("db.Exec( appears in 1 file — should be below threshold")
	}
}

func TestDiscoveredProbes_ReturnStrings(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)
	probes := DiscoveredProbes(root, ".go")
	if len(probes) == 0 {
		t.Fatal("expected probes, got none")
	}

	// Every probe should end with (.
	for _, p := range probes {
		if !strings.HasSuffix(p, "(") {
			t.Errorf("probe %q should end with (", p)
		}
	}
}

func TestNoiseFiltering_Comprehensive(t *testing.T) {
	t.Parallel()

	// Verify all noise entries match the regex they're supposed to filter.
	for pattern := range goNoise {
		if !goCallSite.MatchString(pattern) {
			t.Errorf("Go noise pattern %q doesn't match goCallSite regex", pattern)
		}
	}
	for pattern := range tsNoise {
		if !tsCallSite.MatchString(pattern) {
			t.Errorf("TS noise pattern %q doesn't match tsCallSite regex", pattern)
		}
	}
}

// findRepoRoot walks up from the test file to find go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}
