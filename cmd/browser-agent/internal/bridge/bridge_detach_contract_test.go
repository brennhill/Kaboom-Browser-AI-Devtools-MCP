// Purpose: Tests for bridge detached process spawning contract.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package bridge

import (
	"go/ast"
	"testing"
)

// TestBridgeSpawnPathsSetDetachedProcess enforces that the daemon command builder
// always detaches child processes from the caller session.
// Both spawnDaemonAsync and respawnIfNeeded delegate to buildDaemonCmd,
// so we verify that buildDaemonCmd calls util.SetDetachedProcess.
func TestBridgeSpawnPathsSetDetachedProcess(t *testing.T) {
	t.Parallel()

	// Located by symbol, not by filename: a contract test that can be silenced by
	// moving code between files is not a contract test.
	fn, _ := findFuncDecl(t, "buildDaemonCmd")

	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if pkgIdent.Name == "util" && sel.Sel.Name == "SetDetachedProcess" {
			found = true
			return false
		}
		return true
	})

	if !found {
		t.Fatal("buildDaemonCmd must call util.SetDetachedProcess(cmd) before returning")
	}
}
