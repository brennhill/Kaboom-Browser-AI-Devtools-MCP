// Purpose: Golden-file snapshot tests for browser-agent output stability.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// csp_golden_test.go — Golden file validation for CSP directive generation.
package main

import (
	"bytes"
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"os"
	"sort"
	"testing"

	gen "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/generate"
)

func TestGoldenCSPModerate(t *testing.T) {
	networkBodies := []types.NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript"},
		{URL: "https://cdn.example.com/vendor.js", ContentType: "application/javascript"},
		{URL: "https://cdn.example.com/styles.css", ContentType: "text/css"},
		{URL: "https://fonts.googleapis.com/css2", ContentType: "text/css"},
		{URL: "https://fonts.gstatic.com/s/roboto/v30/regular.woff2", ContentType: "font/woff2"},
		{URL: "https://images.example.com/logo.png", ContentType: "image/png"},
		{URL: "https://images.example.com/hero.jpg", ContentType: "image/jpeg"},
		{URL: "https://api.example.com/users", ContentType: "application/json"},
	}

	directives := gen.BuildCSPDirectives(networkBodies)

	// Sort directive values for deterministic output
	sortedDirectives := make(map[string][]string)
	for k, v := range directives {
		sorted := make([]string, len(v))
		copy(sorted, v)
		sort.Strings(sorted)
		sortedDirectives[k] = sorted
	}

	data, err := json.MarshalIndent(sortedDirectives, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}
	data = append(data, '\n')

	goldenPath := "testdata/csp-moderate.golden.json"

	if updateGolden {
		err = os.WriteFile(goldenPath, data, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		t.Logf("Updated golden file (%d bytes, %d directives)", len(data), len(sortedDirectives))
	} else {
		goldenData, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("Failed to read golden file (run with UPDATE_GOLDEN=1 first): %v", err)
		}

		if !bytes.Equal(data, goldenData) {
			t.Errorf("Golden file mismatch for %s", goldenPath)
			t.Errorf("Expected:\n%s", string(goldenData))
			t.Errorf("Got:\n%s", string(data))
			t.Fatalf("Run with UPDATE_GOLDEN=1 to update golden files")
		}
		t.Logf("CSP golden file validation passed (%d bytes, %d directives)", len(data), len(sortedDirectives))
	}
}
