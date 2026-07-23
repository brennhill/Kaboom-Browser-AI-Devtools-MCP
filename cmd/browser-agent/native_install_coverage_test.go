// Purpose: Coverage tests for the --install flow in native_install.go — installer
// output panels, the `claude mcp add-json` shell-out, background daemon launch,
// and the mergeJSONConfig error/custom-shape paths not exercised elsewhere.
// Docs: docs/features/feature/mcp-persistent-server/index.md
//
// WARNING: tests here swap the package-global stderr sink and use t.Setenv, so
// none of them may call t.Parallel().

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsync"
)

// lifecovSyncBuffer is a mutex-guarded stderr sink. stderrf can be reached from
// background goroutines, so a bare bytes.Buffer would be a data race under -race.
type lifecovSyncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lifecovSyncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lifecovSyncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// lifecovCaptureStderr redirects stderrf output for the duration of the test.
// Must not be used from a parallel test: stderrSink is package state.
func lifecovCaptureStderr(t *testing.T) *lifecovSyncBuffer {
	t.Helper()
	buf := &lifecovSyncBuffer{}
	old := stderrSink
	setStderrSink(buf)
	t.Cleanup(func() { stderrSink = old })
	return buf
}

// lifecovFakeClaudeCLI writes an executable `claude` shim into a fresh dir and
// points PATH at that dir alone, so exec.LookPath("claude") is fully controlled
// (the developer's real `claude` must never be invoked by a test).
// The shim appends its argv, stdin and CLAUDECODE value to recordPath.
func lifecovFakeClaudeCLI(t *testing.T, script string) (binDir, recordPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script shim is POSIX-only")
	}
	binDir = t.TempDir()
	recordPath = filepath.Join(binDir, "record.txt")
	// stdin is read with the `read` builtin, not `$(cat)`: PATH is pinned to
	// binDir alone (below), so external tools like `cat` are not resolvable and a
	// `$(cat)` would silently record an empty stdin. The install payload is a
	// single JSON line with no trailing newline, which `read` still captures.
	shim := "#!/bin/sh\n" +
		"RECORD=" + recordPath + "\n" +
		"{ echo \"argv: $*\"; echo \"claudecode: [${CLAUDECODE}]\"; IFS= read -r STDIN_LINE; echo \"stdin: $STDIN_LINE\"; } >> \"$RECORD\"\n" +
		script
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte(shim), 0o700); err != nil {
		t.Fatalf("write claude shim: %v", err)
	}
	t.Setenv("PATH", binDir)
	return binDir, recordPath
}

// ============================================
// Installer output
// ============================================

func TestInstallerPanel_RendersTitleAndEveryLineInsideTheBorder(t *testing.T) {
	buf := lifecovCaptureStderr(t)

	printInstallerPanel("INSTALL SUMMARY", []string{"line one", "line two"})

	out := buf.String()
	for _, want := range []string{"INSTALL SUMMARY", "line one", "line two"} {
		if !strings.Contains(out, want) {
			t.Errorf("panel output missing %q:\n%s", want, out)
		}
	}
	// Three borders: top, under-title, bottom. A dropped border makes the panel
	// visually merge with the surrounding install log.
	if got := strings.Count(out, "+----------------------------------------------------------+"); got != 3 {
		t.Errorf("border count = %d, want 3 (top, under-title, bottom)", got)
	}
}

func TestInstallerPanel_EmptyLinesStillRendersTheFrame(t *testing.T) {
	buf := lifecovCaptureStderr(t)

	printInstallerPanel("EMPTY", nil)

	if got := strings.Count(buf.String(), "+---"); got != 3 {
		t.Errorf("border count = %d, want 3 even with no body lines", got)
	}
}

func TestManualExtensionSetupChecklist_PrintsEveryStepAndTheStagedDir(t *testing.T) {
	buf := lifecovCaptureStderr(t)
	extDir := filepath.Join(t.TempDir(), "KaboomAgenticDevtoolExtension")

	printManualExtensionSetupChecklist(extDir)

	out := buf.String()
	// The staged path is the one thing the user must copy into the browser; if
	// it stops being printed the checklist is useless.
	if !strings.Contains(out, extDir) {
		t.Errorf("checklist did not print the staged extension dir %q:\n%s", extDir, out)
	}
	lines := manualExtensionSetupChecklist(extDir)
	for _, line := range lines {
		if !strings.Contains(out, strings.TrimSpace(line)) {
			t.Errorf("checklist line not printed: %q", line)
		}
	}
	if got := strings.Count(out, "\n"); got != len(lines) {
		t.Errorf("printed %d lines, want %d (one per checklist entry)", got, len(lines))
	}
}

// ============================================
// installClaudeCode
// ============================================

