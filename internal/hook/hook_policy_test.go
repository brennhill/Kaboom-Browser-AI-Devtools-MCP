// Purpose: Tests hook protocol parsing, quality policy, and decision enforcement.
// Docs: docs/features/feature/quality-gates/index.md

package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestPackageFileBoundary(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read hook package: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			count++
		}
	}
	if count > 10 {
		t.Fatalf("hook package has %d Go files; maximum is 10", count)
	}
}

func TestReadInput_ValidJSON(t *testing.T) {
	t.Parallel()
	input := `{"tool_name":"Bash","tool_input":{"command":"go test ./..."},"tool_response":"ok"}`
	in, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if in.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", in.ToolName)
	}
}

func TestReadInput_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := ReadInput(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseToolInput_FilePath(t *testing.T) {
	t.Parallel()
	in := Input{ToolInput: json.RawMessage(`{"file_path":"/tmp/foo.go"}`)}
	fields := in.ParseToolInput()
	if fields.FilePath != "/tmp/foo.go" {
		t.Errorf("FilePath = %q, want /tmp/foo.go", fields.FilePath)
	}
}

func TestParseToolInput_Command(t *testing.T) {
	t.Parallel()
	in := Input{ToolInput: json.RawMessage(`{"command":"go test ./..."}`)}
	fields := in.ParseToolInput()
	if fields.Command != "go test ./..." {
		t.Errorf("Command = %q, want 'go test ./...'", fields.Command)
	}
}

func TestResponseText_String(t *testing.T) {
	t.Parallel()
	in := Input{ToolResponse: json.RawMessage(`"hello world"`)}
	if got := in.ResponseText(); got != "hello world" {
		t.Errorf("ResponseText = %q, want 'hello world'", got)
	}
}

func TestResponseText_ObjectWithOutput(t *testing.T) {
	t.Parallel()
	in := Input{ToolResponse: json.RawMessage(`{"output":"test output"}`)}
	if got := in.ResponseText(); got != "test output" {
		t.Errorf("ResponseText = %q, want 'test output'", got)
	}
}

func TestResponseText_ObjectWithStdout(t *testing.T) {
	t.Parallel()
	in := Input{ToolResponse: json.RawMessage(`{"stdout":"stdout text"}`)}
	if got := in.ResponseText(); got != "stdout text" {
		t.Errorf("ResponseText = %q, want 'stdout text'", got)
	}
}

func TestResponseText_Empty(t *testing.T) {
	t.Parallel()
	in := Input{}
	if got := in.ResponseText(); got != "" {
		t.Errorf("ResponseText = %q, want empty", got)
	}
}

func TestWriteOutput_NonEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := WriteOutput(&buf, "some context"); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}
	var out Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.AdditionalContext != "some context" {
		t.Errorf("AdditionalContext = %q, want 'some context'", out.AdditionalContext)
	}
}

func TestWriteOutput_Empty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := WriteOutput(&buf, ""); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty context, got %q", buf.String())
	}
}

func TestDetectAgent_Default(t *testing.T) {
	// Clear any env vars that might be set.
	t.Setenv("GEMINI_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")
	if got := DetectAgent(); got != AgentClaude {
		t.Errorf("DetectAgent() = %q, want %q", got, AgentClaude)
	}
}

func TestDetectAgent_Gemini(t *testing.T) {
	t.Setenv("GEMINI_SESSION_ID", "test-session")
	if got := DetectAgent(); got != AgentGemini {
		t.Errorf("DetectAgent() = %q, want %q", got, AgentGemini)
	}
}

func TestDetectAgent_Codex(t *testing.T) {
	t.Setenv("CODEX_SESSION_ID", "test-session")
	t.Setenv("GEMINI_SESSION_ID", "")
	if got := DetectAgent(); got != AgentCodex {
		t.Errorf("DetectAgent() = %q, want %q", got, AgentCodex)
	}
}

func TestWriteOutput_GeminiFormat(t *testing.T) {
	t.Setenv("GEMINI_SESSION_ID", "test-session")

	var buf bytes.Buffer
	if err := WriteOutput(&buf, "test context"); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	hso, ok := raw["hookSpecificOutput"]
	if !ok {
		t.Fatal("expected hookSpecificOutput key for Gemini format")
	}

	var inner map[string]string
	if err := json.Unmarshal(hso, &inner); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}

	if inner["additionalContext"] != "test context" {
		t.Errorf("additionalContext = %q, want 'test context'", inner["additionalContext"])
	}
}

