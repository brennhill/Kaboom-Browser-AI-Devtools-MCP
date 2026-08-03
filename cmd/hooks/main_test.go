// main_test.go — Tests for the kaboom-hooks binary CLI dispatch.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hooksBinary is built once in TestMain and shared across all tests.
var hooksBinary string

func TestMain(m *testing.M) {
	// Build the binary once for all tests.
	dir, err := os.MkdirTemp("", "kaboom-hooks-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	binPath := filepath.Join(dir, "kaboom-hooks")
	repoRoot := findRepoRoot()
	buildArgs := []string{"build", "-o", binPath, "."}
	if os.Getenv("KABOOM_GO_COVERDIR") != "" {
		buildArgs = []string{"build", "-cover", "-coverpkg=./...", "-o", binPath, "."}
	}
	cmd := exec.Command("go", buildArgs...)
	cmd.Dir = filepath.Join(repoRoot, "cmd", "hooks")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build kaboom-hooks: %v\n%s\n", err, out)
		os.Exit(1)
	}

	hooksBinary = binPath
	os.Exit(m.Run())
}

func hooksCommand(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(hooksBinary, args...)
	cmd.Env = os.Environ()
	if root := os.Getenv("KABOOM_GO_COVERDIR"); root != "" {
		dir, err := os.MkdirTemp(root, "hooks-")
		if err != nil {
			t.Fatalf("create hook coverage directory: %v", err)
		}
		cmd.Env = append(os.Environ(), "GOCOVERDIR="+dir)
	}
	return cmd
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find repo root")
		}
		dir = parent
	}
}

func TestCLI_NoArgs(t *testing.T) {
	t.Parallel()
	cmd := hooksCommand(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit with no args")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 2 {
			t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
		}
	}
	if !strings.Contains(string(out), "Usage:") {
		t.Error("expected usage output on no args")
	}
}

func TestCLI_Version(t *testing.T) {
	t.Parallel()
	out, err := hooksCommand(t, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		t.Fatal("--version produced empty output")
	}
	// Should look like a semver.
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		t.Fatalf("--version output doesn't look like semver: %q", trimmed)
	}
}