func TestInstallClaudeCode_SkipsSilentlyWhenClaudeIsNotOnPath(t *testing.T) {
	// PATH points at an empty dir, so LookPath must fail. Nothing may be
	// written and no error may be reported: a missing client is not an install
	// failure.
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	if err := installClaudeCode("/opt/kaboom/kaboom-agentic-browser"); err != nil {
		t.Fatalf("installClaudeCode() with no claude on PATH = %v, want nil", err)
	}
	entries, readErr := os.ReadDir(empty)
	if readErr != nil {
		t.Fatalf("ReadDir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Errorf("installClaudeCode touched the filesystem when claude was absent: %v", entries)
	}
}

func TestInstallClaudeCode_PassesUserScopeAndBinaryPathOnStdin(t *testing.T) {
	_, record := lifecovFakeClaudeCLI(t, "exit 0\n")
	const exePath = "/opt/kaboom/kaboom-agentic-browser"

	if err := installClaudeCode(exePath); err != nil {
		t.Fatalf("installClaudeCode() = %v, want nil", err)
	}

	recorded, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("claude shim was never invoked: %v", err)
	}
	got := string(recorded)
	// `--scope user` is what makes the registration survive across projects;
	// dropping it silently downgrades the install to the cwd.
	if !strings.Contains(got, "argv: mcp add-json --scope user "+mcpServerName) {
		t.Errorf("claude argv = %q, want `mcp add-json --scope user %s`", got, mcpServerName)
	}

	stdinLine := ""
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "stdin: ") {
			stdinLine = strings.TrimPrefix(line, "stdin: ")
		}
	}
	var entry struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.Unmarshal([]byte(stdinLine), &entry); err != nil {
		t.Fatalf("stdin payload is not JSON (%v): %q", err, stdinLine)
	}
	if entry.Command != exePath {
		t.Errorf("stdin command = %q, want the resolved binary path %q", entry.Command, exePath)
	}
	if entry.Args == nil {
		t.Error("stdin args must be an empty array, not null: claude rejects a null args field")
	}
}

func TestInstallClaudeCode_BlanksCLAUDECODESoItRunsInsideAClaudeSession(t *testing.T) {
	_, record := lifecovFakeClaudeCLI(t, "exit 0\n")
	// The installer may itself be launched from inside a Claude Code session.
	t.Setenv("CLAUDECODE", "1")

	if err := installClaudeCode("/opt/kaboom/bin"); err != nil {
		t.Fatalf("installClaudeCode() = %v", err)
	}

	recorded, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("claude shim was never invoked: %v", err)
	}
	// cmd.Env appends "CLAUDECODE=" to neutralise the inherited value. If that
	// append is lost, `claude mcp add-json` refuses to run nested.
	if !strings.Contains(string(recorded), "claudecode: []") {
		t.Errorf("child saw CLAUDECODE inherited; want it blanked:\n%s", recorded)
	}
}

func TestInstallClaudeCode_SurfacesCLIOutputWhenAddJSONFails(t *testing.T) {
	lifecovFakeClaudeCLI(t, "echo 'server already exists' >&2\nexit 3\n")

	err := installClaudeCode("/opt/kaboom/bin")
	if err == nil {
		t.Fatal("installClaudeCode() = nil, want an error when `claude mcp add-json` exits non-zero")
	}
	msg := err.Error()
	if !strings.Contains(msg, "claude mcp add-json failed") {
		t.Errorf("error = %q, want it to name the failing command", msg)
	}
	// The CLI's own stderr is the only actionable detail; losing it leaves the
	// user with a bare exit status.
	if !strings.Contains(msg, "server already exists") {
		t.Errorf("error = %q, want it to include the CLI output", msg)
	}
}

// ============================================
// startDaemonSilently
// ============================================

func TestStartDaemonSilently_LaunchesTheBinaryWithDaemonFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script shim is POSIX-only")
	}
	buf := lifecovCaptureStderr(t)
	dir := t.TempDir()
	record := filepath.Join(dir, "argv.txt")
	fake := filepath.Join(dir, "fake-kaboom")
	// The shim exits immediately; it never opens a port, so nothing here can
	// collide with a real daemon on 7890.
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"$*\" > "+record+"\n"), 0o700); err != nil {
		t.Fatalf("write shim: %v", err)
	}

	startDaemonSilently(fake)

	testsync.Eventually(t, testsync.DefaultTimeout, "the launched daemon shim to record its argv", func() bool {
		_, err := os.Stat(record)
		return err == nil
	})
	argv, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	// Port 7890 is the contract the installer's success panel advertises.
	if got := strings.TrimSpace(string(argv)); got != "--daemon --port 7890" {
		t.Errorf("daemon argv = %q, want %q", got, "--daemon --port 7890")
	}
	if !strings.Contains(buf.String(), "✅") {
		t.Errorf("expected a success marker on stderr, got %q", buf.String())
	}
}

func TestStartDaemonSilently_ReportsLaunchFailureInsteadOfPanicking(t *testing.T) {
	buf := lifecovCaptureStderr(t)
	missing := filepath.Join(t.TempDir(), "no-such-binary")

	startDaemonSilently(missing)

	out := buf.String()
	if !strings.Contains(out, "could not start background server") {
		t.Errorf("stderr = %q, want a launch-failure warning", out)
	}
	// A failed launch must not print the success marker: the install summary
	// that follows claims the server is running on 7890.
	if strings.Contains(out, "✅") {
		t.Errorf("failed launch printed the success marker: %q", out)
	}
}

