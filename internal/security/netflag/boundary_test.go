// boundary_test.go — Enforces the leaf network-security package boundary.
package netflag

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestNetflagDoesNotImportCaptureOrSiblingSecurityPackages(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		for _, imported := range file.Imports {
			path, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				t.Fatalf("unquote import in %s: %v", entry.Name(), unquoteErr)
			}
			if strings.HasSuffix(path, "/internal/capture") ||
				(strings.Contains(path, "/internal/security/") && !strings.HasSuffix(path, "/internal/security/netflag")) {
				t.Fatalf("%s imports forbidden broad/sibling owner %q", entry.Name(), path)
			}
		}
	}
}

func TestNetflagRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("internal/security/netflag has %d files; want at most 10 change-coupled owners", files)
	}
}
