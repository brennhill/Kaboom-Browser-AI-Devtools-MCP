// Purpose: Tests for SARIF accessibility report export.
// Docs: docs/features/feature/har-export/index.md

// These were gated behind `//go:build integration` with a note saying they "require
// MCP handler types that aren't exported". They reference no MCP type at all — the
// imports below are stdlib only, and the file is in package export. The tag is gone
// and these tests run by default.
package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// Conversion Tests
// ============================================

func TestAxeViolationToSARIFResult(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Ensures the contrast between foreground and background colors meets WCAG 2 AA minimum contrast ratio thresholds",
			"help": "Elements must meet minimum color contrast ratio thresholds",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/color-contrast",
			"tags": ["cat.color", "wcag2aa", "wcag143"],
			"nodes": [{
				"html": "<span class=\"low-contrast\">Hard to read</span>",
				"target": ["#main > .content > span.low-contrast"],
				"impact": "serious"
			}]
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

	if len(log.Runs) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(log.Runs))
	}

	if len(log.Runs[0].Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(log.Runs[0].Results))
	}

	result := log.Runs[0].Results[0]
	if result.RuleID != "color-contrast" {
		t.Errorf("Expected ruleId 'color-contrast', got %q", result.RuleID)
	}
	if result.Level != "error" {
		t.Errorf("Expected level 'error' for serious impact, got %q", result.Level)
	}
	if result.Message.Text == "" {
		t.Error("Expected non-empty message text")
	}
}

func TestAxeImpactToSARIFLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		impact   string
		expected string
	}{
		{"critical", "error"},
		{"serious", "error"},
		{"moderate", "warning"},
		{"minor", "note"},
		{"", "warning"},        // unknown defaults to warning
		{"unknown", "warning"}, // unknown defaults to warning
	}

	for _, tc := range tests {
		t.Run(tc.impact, func(t *testing.T) {
			got := axeImpactToLevel(tc.impact)
			if got != tc.expected {
				t.Errorf("axeImpactToLevel(%q) = %q, want %q", tc.impact, got, tc.expected)
			}
		})
	}
}

func TestAxeNodeToSARIFLocation(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "image-alt",
			"impact": "critical",
			"description": "Images must have alternate text",
			"help": "Images must have alternate text",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/image-alt",
			"tags": ["wcag2a"],
			"nodes": [{
				"html": "<img src=\"photo.jpg\">",
				"target": ["img.hero-image"],
				"impact": "critical"
			}]
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

	result := log.Runs[0].Results[0]
	if len(result.Locations) != 1 {
		t.Fatalf("Expected 1 location, got %d", len(result.Locations))
	}

	loc := result.Locations[0]
	if loc.PhysicalLocation.ArtifactLocation.URI != "img.hero-image" {
		t.Errorf("Expected URI 'img.hero-image', got %q", loc.PhysicalLocation.ArtifactLocation.URI)
	}
	if loc.PhysicalLocation.Region.Snippet.Text != "<img src=\"photo.jpg\">" {
		t.Errorf("Expected snippet with img html, got %q", loc.PhysicalLocation.Region.Snippet.Text)
	}
}

func TestAxeViolationToSARIFRule(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Ensures the contrast between foreground and background colors meets WCAG 2 AA",
			"help": "Elements must meet minimum color contrast ratio thresholds",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/color-contrast",
			"tags": ["cat.color", "wcag2aa", "wcag143"],
			"nodes": [{
				"html": "<span>text</span>",
				"target": ["span.low"],
				"impact": "serious"
			}]
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

	rules := log.Runs[0].Tool.Driver.Rules
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}

	rule := rules[0]
	if rule.ID != "color-contrast" {
		t.Errorf("Expected rule ID 'color-contrast', got %q", rule.ID)
	}
	if rule.ShortDescription.Text != "Ensures the contrast between foreground and background colors meets WCAG 2 AA" {
		t.Errorf("Unexpected shortDescription: %q", rule.ShortDescription.Text)
	}
	if rule.FullDescription.Text != "Elements must meet minimum color contrast ratio thresholds" {
		t.Errorf("Unexpected fullDescription: %q", rule.FullDescription.Text)
	}
	if rule.HelpURI != "https://dequeuniversity.com/rules/axe/4.10/color-contrast" {
		t.Errorf("Unexpected helpUri: %q", rule.HelpURI)
	}
}

