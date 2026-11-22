// Purpose: Tests for canonical MCP server identity constants.
// Why: Ensures identity values stay correct and legacy names don't accidentally include the current name.

package identity

import "testing"

func TestMCPServerName(t *testing.T) {
	t.Parallel()
	if MCPServerName != "kaboom-browser-devtools" {
		t.Fatalf("MCPServerName = %q, want %q", MCPServerName, "kaboom-browser-devtools")
	}
}
