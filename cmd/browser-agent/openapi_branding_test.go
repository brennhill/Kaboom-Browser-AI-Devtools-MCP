// openapi_branding_test.go — Branding contract for the root-owned HTTP API specification.

package main

import (
	"os"
	"strings"
	"testing"
)

func TestOpenAPISpecUsesKaboomBranding(t *testing.T) {
	content, err := os.ReadFile("openapi.json")
	if err != nil {
		t.Fatalf("os.ReadFile(openapi.json) error = %v", err)
	}
	for _, want := range []string{"Kaboom MCP Server", "X-Kaboom-Client"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("openapi.json should contain %q", want)
		}
	}
}