func TestMultipleNodesCreateMultipleResults(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Color contrast check",
			"help": "Must meet contrast",
			"helpUrl": "https://example.com",
			"tags": ["wcag2aa"],
			"nodes": [
				{"html": "<span>one</span>", "target": ["span.a"], "impact": "serious"},
				{"html": "<span>two</span>", "target": ["span.b"], "impact": "serious"},
				{"html": "<span>three</span>", "target": ["span.c"], "impact": "serious"}
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

	if len(log.Runs[0].Results) != 3 {
		t.Errorf("Expected 3 results (one per node), got %d", len(log.Runs[0].Results))
	}

	// All results should reference the same rule
	for _, r := range log.Runs[0].Results {
		if r.RuleID != "color-contrast" {
			t.Errorf("Expected all results to have ruleId 'color-contrast', got %q", r.RuleID)
		}
		if r.RuleIndex != 0 {
			t.Errorf("Expected ruleIndex 0, got %d", r.RuleIndex)
		}
	}
}

func TestWCAGTagExtraction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		tags     []string
		expected []string
	}{
		{
			name:     "mixed tags",
			tags:     []string{"cat.color", "wcag2aa", "wcag143", "TTv5", "TT6.a"},
			expected: []string{"wcag2aa", "wcag143"},
		},
		{
			name:     "no wcag tags",
			tags:     []string{"cat.color", "TTv5"},
			expected: []string{},
		},
		{
			name:     "all wcag tags",
			tags:     []string{"wcag2a", "wcag2aa", "wcag21aa"},
			expected: []string{"wcag2a", "wcag2aa", "wcag21aa"},
		},
		{
			name:     "empty tags",
			tags:     []string{},
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractWCAGTags(tc.tags)
			if len(got) != len(tc.expected) {
				t.Errorf("extractWCAGTags(%v) returned %d tags, want %d", tc.tags, len(got), len(tc.expected))
				return
			}
			for i := range got {
				if got[i] != tc.expected[i] {
					t.Errorf("extractWCAGTags(%v)[%d] = %q, want %q", tc.tags, i, got[i], tc.expected[i])
				}
			}
		})
	}
}

func TestPassesIncludedWhenRequested(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [{
			"id": "button-name",
			"impact": "critical",
			"description": "Buttons must have discernible text",
			"help": "Buttons must have discernible text",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/button-name",
			"tags": ["wcag2a", "wcag412"],
			"nodes": [{
				"html": "<button>Submit</button>",
				"target": ["button.submit"],
				"impact": "critical"
			}]
		}],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{IncludePasses: true}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs[0].Results) != 1 {
		t.Fatalf("Expected 1 result from passes, got %d", len(log.Runs[0].Results))
	}

	result := log.Runs[0].Results[0]
	if result.Level != "none" {
		t.Errorf("Expected level 'none' for passes, got %q", result.Level)
	}
	if result.RuleID != "button-name" {
		t.Errorf("Expected ruleId 'button-name', got %q", result.RuleID)
	}
}

func TestPassesExcludedByDefault(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [{
			"id": "button-name",
			"impact": "critical",
			"description": "Buttons must have discernible text",
			"help": "Buttons must have discernible text",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/button-name",
			"tags": ["wcag2a"],
			"nodes": [{
				"html": "<button>Submit</button>",
				"target": ["button.submit"],
				"impact": "critical"
			}]
		}],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{IncludePasses: false}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	if len(log.Runs[0].Results) != 0 {
		t.Errorf("Expected 0 results when passes excluded, got %d", len(log.Runs[0].Results))
	}
}

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

// ============================================
// File Save Tests
// ============================================

