// contexts_test.go — Regression tests for the bounded-context dependency rules in contexts.go.
package main

import (
	"strings"
	"testing"
)

func TestBoundedContextsRejectUpstreamImports(t *testing.T) {
	root := t.TempDir()
	p := modulePrefix
	writeTree(t, root, map[string]string{
		// Observation Capture stores are leaves: no commands, sync, recording, MCP or HTTP.
		"internal/observation/store/telemetry/a.go": "package telemetrystore\n\nimport _ \"" + p + "internal/browsercommand\"\n",
		"internal/observation/store/logs/b.go":      "package logstore\n\nimport _ \"net/http\"\n",
		"internal/observation/store/perf/c.go":      "package perfstore\n\nimport _ \"" + p + "internal/composition/capture\"\n",
		// Event Recording never reaches the command lifecycle or the sync runtime.
		"internal/recording/d.go":          "package recording\n\nimport _ \"" + p + "internal/browsercommand/query\"\n",
		"internal/recording/playback/e.go": "package playback\n\nimport _ \"" + p + "internal/sync/runtime\"\n",
		// Connection & Sync does not know about tools, use cases or event recording.
		"internal/sync/runtime/f.go": "package syncruntime\n\nimport _ \"" + p + "internal/recording\"\n",
		// Extension HTTP ingestion does not reach the MCP tool layer.
		"internal/ingestion/http/g.go": "package httpingest\n\nimport _ \"" + p + "internal/tools/observe/core\"\n",

		// Clean: allowed downstream dependencies.
		"internal/observation/store/telemetry/ok.go": "package telemetrystore\n\nimport _ \"" + p + "internal/observation/store/ring\"\n",
		"internal/recording/actionlog/ok.go":         "package actionlog\n\nimport _ \"" + p + "internal/observation/store/telemetry\"\n",
		"internal/sync/runtime/ok.go":                "package syncruntime\n\nimport (\n\t_ \"net/http\"\n\t_ \"" + p + "internal/browsercommand\"\n\t_ \"" + p + "internal/observation/store/logs\"\n)\n",
		"internal/ingestion/http/ok.go":              "package httpingest\n\nimport _ \"" + p + "internal/recording\"\n",
		// Test files are exempt.
		"internal/observation/store/logs/x_test.go": "package logstore\n\nimport _ \"" + p + "internal/browsercommand\"\n",
	})
	scan, err := scanTree(root)
	if err != nil {
		t.Fatal(err)
	}
	violations := evaluateMatrix(scan)
	want := []string{
		"internal/observation/store/telemetry/a.go (Observation Capture) imports " + p + "internal/browsercommand",
		"internal/observation/store/logs/b.go (Observation Capture) imports net/http",
		"internal/observation/store/perf/c.go (Observation Capture) imports " + p + "internal/composition/capture",
		"internal/recording/d.go (Event Recording) imports " + p + "internal/browsercommand/query",
		"internal/recording/playback/e.go (Event Recording) imports " + p + "internal/sync/runtime",
		"internal/sync/runtime/f.go (Connection & Sync) imports " + p + "internal/recording",
		"internal/ingestion/http/g.go (Extension Ingestion) imports " + p + "internal/tools/observe/core",
	}
	if len(violations) != len(want) {
		t.Fatalf("violations = %v\nwant %d context breaches", violations, len(want))
	}
	for _, needle := range want {
		found := false
		for _, v := range violations {
			if strings.HasPrefix(v, "context: ") && strings.Contains(v, needle) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing context violation %q in %v", needle, violations)
		}
	}
}

func TestBoundedContextNamesAreUniqueAndDocumented(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range boundedContexts() {
		if seen[c.name] {
			t.Errorf("duplicate bounded context %q", c.name)
		}
		seen[c.name] = true
		if len(c.packages) == 0 || (len(c.forbidden) == 0 && len(c.forbiddenExact) == 0) || c.why == "" {
			t.Errorf("bounded context %q must declare packages, forbidden dependencies and why", c.name)
		}
	}
}
