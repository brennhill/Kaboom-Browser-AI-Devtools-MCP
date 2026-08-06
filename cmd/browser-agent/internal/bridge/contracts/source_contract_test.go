// source_contract_test.go — Static safety contracts for bridge lifecycle code.
package contracts_test

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bridgeDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve bridge directory: %v", err)
	}
	return dir
}

func findFuncDecl(t *testing.T, name string) (*ast.FuncDecl, *token.FileSet) {
	t.Helper()
	entries, err := os.ReadDir(bridgeDir(t))
	if err != nil {
		t.Fatalf("read bridge package: %v", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(fileName, ".go") || strings.HasSuffix(fileName, "_test.go") {
			continue
		}
		path := filepath.Join(bridgeDir(t), fileName)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", fileName, err)
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok && function.Name.Name == name {
				return function, fset
			}
		}
	}
	t.Fatalf("%s not found in bridge source", name)
	return nil, nil
}

func TestForwardRequestDoesNotReassignCancel(t *testing.T) {
	function, _ := findFuncDecl(t, "bridgeForwardRequest")
	ast.Inspect(function.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || assignment.Tok != token.ASSIGN {
			return true
		}
		for _, left := range assignment.Lhs {
			if identifier, ok := left.(*ast.Ident); ok && identifier.Name == "cancel" {
				t.Fatal("bridgeForwardRequest reassigns cancel; retry contexts require a fresh cancel variable")
			}
		}
		return true
	})
}

func TestForwardRequestDoesNotUseFragileRetryAssignment(t *testing.T) {
	function, fset := findFuncDecl(t, "bridgeForwardRequest")
	for _, statement := range function.Body.List {
		assignment, ok := statement.(*ast.AssignStmt)
		if !ok {
			continue
		}
		var rendered strings.Builder
		if err := format.Node(&rendered, fset, assignment); err != nil {
			t.Fatalf("render assignment: %v", err)
		}
		if strings.Contains(rendered.String(), "ctx, cancel = context.WithTimeout") {
			t.Fatalf("fragile retry assignment remains: %q", rendered.String())
		}
	}
}

func TestBridgeHasNoGlobalDependencyLocator(t *testing.T) {
	entries, err := os.ReadDir(bridgeDir(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, readErr := os.ReadFile(filepath.Join(bridgeDir(t), entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, forbidden := range []string{"var deps ", "func Init(", "type Deps struct"} {
			if strings.Contains(string(source), forbidden) {
				t.Fatalf("%s retains obsolete dependency surface %q", entry.Name(), forbidden)
			}
		}
	}
}

func TestForwardRequestUsesSerializedFramingAwareTransport(t *testing.T) {
	function, fset := findFuncDecl(t, "bridgeForwardRequest")
	var rendered strings.Builder
	if err := format.Node(&rendered, fset, function.Body); err != nil {
		t.Fatalf("render bridgeForwardRequest: %v", err)
	}
	source := rendered.String()
	if !strings.Contains(source, "r.transport.Write(body, framing)") {
		t.Fatal("bridgeForwardRequest must forward HTTP bodies through the serialized framing-aware transport")
	}
	if strings.Contains(source, "fmt.Println(string(body))") {
		t.Fatal("bridgeForwardRequest writes response bodies directly to stdout")
	}
}
