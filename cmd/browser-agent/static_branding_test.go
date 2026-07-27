// static_branding_test.go — Branding contracts for server-owned static assets.

package main

import (
	"os"
	"strings"
	"testing"
)

func TestGoStaticContractsUseKaboomBranding(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{path: "setup.html", want: []string{"Kaboom MCP Server", "kaboom-mcp"}},
		{path: "docs.html", want: []string{"Kaboom MCP Server"}},
		{path: "logs.html", want: []string{"Kaboom MCP Server"}},
		{path: "openapi.json", want: []string{"Kaboom MCP Server", "X-Kaboom-Client"}},
	}
	for _, test := range tests {
		content, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatalf("os.ReadFile(%s) error = %v", test.path, err)
		}
		for _, want := range test.want {
			if !strings.Contains(string(content), want) {
				t.Errorf("%s should contain %q", test.path, want)
			}
		}
	}
}
