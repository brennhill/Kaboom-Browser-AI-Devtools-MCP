// mcp_test.go — Verifies managed MCP configuration discovery.

package configdiscovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContainsManagedMCPConfigRecognizesOnlyCanonicalName(t *testing.T) {
	if !ContainsManagedMCPConfig(`{"mcpServers":{"kaboom-browser-devtools":{}}}`) {
		t.Fatal("canonical MCP config should be recognized")
	}
	for _, name := range []string{"kaboom", "kaboom-agentic-browser", "gasoline", "strum"} {
		if ContainsManagedMCPConfig(`{"mcpServers":{"` + name + `":{}}}`) {
			t.Fatalf("non-canonical MCP name %q should not be recognized", name)
		}
	}
	if ContainsManagedMCPConfig(`{"mcpServers":{"github":{}}}`) {
		t.Fatal("unmanaged MCP config should not be recognized")
	}
}

func TestFindPrefersProjectLocalConfig(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDir) })

	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".mcp.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if got := Find(); got != ".mcp.json" {
		t.Fatalf("Find() = %q, want .mcp.json", got)
	}
}

func TestFindDiscoversOnlyManagedUserConfigurations(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDir) })
	project := t.TempDir()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := Find(); got != "" {
		t.Fatalf("Find() without configs = %q", got)
	}
	claude := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(claude, []byte(`{"mcpServers":{"github":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cursor := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(cursor), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cursor, []byte(`{"mcpServers":{"kaboom-browser-devtools":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Find(); got != cursor {
		t.Fatalf("Find() = %q, want %q", got, cursor)
	}
}
