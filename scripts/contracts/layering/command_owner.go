// command_owner.go — Enforces internal/browsercommand.Owner as the only path to browser-command state.
//
// Rule (hard-fail, production files only):
//
//	command-owner — the query dispatcher lives in
//	internal/browsercommand/internal/dispatch (the Go compiler already keeps
//	other trees out). Outside internal/browsercommand/** no production file may
//	import that package, and no production file anywhere may call X.Queries()
//	(a dispatcher accessor that bypasses the owner). Workflows take the owner
//	(or a small consumer interface it satisfies, e.g. queries.QueryRunner)
//	from the composition root; tests use internal/browsercommand/commandtest.
package main

import (
	"fmt"
	"go/ast"
	"strings"
)

const dispatchPackage = modulePrefix + "internal/browsercommand/internal/dispatch"

// commandOwnerExempt reports whether a production package may import the dispatcher.
func commandOwnerExempt(pkgPath string) bool {
	return isPkgOrSub(pkgPath, modulePrefix+"internal/browsercommand")
}

// commandOwnerBreaches lists dispatcher references in one production file.
func commandOwnerBreaches(file *ast.File, fileKey, pkgPath string) []string {
	var breaches []string
	if !commandOwnerExempt(pkgPath) {
		for _, imp := range file.Imports {
			if strings.Trim(imp.Path.Value, `"`) == dispatchPackage {
				breaches = append(breaches, fmt.Sprintf("command-owner: %s imports the internal dispatcher; use the browsercommand.Owner", fileKey))
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Queries" && len(call.Args) == 0 {
			breaches = append(breaches, fmt.Sprintf("command-owner: %s calls .Queries(); use the browsercommand.Owner (capture.Commands())", fileKey))
		}
		return true
	})
	return breaches
}
