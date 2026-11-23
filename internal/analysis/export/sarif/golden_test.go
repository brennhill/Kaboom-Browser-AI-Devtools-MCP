// Purpose: Golden-file snapshot tests for HAR/SARIF export output stability.
// Docs: docs/features/feature/har-export/index.md

// export_sarif_golden_test.go — Golden file validation for SARIF export output.
package sarif

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

var normalizeVersionRe = regexp.MustCompile(`"version"\s*:\s*"[^"]*"`)

func normalizeExportVersion(data []byte) []byte {
	return normalizeVersionRe.ReplaceAll(data, []byte(`"version": "VERSION"`))
}

func TestGoldenSARIFViolations(t *testing.T) {
	axeJSON := json.RawMessage(`{
		"violations": [
			{
				"id": "color-contrast",
				"impact": "serious",
				"description": "Elements must have sufficient color contrast",
				"help": "Elements must have sufficient color contrast",
				"helpUrl": "https://dequeuniversity.com/rules/axe/4.7/color-contrast",
				"tags": ["wcag2aa", "wcag143"],
				"nodes": [
					{
						"html": "<span class=\"low-contrast\">Hello</span>",
						"target": ["span.low-contrast"],
						"failureSummary": "Fix any of the following: Element has insufficient color contrast of 2.5:1"
					}
				]
			},
			{
				"id": "image-alt",
				"impact": "critical",
				"description": "Images must have alternate text",
				"help": "Images must have alternate text",
				"helpUrl": "https://dequeuniversity.com/rules/axe/4.7/image-alt",
				"tags": ["wcag2a", "wcag111"],
				"nodes": [
					{
						"html": "<img src=\"photo.jpg\">",
						"target": ["img"],
						"failureSummary": "Fix any of the following: Element does not have an alt attribute"
					}
				]
			}
		],
		"passes": [
			{
				"id": "html-has-lang",
				"impact": "serious",
				"description": "html element must have a lang attribute",
				"help": "html element must have a lang attribute",
				"helpUrl": "https://dequeuniversity.com/rules/axe/4.7/html-has-lang",
				"tags": ["wcag2a", "wcag311"],
				"nodes": [
					{
						"html": "<html lang=\"en\">",
						"target": ["html"],
						"failureSummary": ""
					}
				]
			}
		]
	}`)

	sarif, err := ExportSARIF(axeJSON, SARIFExportOptions{IncludePasses: false})
	if err != nil {
		t.Fatalf("ExportSARIF failed: %v", err)
	}

	data, err := json.MarshalIndent(sarif, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}
	data = append(data, '\n')
	normalizedData := normalizeExportVersion(data)

	goldenPath := "testdata/sarif-violations.golden.json"

	if updateGolden {
		err = os.WriteFile(goldenPath, normalizedData, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		t.Logf("Updated golden file (%d bytes)", len(normalizedData))
	} else {
		goldenData, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("Failed to read golden file (run with UPDATE_GOLDEN=1 first): %v", err)
		}
		normalizedGolden := normalizeExportVersion(goldenData)

		if !bytes.Equal(normalizedData, normalizedGolden) {
			t.Errorf("Golden file mismatch for %s", goldenPath)
			t.Errorf("Expected %d bytes, got %d bytes", len(normalizedGolden), len(normalizedData))
			t.Fatalf("Run with UPDATE_GOLDEN=1 to update golden files")
		}
		t.Logf("SARIF golden file validation passed (%d bytes)", len(normalizedData))
	}
}
