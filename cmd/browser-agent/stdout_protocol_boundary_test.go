// stdout_protocol_boundary_test.go — Guards stdout as an MCP-only transport.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionStdoutIsReservedForMCPTransport(t *testing.T) {
	t.Parallel()
	allowedTransportFiles := map[string]bool{
		"internal/bridge/stdioisolate/isolation.go":         true,
		"internal/bridge/stdioisolate/isolation_unix.go":    true,
		"internal/bridge/stdioisolate/isolation_windows.go": true,
	}
	var violations []string
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		normalized := filepath.ToSlash(path)
		normalized = strings.TrimPrefix(normalized, "./")
		if allowedTransportFiles[normalized] {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch expr := node.(type) {
			case *ast.SelectorExpr:
				if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "os" && expr.Sel.Name == "Stdout" {
					violations = append(violations, normalized+": direct os.Stdout reference")
				}
			case *ast.CallExpr:
				selector, ok := expr.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				if pkg.Name == "fmt" && (selector.Sel.Name == "Print" || selector.Sel.Name == "Printf" || selector.Sel.Name == "Println") {
					violations = append(violations, normalized+": fmt."+selector.Sel.Name)
				}
				if pkg.Name == "log" && strings.HasPrefix(selector.Sel.Name, "Print") {
					violations = append(violations, normalized+": log."+selector.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk production Go sources: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("stdout is reserved for MCP framing; route diagnostics to stderr:\n%s", strings.Join(violations, "\n"))
	}
}
