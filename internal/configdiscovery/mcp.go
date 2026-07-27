// mcp.go — Discovers project and user MCP configurations managed by Kaboom.

package configdiscovery

import (
	"os"
	"path/filepath"
	"strings"
)

func Find() string {
	if _, err := os.Stat(".mcp.json"); err == nil {
		return ".mcp.json"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	locations := []string{
		filepath.Join(home, ".claude.json"),
		filepath.Join(home, ".cursor", "mcp.json"),
		filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"),
		filepath.Join(home, ".continue", "config.json"),
		filepath.Join(home, ".config", "zed", "settings.json"),
	}
	for _, path := range locations {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		data, err := os.ReadFile(path) // #nosec G304 -- fixed client configuration paths.
		if err == nil && ContainsManagedMCPConfig(string(data)) {
			return path
		}
	}
	return ""
}

func ContainsManagedMCPConfig(data string) bool {
	for _, name := range []string{
		"kaboom-browser-devtools",
		"kaboom-agentic-browser",
		"kaboom",
		"gasoline-browser-devtools",
		"gasoline-agentic-browser",
		"gasoline",
		"strum-browser-devtools",
		"strum-agentic-browser",
		"strum",
	} {
		if strings.Contains(data, name) {
			return true
		}
	}
	return false
}