func TestWriteOutput_ClaudeFormat(t *testing.T) {
	t.Setenv("GEMINI_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")

	var buf bytes.Buffer
	if err := WriteOutput(&buf, "test context"); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	var out Output
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out.AdditionalContext != "test context" {
		t.Errorf("AdditionalContext = %q, want 'test context'", out.AdditionalContext)
	}
}

func makeEditInput(filePath string) Input {
	ti, _ := json.Marshal(map[string]string{"file_path": filePath})
	return Input{
		ToolName:  "Edit",
		ToolInput: ti,
	}
}

func setupProject(t *testing.T, lines int) (projectDir, filePath string) {
	t.Helper()
	dir := t.TempDir()

	// Write .kaboom.json
	cfg := `{"code_standards":"standards.md","file_size_limit":100}`
	if err := os.WriteFile(filepath.Join(dir, ".kaboom.json"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	// Write standards doc
	if err := os.WriteFile(filepath.Join(dir, "standards.md"), []byte("# Standards\n\n- Rule 1\n- Rule 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a source file with N lines
	var content strings.Builder
	for i := 0; i < lines; i++ {
		content.WriteString("line\n")
	}
	fp := filepath.Join(dir, "main.go")
	if err := os.WriteFile(fp, []byte(content.String()), 0644); err != nil {
		t.Fatal(err)
	}

	return dir, fp
}

func TestRunQualityGate_NotEditOrWrite(t *testing.T) {
	t.Parallel()
	in := Input{ToolName: "Bash"}
	if result := RunQualityGate(in); result != nil {
		t.Error("expected nil for non-Edit/Write tool")
	}
}

func TestRunQualityGate_NoFilePath(t *testing.T) {
	t.Parallel()
	in := Input{ToolName: "Edit", ToolInput: json.RawMessage(`{}`)}
	if result := RunQualityGate(in); result != nil {
		t.Error("expected nil for missing file_path")
	}
}

func TestRunQualityGate_NoKaboomConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fp := filepath.Join(dir, "foo.go")
	os.WriteFile(fp, []byte("package main\n"), 0644)

	in := makeEditInput(fp)
	if result := RunQualityGate(in); result != nil {
		t.Error("expected nil when no .kaboom.json exists")
	}
}

func TestRunQualityGate_InjectsStandards(t *testing.T) {
	t.Parallel()
	_, fp := setupProject(t, 50) // under limit

	in := makeEditInput(fp)
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected quality gate result")
	}
	if !strings.Contains(result.Context, "PROJECT CODE STANDARDS") {
		t.Error("missing standards header")
	}
	if !strings.Contains(result.Context, "Rule 1") {
		t.Error("missing standards content")
	}
	if !strings.Contains(result.Context, "QUALITY GATE") {
		t.Error("missing review instruction")
	}
}

func TestRunQualityGate_FileSizeWarning(t *testing.T) {
	t.Parallel()
	_, fp := setupProject(t, 120) // over limit of 100

	in := makeEditInput(fp)
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected quality gate result")
	}
	if !strings.Contains(result.Context, "WARNING:") {
		t.Error("missing file size warning")
	}
	if !strings.Contains(result.Context, "must be split") {
		t.Error("missing split instruction")
	}
}

func TestRunQualityGate_FileSizeNote(t *testing.T) {
	t.Parallel()
	_, fp := setupProject(t, 95) // 95% of 100 limit

	in := makeEditInput(fp)
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected quality gate result")
	}
	if !strings.Contains(result.Context, "NOTE:") {
		t.Error("missing approaching-limit note")
	}
}

func TestRunQualityGate_WriteToolAlsoWorks(t *testing.T) {
	t.Parallel()
	_, fp := setupProject(t, 50)

	ti, _ := json.Marshal(map[string]string{"file_path": fp})
	in := Input{ToolName: "Write", ToolInput: ti}
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected quality gate result for Write tool")
	}
}