func TestExportSARIF_SaveToFile(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "image-alt",
			"impact": "critical",
			"description": "Images must have alt text",
			"help": "Add alt attribute",
			"helpUrl": "https://example.com",
			"tags": ["wcag2a"],
			"nodes": [{"html": "<img>", "target": ["img"], "impact": "critical"}]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "results.sarif")

	opts := SARIFExportOptions{SaveTo: savePath}
	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	// Verify the file was written
	data, err := os.ReadFile(savePath) // nosemgrep: go_filesystem_rule-fileread
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Saved file is empty")
	}

	// Verify it's valid JSON
	var parsed SARIFLog
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Saved file is not valid SARIF JSON: %v", err)
	}

	// The returned log should still be valid
	if log.Version != "2.1.0" {
		t.Errorf("Expected version '2.1.0', got %q", log.Version)
	}
}

func TestExportSARIF_InvalidPath(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	opts := SARIFExportOptions{SaveTo: "/nonexistent/path/results.sarif"}
	_, err := ExportSARIF(a11yResult, opts)
	if err == nil {
		t.Error("Expected error for invalid path, got nil")
	}
}

func TestExportSARIF_PathTraversal(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	// Path traversal attempts should be rejected
	badPaths := []string{
		"../../etc/passwd",
		"/etc/passwd",
	}

	for _, path := range badPaths {
		opts := SARIFExportOptions{SaveTo: path}
		_, err := ExportSARIF(a11yResult, opts)
		// Either error or the path should be rejected
		// We allow /tmp and cwd, reject everything else
		if err == nil && !strings.HasPrefix(path, "/tmp") {
			// Check if it's under cwd
			cwd, _ := os.Getwd()
			absPath, _ := filepath.Abs(path)
			if !strings.HasPrefix(absPath, cwd) {
				t.Errorf("Expected error for path traversal %q, got nil", path)
			}
		}
	}
}

// ============================================
// Coverage Gap Tests
// ============================================

func TestSaveSARIFToFile_ValidAbsPath(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "image-alt",
			"impact": "critical",
			"description": "Images must have alternate text",
			"help": "Images must have alternate text",
			"helpUrl": "https://dequeuniversity.com/rules/axe/4.10/image-alt",
			"tags": ["wcag2a", "wcag111"],
			"nodes": [{
				"html": "<img src=\"photo.jpg\">",
				"target": ["img"],
				"impact": "critical"
			}]
		}],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "subdir", "results.sarif.json")

	opts := SARIFExportOptions{
		SaveTo: savePath,
	}

	log, err := ExportSARIF(a11yResult, opts)
	if err != nil {
		t.Fatalf("ExportSARIF with save_to failed: %v", err)
	}
	if log == nil {
		t.Fatal("Expected non-nil SARIF log")
	}

	// Verify the file was written
	data, err := os.ReadFile(savePath) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("Failed to read saved SARIF file: %v", err)
	}

	var savedLog SARIFLog
	if err := json.Unmarshal(data, &savedLog); err != nil {
		t.Fatalf("Saved file is not valid SARIF JSON: %v", err)
	}
	if savedLog.Version != "2.1.0" {
		t.Errorf("Expected version 2.1.0, got %q", savedLog.Version)
	}
	if len(savedLog.Runs[0].Results) != 1 {
		t.Errorf("Expected 1 result in saved file, got %d", len(savedLog.Runs[0].Results))
	}
}

