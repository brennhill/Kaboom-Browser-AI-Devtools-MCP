package nativeinstall

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
)

// TestFileConfigTargets_VSCodeUsesServersKey is the regression test for the
// VS Code MCP config key: VS Code reads servers from a top-level "servers"
// key in mcp.json, not "mcpServers".
func TestFileConfigTargets_VSCodeUsesServersKey(t *testing.T) {
	configs := fileConfigTargets("/Users/tester")

	foundVSCode := false
	for _, cfg := range configs {
		if cfg.name != "VS Code" {
			continue
		}
		foundVSCode = true
		if cfg.key != "servers" {
			t.Errorf("VS Code config key = %q, want %q", cfg.key, "servers")
		}
		if cfg.isCustom {
			t.Error("VS Code config should use the standard {command,args} entry shape")
		}
	}
	if !foundVSCode {
		t.Fatal("fileConfigTargets missing a VS Code entry for this OS")
	}

	// Other standard clients keep the conventional mcpServers key.
	for _, name := range []string{"Cursor", "Windsurf", "Gemini CLI"} {
		for _, cfg := range configs {
			if cfg.name == name && cfg.key != "mcpServers" {
				t.Errorf("%s config key = %q, want %q", name, cfg.key, "mcpServers")
			}
		}
	}
}

func TestRunInstallerCodexOnlyUsesInjectedDaemonStarter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("PATH", "")
	t.Setenv("KABOOM_NO_OPEN", "1")
	t.Setenv("KABOOM_INSTALL_NO_WAIT", "1")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	cleanupCalled := false
	startedWith := ""
	err := runInstaller(
		func() error {
			cleanupCalled = true
			return nil
		},
		func(executable string) {
			startedWith = executable
		},
		"codex",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !cleanupCalled {
		t.Fatal("force cleanup was not called")
	}
	if startedWith == "" {
		t.Fatal("daemon starter did not receive the executable")
	}
	config, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), `[mcp_servers.`+identity.MCPServerName+`]`) ||
		!strings.Contains(string(config), startedWith) {
		t.Fatalf("Codex config = %s", config)
	}
}

func TestRunInstallerConfiguresDetectedFileClients(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, "missing-codex"))
	t.Setenv("PATH", "")
	t.Setenv("KABOOM_NO_OPEN", "1")
	t.Setenv("KABOOM_INSTALL_NO_WAIT", "1")

	var expectedPaths []string
	for _, target := range fileConfigTargets(home) {
		path := target.path
		if strings.HasPrefix(path, "~/") {
			path = filepath.Join(home, path[2:])
		} else if !filepath.IsAbs(path) {
			path = filepath.Join(home, path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		expectedPaths = append(expectedPaths, path)
	}

	started := false
	if err := runInstaller(nil, func(string) { started = true }); err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("daemon starter was not called")
	}
	for _, path := range expectedPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(data), identity.MCPServerName) {
			t.Errorf("%s missing managed server: %s", path, data)
		}
	}
}

func TestRunInstallerAlwaysConfiguresCodex(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, "not-yet-created", ".codex")
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("PATH", "")
	t.Setenv("KABOOM_NO_OPEN", "1")
	t.Setenv("KABOOM_INSTALL_NO_WAIT", "1")

	if err := runInstaller(nil, func(string) {}); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(codexHome, "config.toml"))
	if err != nil {
		t.Fatalf("Codex config was not created: %v", err)
	}
	if !strings.Contains(string(config), identity.MCPServerName) {
		t.Fatalf("Codex config missing managed server: %s", config)
	}
}

// TestEnvWithout_RemovesVariable verifies the variable is absent entirely,
// not merely present-but-empty (which nesting guards still detect).
func TestEnvWithout_RemovesVariable(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")

	env := envWithout("CLAUDECODE")
	for _, kv := range env {
		if strings.HasPrefix(kv, "CLAUDECODE=") {
			t.Fatalf("envWithout left %q in the environment", kv)
		}
	}

	// Unrelated variables must survive.
	t.Setenv("KABOOM_ENVWITHOUT_PROBE", "keep")
	env = envWithout("CLAUDECODE")
	found := false
	for _, kv := range env {
		if kv == "KABOOM_ENVWITHOUT_PROBE=keep" {
			found = true
		}
	}
	if !found {
		t.Fatal("envWithout dropped an unrelated environment variable")
	}
}

