// branding_test.go — Branding contract for the root-owned HTTP API specification.

package openapibranding

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenAPISpecUsesKaboomBranding(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "cmd", "browser-agent", "openapi.json"))
	if err != nil {
		t.Fatalf("os.ReadFile(openapi.json) error = %v", err)
	}
	for _, want := range []string{"Kaboom MCP Server", "X-Kaboom-Client"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("openapi.json should contain %q", want)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract source path")
	}
	for dir := filepath.Dir(sourceFile); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			t.Fatal("find repository root containing go.mod")
		}
	}
}
