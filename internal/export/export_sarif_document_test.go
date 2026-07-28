// export_sarif_document_test.go — Tests SARIF document assembly and rule reuse.
// Docs: docs/features/feature/sarif-export/index.md

package export

import (
	"encoding/json"
	"testing"
)

// ============================================
// Full Export Tests
// ============================================

func TestExportSARIF_EmptyViolations(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(log.Runs))
	}
	if len(log.Runs[0].Results) != 0 {
		t.Errorf("Expected 0 results for empty violations, got %d", len(log.Runs[0].Results))
	}

	// Should still be valid SARIF
	data, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("Failed to marshal SARIF: %v", err)
	}
	if !json.Valid(data) {
		t.Error("Expected valid JSON output")
	}
}

func TestExportSARIF_MultipleViolations(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [
			{
				"id": "color-contrast",
				"impact": "serious",
				"description": "Color contrast",
				"help": "Must have contrast",
				"helpUrl": "https://example.com/color",
				"tags": ["wcag2aa"],
				"nodes": [
					{"html": "<span>a</span>", "target": ["span.a"], "impact": "serious"},
					{"html": "<span>b</span>", "target": ["span.b"], "impact": "serious"}
				]
			},
			{
				"id": "image-alt",
				"impact": "critical",
				"description": "Image alt text",
				"help": "Images must have alt",
				"helpUrl": "https://example.com/img",
				"tags": ["wcag2a"],
				"nodes": [
					{"html": "<img>", "target": ["img.x"], "impact": "critical"}
				]
			}
		],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	// 2 nodes from first violation + 1 from second = 3 results
	if len(log.Runs[0].Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(log.Runs[0].Results))
	}

	// 2 unique rules
	if len(log.Runs[0].Tool.Driver.Rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(log.Runs[0].Tool.Driver.Rules))
	}
}

func TestExportSARIF_Schema(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if log.Version != "2.1.0" {
		t.Errorf("Expected version '2.1.0', got %q", log.Version)
	}
	if log.Schema != "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json" {
		t.Errorf("Unexpected $schema: %q", log.Schema)
	}
	if log.Runs[0].Tool.Driver.Name != "Kaboom Agentic Browser" {
		t.Errorf("Expected tool name 'Kaboom Agentic Browser', got %q", log.Runs[0].Tool.Driver.Name)
	}
	if log.Runs[0].Tool.Driver.Version != version {
		t.Errorf("Expected tool version %q, got %q", version, log.Runs[0].Tool.Driver.Version)
	}
	if log.Runs[0].Tool.Driver.InformationURI != "https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP" {
		t.Errorf("Unexpected informationUri: %q", log.Runs[0].Tool.Driver.InformationURI)
	}
}

func TestExportSARIF_RulesDeduplication(t *testing.T) {
	t.Parallel()
	// Same rule ID appearing in multiple results should only produce 1 rule entry
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Color contrast",
			"help": "Must have contrast",
			"helpUrl": "https://example.com/color",
			"tags": ["wcag2aa"],
			"nodes": [
				{"html": "<span>a</span>", "target": ["span.a"], "impact": "serious"},
				{"html": "<span>b</span>", "target": ["span.b"], "impact": "serious"},
				{"html": "<span>c</span>", "target": ["span.c"], "impact": "serious"}
			]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs[0].Tool.Driver.Rules) != 1 {
		t.Errorf("Expected 1 deduplicated rule, got %d", len(log.Runs[0].Tool.Driver.Rules))
	}
	if len(log.Runs[0].Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(log.Runs[0].Results))
	}
}

func TestExportSARIF_IncludePasses(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Color contrast issue",
			"help": "Fix contrast",
			"helpUrl": "https://example.com",
			"tags": ["wcag2aa"],
			"nodes": [{"html": "<p>text</p>", "target": ["p"], "impact": "serious"}]
		}],
		"passes": [{
			"id": "image-alt",
			"impact": "critical",
			"description": "Images have alt text",
			"help": "Good alt text",
			"helpUrl": "https://example.com/alt",
			"tags": ["wcag2a"],
			"nodes": [{"html": "<img alt='photo'>", "target": ["img"], "impact": "minor"}]
		}],
		"incomplete": []
	}`)

	log, err := ExportSARIF(a11yResult, SARIFExportOptions{IncludePasses: true})
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	run := log.Runs[0]
	// Should have 2 rules: one from violations, one from passes
	if len(run.Tool.Driver.Rules) != 2 {
		t.Errorf("Expected 2 rules (violation + pass), got %d", len(run.Tool.Driver.Rules))
	}
	// Should have 2 results
	if len(run.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(run.Results))
	}
}