func TestManualExtensionSetupChecklist_IncludesRequiredSteps(t *testing.T) {
	extPath := `/Users/tester/KaboomAgenticDevtoolExtension`
	checklist := manualExtensionSetupChecklist(extPath)
	joined := strings.Join(checklist, "\n")

	required := []string{
		"MANUAL STEP REQUIRED",
		"cannot click browser UI controls",
		"chrome://extensions (or brave://extensions)",
		"Enable Developer mode",
		"Load unpacked",
		extPath,
		"Pin Kaboom",
		"Track This Tab",
	}

	for _, want := range required {
		if !strings.Contains(joined, want) {
			t.Fatalf("checklist missing %q; got:\n%s", want, joined)
		}
	}
}

func TestExtensionInstallDir_DefaultVisiblePath(t *testing.T) {
	t.Setenv("KABOOM_EXTENSION_DIR", "")
	home := "/Users/tester"
	want := filepath.Join(home, "KaboomAgenticDevtoolExtension")

	if got := extensionInstallDir(home); got != want {
		t.Fatalf("extensionInstallDir(%q) = %q, want %q", home, got, want)
	}
}

func TestExtensionInstallDir_EnvOverride(t *testing.T) {
	override := "/tmp/custom-kaboom-ext"
	t.Setenv("KABOOM_EXTENSION_DIR", override)
	home := "/Users/tester"

	if got := extensionInstallDir(home); got != override {
		t.Fatalf("extensionInstallDir(%q) = %q, want env override %q", home, got, override)
	}
}

func TestOpenExtensionFolderWithPlatformOpener(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell script")
	}
	dir := t.TempDir()
	bin := t.TempDir()
	opener, _, ok := fileManagerOpenCommand(runtime.GOOS, dir)
	if !ok {
		t.Skip("platform has no desktop folder opener")
	}
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, opener), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("KABOOM_NO_OPEN", "")
	t.Setenv("KABOOM_INSTALL_NO_OPEN", "")
	if !openExtensionFolder(dir) {
		t.Fatal("openExtensionFolder returned false")
	}
}

func TestOpenExtensionFolderRejectsUnavailableTargets(t *testing.T) {
	t.Setenv("KABOOM_NO_OPEN", "1")
	if openExtensionFolder(t.TempDir()) {
		t.Fatal("opt-out was ignored")
	}
	t.Setenv("KABOOM_NO_OPEN", "")
	t.Setenv("PATH", "")
	if openExtensionFolder("") || openExtensionFolder(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("invalid directory was opened")
	}
	if openExtensionFolder(t.TempDir()) {
		t.Fatal("missing opener was treated as available")
	}
}

func TestInstallClaudeCodeSuccessAndFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell script")
	}
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "claude.log")
	claudePath := filepath.Join(bin, "claude")
	success := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\n/bin/cat >> " + logPath + "\n"
	if err := os.WriteFile(claudePath, []byte(success), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("CLAUDECODE", "nested")
	if err := installClaudeCode("/opt/kaboom"); err != nil {
		t.Fatal(err)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logged), "mcp remove") ||
		!strings.Contains(string(logged), "mcp add-json") ||
		!strings.Contains(string(logged), `"/opt/kaboom"`) {
		t.Fatalf("claude invocations = %s", logged)
	}

	failure := "#!/bin/sh\nif [ \"$2\" = \"add-json\" ]; then echo rejected >&2; exit 7; fi\n"
	if err := os.WriteFile(claudePath, []byte(failure), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installClaudeCode("/opt/kaboom"); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("installClaudeCode error = %v", err)
	}
}

func TestStartDaemonSilentlyRunsDetachedExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell script")
	}
	exe := filepath.Join(t.TempDir(), "kaboom")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	startDaemonSilently(exe)
}
