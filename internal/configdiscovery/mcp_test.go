// mcp_test.go — Verifies managed MCP configuration discovery.

package configdiscovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContainsManagedMCPConfigRecognizesCurrentAndLegacyNames(t *testing.T) {
	for _, name := range []string{
		"kaboom-browser-devtools",
		"kaboom-agentic-browser",
		"gasoline-agentic-browser",
		"strum-browser-devtools",
	} {
		if !ContainsManagedMCPConfig(`{"mcpServers":{"` + name + `":{}}}`) {
			t.Fatalf("expected %q to be recognized", name)
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