func TestCLI_Help(t *testing.T) {
	t.Parallel()
	out, err := hooksCommand(t, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
	s := string(out)
	if !strings.Contains(s, "quality-gate") {
		t.Error("--help missing quality-gate command")
	}
	if !strings.Contains(s, "compress-output") {
		t.Error("--help missing compress-output command")
	}
}

func TestCLI_UnknownCommand(t *testing.T) {
	t.Parallel()
	cmd := hooksCommand(t, "nonexistent")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(string(out), "Unknown command") {
		t.Error("expected 'Unknown command' in output")
	}
}

func TestCLI_QualityGate_EmptyStdin(t *testing.T) {
	t.Parallel()
	cmd := hooksCommand(t, "quality-gate")
	cmd.Stdin = strings.NewReader("")
	if err := cmd.Run(); err != nil {
		t.Fatalf("quality-gate with empty stdin should exit 0, got: %v", err)
	}
}

func TestCLI_CompressOutput_EmptyStdin(t *testing.T) {
	t.Parallel()
	cmd := hooksCommand(t, "compress-output")
	cmd.Stdin = strings.NewReader("")
	if err := cmd.Run(); err != nil {
		t.Fatalf("compress-output with empty stdin should exit 0, got: %v", err)
	}
}

func TestCLI_SessionTrackSurfacesPersistenceFailure(t *testing.T) {
	root := t.TempDir()
	blockedHome := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockedHome, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(map[string]any{
		"tool_name":  "Read",
		"tool_input": map[string]string{"file_path": "/private/file.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	cmd := hooksCommand(t, "session-track")
	cmd.Env = append(os.Environ(), "HOME="+blockedHome, "GEMINI_SESSION_ID=persistence-failure")
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("session-track failed: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("session-track output is not protocol JSON: %v\n%s", err, out)
	}
	context, _ := result["hookSpecificOutput"].(map[string]any)["additionalContext"].(string)
	if !strings.Contains(context, "session_directory_create_failed") || strings.Contains(context, blockedHome) {
		t.Fatalf("session-track failure context = %q", context)
	}
}

func TestCLI_QualityGate_ValidInput(t *testing.T) {
	t.Parallel()

	// Create a project with .kaboom.json and standards.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".kaboom.json"), []byte(`{"code_standards":"standards.md","file_size_limit":800}`), 0644)
	os.WriteFile(filepath.Join(dir, "standards.md"), []byte("# Standards\nNo dead code.\n"), 0644)

	// Create the file being edited.
	editedFile := filepath.Join(dir, "foo.go")
	os.WriteFile(editedFile, []byte("package main\n\nfunc main() {}\n"), 0644)

	// Build hook input JSON (Claude Code hook protocol uses snake_case).
	input := map[string]any{
		"tool_name": "Edit",
		"tool_input": map[string]string{
			"file_path":  editedFile,
			"new_string": "func hello() {}",
		},
	}
	inputJSON, _ := json.Marshal(input)

	cmd := hooksCommand(t, "quality-gate")
	cmd.Stdin = bytes.NewReader(inputJSON)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("quality-gate failed: %v\n%s", err, out)
	}

	// Should output additionalContext JSON containing the standards.
	if len(out) == 0 {
		t.Fatal("expected non-empty output with standards context")
	}
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	ctx, _ := result["additionalContext"].(string)
	if !strings.Contains(ctx, "STANDARDS") {
		t.Errorf("expected STANDARDS in additionalContext, got: %s", ctx)
	}
}

func TestCLI_CompressOutput_GoTestOutput(t *testing.T) {
	t.Parallel()

	// Build a go test-like output (needs 50+ lines to trigger compression).
	var lines []string
	for i := 0; i < 60; i++ {
		lines = append(lines, fmt.Sprintf("=== RUN   TestFunc%d", i))
		lines = append(lines, fmt.Sprintf("--- PASS: TestFunc%d (0.01s)", i))
	}
	lines = append(lines, "PASS")
	lines = append(lines, "ok  \tgithub.com/example/pkg\t0.05s")
	goTestOutput := strings.Join(lines, "\n")

	input := map[string]any{
		"tool_name": "Bash",
		"tool_input": map[string]string{
			"command": "go test ./...",
		},
		"tool_response": map[string]string{
			"stdout": goTestOutput,
		},
	}
	inputJSON, _ := json.Marshal(input)

	cmd := hooksCommand(t, "compress-output")
	cmd.Stdin = bytes.NewReader(inputJSON)
	// Set port 0 so token savings POST silently fails (no daemon).
	cmd.Env = append(os.Environ(), "KABOOM_PORT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compress-output failed: %v\n%s", err, out)
	}

	if len(out) == 0 {
		t.Fatal("expected compressed output")
	}
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
}

func TestCLI_StatefulHookCommands(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.test/app\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(project, "service.go")
	if err := os.WriteFile(edited, []byte("package app\n\nfunc Exported() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(map[string]any{
		"tool_name": "Edit",
		"tool_input": map[string]string{
			"file_path":  edited,
			"new_string": "func Exported() { println(\"changed\") }",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"session-track", "blast-radius", "decision-guard"} {
		t.Run(command, func(t *testing.T) {
			cmd := hooksCommand(t, command)
			cmd.Stdin = bytes.NewReader(input)
			cmd.Env = append(cmd.Env,
				"HOME="+t.TempDir(),
				"KABOOM_STATE_DIR="+t.TempDir(),
			)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s failed: %v\n%s", command, err, output)
			}
			if len(output) > 0 {
				var result map[string]any
				if err := json.Unmarshal(output, &result); err != nil {
					t.Fatalf("%s output is not hook JSON: %v\n%s", command, err, output)
				}
			}
		})
	}
}