func TestRunQualityGate_DefaultConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Minimal .kaboom.json with no code_standards field.
	os.WriteFile(filepath.Join(dir, ".kaboom.json"), []byte(`{}`), 0644)
	// Write the default standards file.
	os.WriteFile(filepath.Join(dir, "kaboom-code-standards.md"), []byte("# Default\n"), 0644)

	fp := filepath.Join(dir, "main.go")
	os.WriteFile(fp, []byte("package main\n"), 0644)

	in := makeEditInput(fp)
	result := RunQualityGate(in)
	if result == nil {
		t.Fatal("expected result with default config")
	}
	if !strings.Contains(result.Context, "Default") {
		t.Error("should load default standards file")
	}
}

func TestFindProjectRoot_Nested(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".kaboom.json"), []byte(`{}`), 0644)
	nested := filepath.Join(dir, "src", "pkg")
	os.MkdirAll(nested, 0755)
	fp := filepath.Join(nested, "foo.go")
	os.WriteFile(fp, []byte("package pkg\n"), 0644)

	root := FindProjectRoot(fp)
	if root != dir {
		t.Errorf("FindProjectRoot = %q, want %q", root, dir)
	}
}

func TestCountLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 0},
		{"one_line_no_newline", "hello", 1},
		{"one_line_with_newline", "hello\n", 1},
		{"three_lines", "a\nb\nc\n", 3},
	}
	for _, tt := range tests {
		fp := filepath.Join(dir, tt.name)
		os.WriteFile(fp, []byte(tt.content), 0644)
		got, err := countLines(fp)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("%s: countLines = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestRunDecisionGuard_PatternMatch(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") +
			`","new_string":"client := &http.Client{Timeout: 5 * time.Second}"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result == nil {
		t.Fatal("expected decision guard result for http.Client{}")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "DECISION-001") {
		t.Errorf("expected DECISION-001 in: %s", ctx)
	}
	if !strings.Contains(ctx, "shared HTTP client") {
		t.Errorf("expected 'shared HTTP client' in: %s", ctx)
	}
	if !strings.Contains(ctx, "DECISION GUARD") {
		t.Errorf("expected 'DECISION GUARD' in: %s", ctx)
	}
}

func TestRunDecisionGuard_RegexMatch(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") +
			`","new_string":"import \"database/sql\"\n\nfunc q() { db, _ := sql.Open(\"pg\", \"\") }"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result == nil {
		t.Fatal("expected decision guard result for database/sql")
	}
	ctx := result.FormatContext()
	if !strings.Contains(ctx, "DECISION-002") {
		t.Errorf("expected DECISION-002 in: %s", ctx)
	}
}

func TestRunDecisionGuard_NoMatch(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") +
			`","new_string":"func HandleHealth(w http.ResponseWriter, r *http.Request) {\n\tw.WriteHeader(200)\n}"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result != nil {
		t.Errorf("expected nil result for non-violating edit, got: %s", result.FormatContext())
	}
}

func TestRunDecisionGuard_ExpiredDecision(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	// The expired decision (DECISION-003) matches "this-should-never-match-anything-real"
	// which won't match normal code, but let's also test that expired decisions are skipped.
	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") +
			`","new_string":"this-should-never-match-anything-real"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result != nil {
		t.Errorf("expected nil result for expired decision, got: %s", result.FormatContext())
	}
}

func TestRunDecisionGuard_ReadIgnored(t *testing.T) {
	projectRoot := setupDecisionProject(t)
	input := Input{
		ToolName:  "Read",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(projectRoot, "handler.go") + `"}`),
	}

	result := RunDecisionGuard(input, projectRoot)
	if result != nil {
		t.Errorf("expected nil result for Read tool, got: %s", result.FormatContext())
	}
}

func TestRunDecisionGuard_NoDecisionsFile(t *testing.T) {
	root := t.TempDir()
	// Project without decisions.json.
	writeFile(t, root, ".kaboom.json", `{}`)
	writeFile(t, root, "handler.go", "package main\n")

	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(root, "handler.go") +
			`","new_string":"client := &http.Client{}"}`),
	}

	result := RunDecisionGuard(input, root)
	if result != nil {
		t.Errorf("expected nil result when no decisions.json exists, got: %s", result.FormatContext())
	}
}

