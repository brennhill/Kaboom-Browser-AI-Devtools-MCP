// blast_radius_test.go — Tests for blast radius hook.

package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBlastRadius_EditExportedFunction(t *testing.T) {
	projectRoot := setupTestProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handlers", "handlers.go") +
			`","new_string":"func HandleUser(w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(200)\n}"}`),
	}

	result := RunBlastRadius(input, projectRoot, "")
	if result == nil {
		t.Fatal("expected blast radius result for exported function edit")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "Blast Radius") {
		t.Errorf("expected 'Blast Radius' in: %s", ctx)
	}
	if !strings.Contains(ctx, "imported by") {
		t.Errorf("expected 'imported by' in: %s", ctx)
	}
}

func TestRunBlastRadius_EditUnexported(t *testing.T) {
	projectRoot := setupTestProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "db", "db.go") +
			`","new_string":"func getConnection() *sql.DB {\n\treturn nil\n}"}`),
	}

	result := RunBlastRadius(input, projectRoot, "")
	if result != nil {
		t.Errorf("expected nil result for unexported function, got: %s", result.FormatContext())
	}
}

func TestRunBlastRadius_ReadIgnored(t *testing.T) {
	projectRoot := setupTestProject(t)
	input := Input{
		ToolName:  "Read",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handlers", "handlers.go") + `"}`),
	}

	result := RunBlastRadius(input, projectRoot, "")
	if result != nil {
		t.Errorf("expected nil result for Read tool, got: %s", result.FormatContext())
	}
}

func TestRunBlastRadius_SessionAware(t *testing.T) {
	projectRoot := setupTestProject(t)
	sessionDir := t.TempDir()

	// Pre-populate session with a read of routes.go.
	routesPath := filepath.Join(projectRoot, "routes", "routes.go")
	_ = AppendTouch(sessionDir, TouchEntry{
		Tool:   "Read",
		File:   routesPath,
		Action: "read",
	})

	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handlers", "handlers.go") +
			`","new_string":"func HandleUser(w http.ResponseWriter, r *http.Request) {}"}`),
	}

	result := RunBlastRadius(input, projectRoot, sessionDir)
	if result == nil {
		t.Fatal("expected blast radius result")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "already read") {
		t.Errorf("expected 'already read' annotation for routes.go in: %s", ctx)
	}
}

func TestLooksExported_Go(t *testing.T) {
	tests := []struct {
		content  string
		exported bool
	}{
		{"func HandleUser() {}", true},
		{"type Config struct {}", true},
		{"var DefaultPort = 8080", true},
		{"func getConnection() {}", false},
		{"var count = 0", false},
	}
	for _, tt := range tests {
		got := looksExported(tt.content, "test.go")
		if got != tt.exported {
			t.Errorf("looksExported(%q, .go) = %v, want %v", tt.content, got, tt.exported)
		}
	}
}

func TestLooksExported_TS(t *testing.T) {
	tests := []struct {
		content  string
		exported bool
	}{
		{"export function getData() {}", true},
		{"export const API_URL = ''", true},
		{"export class UserService {}", true},
		{"function helper() {}", false},
		{"const x = 1", false},
	}
	for _, tt := range tests {
		got := looksExported(tt.content, "test.ts")
		if got != tt.exported {
			t.Errorf("looksExported(%q, .ts) = %v, want %v", tt.content, got, tt.exported)
		}
	}
}

func TestLooksExported_Python(t *testing.T) {
	tests := []struct {
		content  string
		exported bool
	}{
		{"def get_users():", true},
		{"class UserService:", true},
		{"def _internal():", false},
		{"class _Helper:", false},
	}
	for _, tt := range tests {
		got := looksExported(tt.content, "test.py")
		if got != tt.exported {
			t.Errorf("looksExported(%q, .py) = %v, want %v", tt.content, got, tt.exported)
		}
	}
}

func TestLooksExported_Rust(t *testing.T) {
	tests := []struct {
		content  string
		exported bool
	}{
		{"pub fn process() {}", true},
		{"pub struct Config {}", true},
		{"fn internal() {}", false},
	}
	for _, tt := range tests {
		got := looksExported(tt.content, "test.rs")
		if got != tt.exported {
			t.Errorf("looksExported(%q, .rs) = %v, want %v", tt.content, got, tt.exported)
		}
	}
}

func TestIsEditTool(t *testing.T) {
	edits := []string{"Edit", "Write", "write_file", "replace_in_file", "edit_file"}
	nonEdits := []string{"Read", "read_file", "Bash", "run_shell_command", ""}

	for _, name := range edits {
		if !isEditTool(name) {
			t.Errorf("isEditTool(%q) should be true", name)
		}
	}
	for _, name := range nonEdits {
		if isEditTool(name) {
			t.Errorf("isEditTool(%q) should be false", name)
		}
	}
}

func TestBuildImportGraph(t *testing.T) {
	projectRoot := setupTestProject(t)
	graph := buildImportGraph(projectRoot)
	if graph == nil {
		t.Fatal("expected non-nil graph")
	}

	// handlers/handlers.go imports db/db.go, so db.go should have handlers.go as an importer.
	// routes/routes.go imports handlers, so handlers/*.go should have routes.go as an importer.
	// main.go imports both handlers and routes.

	// Check that at least some importers were found.
	totalImporters := 0
	for _, importers := range graph.Importers {
		totalImporters += len(importers)
	}
	if totalImporters == 0 {
		t.Error("expected at least some import relationships in graph")
	}
}

