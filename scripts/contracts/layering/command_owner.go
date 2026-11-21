// command_owner.go — Enforces internal/browsercommand.Owner as the only path to browser-command state.
//
// Rule (hard-fail, production files only):
//
//	command-owner — outside internal/queries, internal/browsercommand and
//	internal/capture/capture.go (which constructs the dispatcher and binds the
//	owner), no file may call X.Queries() or name queries.QueryDispatcher /
//	queries.NewQueryDispatcher. Workflows take the owner (or a small consumer
//	interface it satisfies, e.g. queries.QueryRunner) from the composition root.
package main

import (
	"fmt"
	"go/ast"
)

const captureCompositionFile = "internal/capture/capture.go"

// commandOwnerExempt reports whether a production file may touch the dispatcher.
func commandOwnerExempt(fileKey, pkgPath string) bool {
	return pkgPath == modulePrefix+"internal/queries" ||
		pkgPath == modulePrefix+"internal/browsercommand" ||
		fileKey == captureCompositionFile
}

// commandOwnerBreaches lists dispatcher references in one production file.
func commandOwnerBreaches(file *ast.File, fileKey, pkgPath string, aliases map[string]string) []string {
	if commandOwnerExempt(fileKey, pkgPath) {
		return nil
	}
	var breaches []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Queries" && len(node.Args) == 0 {
				breaches = append(breaches, fmt.Sprintf("command-owner: %s calls .Queries(); use the browsercommand.Owner (capture.Commands())", fileKey))
			}
		case *ast.SelectorExpr:
			ident, ok := node.X.(*ast.Ident)
			if !ok || aliases[ident.Name] != modulePrefix+"internal/queries" {
				return true
			}
			if node.Sel.Name == "QueryDispatcher" || node.Sel.Name == "NewQueryDispatcher" {
				breaches = append(breaches, fmt.Sprintf("command-owner: %s references queries.%s; only the browser-command owner may hold the dispatcher", fileKey, node.Sel.Name))
			}
		}
		return true
	})
	return breaches
}