func TestRunDecisionGuard_InlineRegex(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".kaboom"), 0o755)
	writeFile(t, root, ".kaboom.json", `{}`)
	writeFile(t, root, ".kaboom/decisions.json", `[
		{"id":"INLINE-RE","rule":"No fmt.Println","pattern":"re:fmt\\.Println\\(","reason":"Use structured logging"}
	]`)
	writeFile(t, root, "main.go", "package main\n")

	input := Input{
		ToolName: "Edit",
		ToolInput: json.RawMessage(`{"file_path":"` + filepath.Join(root, "main.go") +
			`","new_string":"fmt.Println(\"debug\")"}`),
	}

	result := RunDecisionGuard(input, root)
	if result == nil {
		t.Fatal("expected match for inline regex pattern")
	}
	if !strings.Contains(result.FormatContext(), "INLINE-RE") {
		t.Errorf("expected INLINE-RE in output: %s", result.FormatContext())
	}
}

func TestMatchesDecision_InvalidRegex(t *testing.T) {
	d := Decision{Regex: "[invalid"}
	if matchesDecision(d, "anything", "") {
		t.Error("invalid regex should not match")
	}
}

func TestIsExpired(t *testing.T) {
	tests := []struct {
		expires string
		expired bool
	}{
		{"", false},
		{"2099-12-31", false},
		{"2020-01-01", true},
		{"invalid-date", false},
	}
	for _, tt := range tests {
		d := Decision{Expires: tt.expires}
		if got := isExpired(d); got != tt.expired {
			t.Errorf("isExpired(expires=%q) = %v, want %v", tt.expires, got, tt.expired)
		}
	}
}

func setupDecisionProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".kaboom"), 0o755)

	writeFile(t, root, ".kaboom.json", `{"code_standards":"standards.md","file_size_limit":800}`)
	writeFile(t, root, "handler.go", "package main\n")

	writeFile(t, root, ".kaboom/decisions.json", `[
		{
			"id": "DECISION-001",
			"rule": "Use shared HTTP client from pkg/httpclient",
			"pattern": "http.Client{",
			"reason": "Shared client has timeouts and retries.",
			"enforced": "2026-01-15"
		},
		{
			"id": "DECISION-002",
			"rule": "All database queries must go through the db package",
			"regex": "database/sql|sql\\.Open",
			"reason": "Centralized connection pooling.",
			"enforced": "2026-02-01"
		},
		{
			"id": "DECISION-003",
			"rule": "Expired decision",
			"pattern": "this-should-never-match-anything-real",
			"enforced": "2025-01-01",
			"expires": "2025-06-01"
		}
	]`)

	return root
}

// TestMain gives the package its own state root. Session tracking and blast-radius
// caching persist under the Kaboom state directory, and without this every test run
// would deposit files in the developer's real ~/.kaboom and read back results a
// previous run had written.
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

// TestRunQualityGate_InjectsProjectConventions is the seam between the gate and the
// convention engine. The engine has its own tests; what this one holds is that the
// gate actually calls it and puts the answer in the context the agent reads — the
// two have lived in separate packages since the engine moved, and a dropped call
// would leave every review with no conventions and no error.
func TestRunQualityGate_InjectsProjectConventions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Two existing files using http.Client{ — enough for the engine to call it an
	// established pattern rather than a one-off.
	for i, name := range []string{"client.go", "probe.go"} {
		writeFile(t, dir, name, fmt.Sprintf(
			"package main\n\nimport \"net/http\"\n\nvar c%d = &http.Client{Timeout: 1e9}\n", i))
	}
	writeFile(t, dir, ".kaboom.json", `{"code_standards":"standards.md","file_size_limit":800}`)
	writeFile(t, dir, "standards.md", "# Standards\n")

	editedFile := filepath.Join(dir, "new_service.go")
	writeFile(t, dir, "new_service.go", "package main\n")

	input := Input{
		ToolName: "Edit",
		ToolInput: mustMarshal(map[string]string{
			"file_path":  editedFile,
			"new_string": `client := &http.Client{Timeout: 1e9}`,
		}),
	}

	result := RunQualityGate(input)
	if result == nil {
		t.Fatal("the quality gate returned nothing for an edit inside a configured project")
	}
	if !strings.Contains(result.Context, "CODEBASE CONVENTIONS") {
		t.Fatalf("the gate injected no conventions; a reviewer sees no existing pattern to match:\n%s", result.Context)
	}
	if !strings.Contains(result.Context, "http.Client{") {
		t.Errorf("the established pattern is missing from the injected context:\n%s", result.Context)
	}
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