// TestBuildImportGraphResolvesEachPathOnce pins the hook's filesystem work.
// Import resolution used to re-ask the disk for the same answer once per
// importer: every file importing a package triggered its own directory listing,
// and every relative TypeScript import re-probed the same candidate extensions.
// The cost is invisible on a warm page cache and dominates a busy one, so this
// asserts on syscalls issued rather than on elapsed time, which varies with
// machine load and would flake instead of catching the regression.
func TestBuildImportGraphResolvesEachPathOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/proj\n")

	const importers = 12
	mkdir(t, root, filepath.Join("pkg", "shared"))
	writeFile(t, root, filepath.Join("pkg", "shared", "a.go"), "package shared\n\nfunc A() {}\n")
	writeFile(t, root, filepath.Join("pkg", "shared", "b.go"), "package shared\n\nfunc B() {}\n")
	for i := 0; i < importers; i++ {
		dir := filepath.Join("cmd", fmt.Sprintf("app%d", i))
		mkdir(t, root, dir)
		writeFile(t, root, filepath.Join(dir, "main.go"),
			"package main\n\nimport (\n\t\"example.com/proj/pkg/shared\"\n)\n\nfunc main() { shared.A() }\n")
	}

	// Every TypeScript file resolves the same relative import, so the candidate
	// extension probes must be answered once for the whole build, not once per file.
	mkdir(t, root, "web")
	writeFile(t, root, filepath.Join("web", "shared.ts"), "export const value = 1;\n")
	for i := 0; i < importers; i++ {
		writeFile(t, root, filepath.Join("web", fmt.Sprintf("view%d.ts", i)),
			"import { value } from './shared';\nexport const v = value;\n")
	}

	resolver := newImportResolver(root)
	graph := buildImportGraphWith(resolver)

	// The graph itself must be unchanged by the memoization.
	for _, pkgFile := range []string{filepath.Join("pkg", "shared", "a.go"), filepath.Join("pkg", "shared", "b.go")} {
		if got := len(graph.Importers[pkgFile]); got != importers {
			t.Errorf("graph.Importers[%q] = %d importers, want %d", pkgFile, got, importers)
		}
	}
	if got := len(graph.Importers[filepath.Join("web", "shared.ts")]); got != importers {
		t.Errorf("web/shared.ts importers = %d, want %d", got, importers)
	}

	// One directory listing for the single imported package, not one per importer.
	if resolver.readDirCalls != 1 {
		t.Errorf("readDirCalls = %d, want 1 (one listing per distinct imported package)", resolver.readDirCalls)
	}
	// './shared' probes the extension-less path then '.ts', and both answers are
	// then reused; a per-file probe would cost 2 stats for each importer.
	if resolver.statCalls > 2 {
		t.Errorf("statCalls = %d, want at most 2 (one probe per distinct candidate path)", resolver.statCalls)
	}
}

func TestExtractImportsForPythonAndRust(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "shared.py", "VALUE = 1\n")
	writeFile(t, root, "pkg/helper.py", "VALUE = 2\n")
	writeFile(t, root, "pkg/app.py", "import shared\nfrom .helper import VALUE\nimport external_package\n")

	pythonImports := extractImports(filepath.Join(root, "pkg/app.py"), ".py", newImportResolver(root))
	for _, want := range []string{"shared.py", filepath.Join("pkg", "helper.py")} {
		found := false
		for _, got := range pythonImports {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("Python imports %v missing %q", pythonImports, want)
		}
	}

	if err := os.MkdirAll(filepath.Join(root, "src", "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "src/client.rs", "pub fn call() {}\n")
	writeFile(t, root, "src/model/mod.rs", "pub struct Model;\n")
	writeFile(t, root, "src/main.rs", "mod client;\nuse crate::model::Model;\nuse external::Thing;\n")
	rustImports := extractImports(filepath.Join(root, "src/main.rs"), ".rs", newImportResolver(root))
	for _, want := range []string{filepath.Join("src", "client.rs"), filepath.Join("src", "model", "mod.rs")} {
		found := false
		for _, got := range rustImports {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("Rust imports %v missing %q", rustImports, want)
		}
	}
}

// setupTestProject creates a temporary Go project with known import relationships.
func setupTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{"handlers", "routes", "db"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// go.mod
	writeFile(t, root, "go.mod", "module github.com/test/web\n\ngo 1.21\n")

	// .kaboom.json
	writeFile(t, root, ".kaboom.json", `{"code_standards":"standards.md","file_size_limit":800}`)

	// handlers/handlers.go — imports db package.
	writeFile(t, root, "handlers/handlers.go", `package handlers

import (
	"net/http"
	"github.com/test/web/db"
)

func HandleUser(w http.ResponseWriter, r *http.Request) {
	db.GetUsers()
}
`)

	// routes/routes.go — imports handlers package.
	writeFile(t, root, "routes/routes.go", `package routes

import (
	"net/http"
	"github.com/test/web/handlers"
)

func Setup() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", handlers.HandleUser)
	return mux
}
`)

	// db/db.go — no local imports.
	writeFile(t, root, "db/db.go", `package db

type User struct {
	ID   int
	Name string
}

func GetUsers() []User {
	return nil
}
`)

	// main.go — imports routes and handlers.
	writeFile(t, root, "main.go", `package main

import (
	"github.com/test/web/handlers"
	"github.com/test/web/routes"
)

func main() {
	r := routes.Setup()
	_ = r
	_ = handlers.HandleUser
}
`)

	return root
}

func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, root, relPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, relPath), 0o750); err != nil {
		t.Fatal(err)
	}
}