func TestSaveSARIFToFile_MkdirAllFailure(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [],
		"passes": [],
		"incomplete": [],
		"inapplicable": []
	}`)

	// Use a path under /tmp but with an invalid parent that can't be created
	// (a file pretending to be a directory)
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("I am a file"), 0600); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}
	// Try to create a file inside the blocking file (which is not a directory)
	savePath := filepath.Join(blockingFile, "subdir", "result.sarif.json")

	opts := SARIFExportOptions{
		SaveTo: savePath,
	}

	_, err := ExportSARIF(a11yResult, opts)
	if err == nil {
		t.Fatal("Expected error when MkdirAll fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create directory") {
		t.Errorf("Expected 'failed to create directory' error, got: %v", err)
	}
}

func TestEnsureRule_DedupPath(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "color-contrast",
			"impact": "serious",
			"description": "Color contrast check",
			"help": "Elements must meet contrast ratio",
			"helpUrl": "https://example.com/color-contrast",
			"tags": ["wcag2aa"],
			"nodes": [
				{"html": "<span class=\"a\">A</span>", "target": ["span.a"], "impact": "serious"},
				{"html": "<span class=\"b\">B</span>", "target": ["span.b"], "impact": "serious"}
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

	// Two nodes under the same violation should produce 2 results but only 1 rule
	run := log.Runs[0]
	if len(run.Tool.Driver.Rules) != 1 {
		t.Errorf("Expected 1 rule (deduped), got %d", len(run.Tool.Driver.Rules))
	}
	if len(run.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(run.Results))
	}
	// Both results should reference rule index 0
	for i, r := range run.Results {
		if r.RuleIndex != 0 {
			t.Errorf("Result[%d] ruleIndex expected 0, got %d", i, r.RuleIndex)
		}
	}
}

// ============================================
// Coverage: ExportSARIF with IncludePasses (line 199 - ensureRule for passes)
// ============================================

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

// ============================================
// Coverage: ExportSARIF with SaveTo option (line 275/286/301/305)
// ============================================

func TestExportSARIF_SaveToTempDir(t *testing.T) {
	t.Parallel()
	a11yResult := json.RawMessage(`{
		"violations": [{
			"id": "label",
			"impact": "critical",
			"description": "Form elements must have labels",
			"help": "Add a label",
			"helpUrl": "https://example.com",
			"tags": ["wcag2a"],
			"nodes": [{"html": "<input>", "target": ["input"], "impact": "critical"}]
		}],
		"passes": [],
		"incomplete": []
	}`)

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "subdir", "output.sarif.json")

	log, err := ExportSARIF(a11yResult, SARIFExportOptions{SaveTo: outPath})
	if err != nil {
		t.Fatalf("ExportSARIF with SaveTo failed: %v", err)
	}
	if log == nil {
		t.Fatal("Expected non-nil log")
	}

	// Verify the file was written
	data, err := os.ReadFile(outPath) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("Failed to read saved SARIF file: %v", err)
	}
	if !strings.Contains(string(data), "label") {
		t.Error("Expected saved file to contain rule ID 'label'")
	}
}

// ============================================
// Coverage: saveSARIFToFile with unwritable directory (line 305)
// ============================================

func TestSaveSARIFToFile_UnwritableDir(t *testing.T) {
	t.Parallel()
	log := &SARIFLog{
		Schema:  "https://example.com/schema",
		Version: "2.1.0",
		Runs:    []SARIFRun{},
	}

	// Create a temp dir, then create a subdir that's not writable
	tmpDir := t.TempDir()
	readonlyDir := filepath.Join(tmpDir, "readonly")
	os.MkdirAll(readonlyDir, 0o555)
	defer os.Chmod(readonlyDir, 0o755)

	outPath := filepath.Join(readonlyDir, "subdir", "cannot-write.sarif")

	err := saveSARIFToFile(log, outPath)
	if err == nil {
		t.Error("Expected error when writing to unwritable directory")
	}
}

// ============================================
// Coverage: ensureRule deduplication (line 199)
// ============================================

// ============================================
// Coverage: resolveExistingPath
// ============================================

func TestResolveExistingPath_ExistingPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	resolved := resolveExistingPath(tmpDir)
	// Should resolve to the real path (EvalSymlinks on existing dir)
	expected, _ := filepath.EvalSymlinks(tmpDir)
	if resolved != expected {
		t.Errorf("resolveExistingPath(%q) = %q, want %q", tmpDir, resolved, expected)
	}
}

func TestResolveExistingPath_NonExistentFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "nonexistent", "file.sarif")
	resolved := resolveExistingPath(target)
	// Parent doesn't exist, grandparent (tmpDir) does.
	// Should resolve tmpDir's real path + "nonexistent/file.sarif"
	resolvedTmp, _ := filepath.EvalSymlinks(tmpDir)
	expected := filepath.Join(resolvedTmp, "nonexistent", "file.sarif")
	if resolved != expected {
		t.Errorf("resolveExistingPath(%q) = %q, want %q", target, resolved, expected)
	}
}

func TestResolveExistingPath_SymlinkInPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	targetDir := t.TempDir()

	// Create a symlink inside tmpDir pointing to targetDir
	symlinkPath := filepath.Join(tmpDir, "link")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Skipf("Cannot create symlinks: %v", err)
	}

	// Resolve a file path through the symlink
	filePath := filepath.Join(symlinkPath, "output.sarif")
	resolved := resolveExistingPath(filePath)

	// The resolved path should be under targetDir, not under tmpDir
	resolvedTarget, _ := filepath.EvalSymlinks(targetDir)
	expected := filepath.Join(resolvedTarget, "output.sarif")
	if resolved != expected {
		t.Errorf("resolveExistingPath(%q) = %q, want %q (should follow symlink)", filePath, resolved, expected)
	}
}

func TestSaveSARIFToFile_SymlinkResolution(t *testing.T) {
	t.Parallel()
	// Verify that saveSARIFToFile resolves symlinks before checking allowed paths.
	// On macOS, t.TempDir() dirs are all under the OS temp dir, so symlinks
	// between temp dirs are legitimately allowed by the temp dir check.
	// This test verifies that symlink resolution works correctly by checking
	// that the file is written to the RESOLVED target (not the symlink path).
	cwdDir := t.TempDir()
	targetDir := t.TempDir()

	symlinkPath := filepath.Join(cwdDir, "link")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Skipf("Cannot create symlinks: %v", err)
	}

	// Verify resolveExistingPath follows the symlink correctly
	filePath := filepath.Join(symlinkPath, "result.sarif")
	resolvedFile := resolveExistingPath(filePath)

	resolvedTarget, _ := filepath.EvalSymlinks(targetDir)
	expected := filepath.Join(resolvedTarget, "result.sarif")
	if resolvedFile != expected {
		t.Errorf("resolveExistingPath through symlink: got %q, want %q", resolvedFile, expected)
	}

	// The resolved path is under the OS temp dir, so the write should succeed.
	// This validates that the resolution + allowed-dir check work together.
	log := &SARIFLog{Version: "2.1.0", Schema: "test", Runs: []SARIFRun{}}
	err := saveSARIFToFile(log, filePath)
	if err != nil {
		t.Fatalf("saveSARIFToFile through symlink under temp should succeed: %v", err)
	}

	// Verify the file was written to the resolved target
	resolvedTargetFile := filepath.Join(resolvedTarget, "result.sarif")
	if _, err := os.Stat(resolvedTargetFile); os.IsNotExist(err) {
		t.Error("Expected file to be written at resolved symlink target")
	}
}

func TestSaveSARIFToFile_OutsideAllowedDirs(t *testing.T) {
	t.Parallel()
	// Test that paths outside both cwd and temp dir are rejected.
	log := &SARIFLog{Version: "2.1.0", Schema: "test", Runs: []SARIFRun{}}
	err := saveSARIFToFile(log, "/nonexistent/path/evil.sarif")
	if err == nil {
		t.Error("Expected error for path outside allowed directories")
	}
	if err != nil && !strings.Contains(err.Error(), "save_to path must be under") {
		t.Errorf("Expected 'save_to path must be under' error, got: %v", err)
	}
}

func TestEnsureRule_Deduplication(t *testing.T) {
	t.Parallel()
	run := &SARIFRun{
		Tool: SARIFTool{
			Driver: SARIFDriver{
				Rules: []SARIFRule{},
			},
		},
		Results: []SARIFResult{},
	}
	indices := make(map[string]int)

	v1 := axeViolation{ID: "rule-1", Description: "Desc 1", Help: "Help 1"}
	v2 := axeViolation{ID: "rule-2", Description: "Desc 2", Help: "Help 2"}

	idx1 := ensureRule(run, indices, v1)
	idx2 := ensureRule(run, indices, v2)
	idx1Again := ensureRule(run, indices, v1) // should return existing

	if idx1 != 0 {
		t.Errorf("Expected first rule index 0, got %d", idx1)
	}
	if idx2 != 1 {
		t.Errorf("Expected second rule index 1, got %d", idx2)
	}
	if idx1Again != 0 {
		t.Errorf("Expected duplicate to return 0, got %d", idx1Again)
	}
	if len(run.Tool.Driver.Rules) != 2 {
		t.Errorf("Expected 2 rules total, got %d", len(run.Tool.Driver.Rules))
	}
}
