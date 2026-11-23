// export_sarif_test.go — Tests accessibility-result conversion into SARIF.
// Docs: docs/features/feature/sarif-export/index.md

package sarif

import (
	"encoding/json"
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

func TestEnsureRule_Deduplication(t *testing.T) {
	t.Parallel()
	run := &sarifRun{
		Tool: sarifTool{
			Driver: sarifDriver{
				Rules: []sarifRule{},
			},
		},
		Results: []sarifResult{},
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