// ============================================
// mergeJSONConfig — error and custom-client shapes
// ============================================

func TestMergeJSONConfig_RejectsNonObjectValueAtTheServersKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers": "not-an-object"}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := mergeJSONConfig(path, "mcpServers", "/opt/kaboom/bin", false)
	if err == nil {
		t.Fatal("mergeJSONConfig() = nil, want an error when the key holds a scalar")
	}
	if !strings.Contains(err.Error(), "unexpected format for key") {
		t.Errorf("error = %q, want `unexpected format for key`", err)
	}
	// The original file must survive: overwriting an unrecognised shape would
	// destroy a hand-edited client config.
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	if !strings.Contains(string(after), "not-an-object") {
		t.Errorf("config was modified despite the error: %s", after)
	}
}

func TestMergeJSONConfig_ReportsWriteFailureWhenParentDirIsMissing(t *testing.T) {
	// No mkdir: os.WriteFile must fail rather than silently reporting success.
	path := filepath.Join(t.TempDir(), "no-such-dir", "mcp.json")

	err := mergeJSONConfig(path, "mcpServers", "/opt/kaboom/bin", false)
	if err == nil {
		t.Fatal("mergeJSONConfig() = nil, want an error writing into a missing directory")
	}
	if !strings.Contains(err.Error(), "no-such-dir") {
		t.Errorf("error = %q, want it to name the unwritable path", err)
	}
}

func TestMergeJSONConfig_OpenCodeShapeUsesLocalTypeAndCommandArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")

	if err := mergeJSONConfig(path, "mcp", "/opt/kaboom/bin", true); err != nil {
		t.Fatalf("mergeJSONConfig() = %v", err)
	}

	entry := lifecovReadServerEntry(t, path, "mcp")
	if got, _ := entry["type"].(string); got != "local" {
		t.Errorf("type = %q, want \"local\" (OpenCode rejects other transports for a binary)", got)
	}
	if got, _ := entry["enabled"].(bool); !got {
		t.Errorf("enabled = %v, want true", entry["enabled"])
	}
	// OpenCode takes command as an argv array, not a string — the shape
	// difference from every other client is the whole point of isCustom.
	cmdArr, ok := entry["command"].([]any)
	if !ok {
		t.Fatalf("command = %#v, want a JSON array", entry["command"])
	}
	if len(cmdArr) != 1 || cmdArr[0] != "/opt/kaboom/bin" {
		t.Errorf("command = %#v, want [\"/opt/kaboom/bin\"]", cmdArr)
	}
}

func TestMergeJSONConfig_ZedShapeUsesCustomSourceAndStringCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zed.json")

	if err := mergeJSONConfig(path, "context_servers", "/opt/kaboom/bin", true); err != nil {
		t.Fatalf("mergeJSONConfig() = %v", err)
	}

	entry := lifecovReadServerEntry(t, path, "context_servers")
	if got, _ := entry["source"].(string); got != "custom" {
		t.Errorf("source = %q, want \"custom\" (Zed ignores entries without it)", got)
	}
	if got, _ := entry["command"].(string); got != "/opt/kaboom/bin" {
		t.Errorf("command = %#v, want the path as a plain string", entry["command"])
	}
	if _, ok := entry["args"].([]any); !ok {
		t.Errorf("args = %#v, want an array", entry["args"])
	}
}

// Documents current behavior, not desired behavior: isCustom=true with a key
// other than "mcp" or "context_servers" writes the config back with the server
// key present but EMPTY — no entry is added and no error is returned, so the
// installer counts the client as configured. See the final report.
func TestMergeJSONConfig_CustomWithUnknownKeyWritesNoServerEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unknown.json")

	if err := mergeJSONConfig(path, "servers", "/opt/kaboom/bin", true); err != nil {
		t.Fatalf("mergeJSONConfig() = %v, want nil (current behavior)", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	servers, ok := parsed["servers"].(map[string]any)
	if !ok {
		t.Fatalf("servers key = %#v, want an object", parsed["servers"])
	}
	if _, present := servers[mcpServerName]; present {
		t.Fatalf("BEHAVIOR CHANGED: %q is now registered for an unknown custom key; "+
			"update this test and the report note", mcpServerName)
	}
}

func TestMergeJSONConfig_BackupIsSkippedWhenThereIsNothingToBackUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.json")

	if err := mergeJSONConfig(path, "mcpServers", "/opt/kaboom/bin", false); err != nil {
		t.Fatalf("mergeJSONConfig() = %v", err)
	}

	// A .bak of a file that never existed would be pure noise in the user's
	// config dir.
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("mergeJSONConfig wrote a .bak for a file that did not exist")
	}
}

func lifecovReadServerEntry(t *testing.T, path, key string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	servers, ok := parsed[key].(map[string]any)
	if !ok {
		t.Fatalf("%s[%q] = %#v, want an object", path, key, parsed[key])
	}
	entry, ok := servers[mcpServerName].(map[string]any)
	if !ok {
		t.Fatalf("%s[%q][%q] = %#v, want an object", path, key, mcpServerName, servers[mcpServerName])
	}
	return entry
}
