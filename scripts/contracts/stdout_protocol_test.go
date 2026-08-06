// stdout_protocol_test.go — Guards stdout as an MCP-only transport.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProductionStdoutIsReservedForMCPTransport(t *testing.T) {
	t.Parallel()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract source path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	root := filepath.Join(repositoryRoot, "cmd", "browser-agent")
	allowed := map[string]bool{
		"internal/bridge/stdioisolate/isolation.go":         true,
		"internal/bridge/stdioisolate/isolation_unix.go":    true,
		"internal/bridge/stdioisolate/isolation_windows.go": true,
	}
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		normalized := filepath.ToSlash(relative)
		if allowed[normalized] {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch expression := node.(type) {
			case *ast.SelectorExpr:
				if ident, ok := expression.X.(*ast.Ident); ok && ident.Name == "os" && expression.Sel.Name == "Stdout" {
					violations = append(violations, normalized+": direct os.Stdout reference")
				}
			case *ast.CallExpr:
				selector, ok := expression.Fun.(*ast.SelectorExpr)
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
