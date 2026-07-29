// codex_test.go — Native Codex TOML installation regression tests.
package nativeinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeCodexConfigPreservesUnrelatedContentAndReplacesManagedBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	existing := `# user comment
model = "gpt-5"

[mcp_servers.other]
command = "other"

[mcp_servers.kaboom-browser-devtools]
command = "old"
[mcp_servers.kaboom-browser-devtools.env]
OLD = "value"

[projects."/work"]
trust_level = "trusted"
`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := mergeCodexConfig(path, `/Applications/Kaboom "Beta"/kaboom`); err != nil {
		t.Fatalf("mergeCodexConfig() error = %v", err)
	}
	gotBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	for _, preserved := range []string{"# user comment", `model = "gpt-5"`, "[mcp_servers.other]", `[projects."/work"]`} {
		if !strings.Contains(got, preserved) {
			t.Fatalf("config lost %q:\n%s", preserved, got)
		}
	}
	if strings.Count(got, "[mcp_servers.kaboom-browser-devtools]") != 1 {
		t.Fatalf("managed block count != 1:\n%s", got)
	}
	if strings.Contains(got, `OLD = "value"`) {
		t.Fatalf("stale managed env subtable survived:\n%s", got)
	}
	if !strings.Contains(got, `default_tools_approval_mode = "approve"`) {
		t.Fatalf("Codex tools are not explicitly approved:\n%s", got)
	}
	if !strings.Contains(got, `command = "/Applications/Kaboom \"Beta\"/kaboom"`) {
		t.Fatalf("binary path was not TOML escaped:\n%s", got)
	}
}

func TestCodexConfigPathHonorsCodexHome(t *testing.T) {
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "custom-codex"))
	got := codexConfigPath("/ignored")
	if want := filepath.Join(os.Getenv("CODEX_HOME"), "config.toml"); got != want {
		t.Fatalf("codexConfigPath() = %q, want %q", got, want)
	}
}

func TestParseInstallTargetsRejectsUnknownAndSelectsCodex(t *testing.T) {
	targets, err := parseInstallTargets([]string{"codex"})
	if err != nil || !targets.codexOnly {
		t.Fatalf("parseInstallTargets(codex) = %+v, %v", targets, err)
	}
	if _, err := parseInstallTargets([]string{"codxe"}); err == nil {
		t.Fatal("unknown install target must fail instead of silently installing every client")
	}
}
