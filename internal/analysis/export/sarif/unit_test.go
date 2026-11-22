// Purpose: Unit tests for HAR/SARIF export export sarif logic.
// Docs: docs/features/feature/har-export/index.md

package sarif

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportSARIF_ViolationsAndPasses(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{
		"violations": [
			{
				"id": "color-contrast",
				"impact": "serious",
				"description": "Contrast",
				"help": "Fix contrast",
				"helpUrl": "https://example.com/rules/color-contrast",
				"tags": ["cat.color", "wcag2aa", "wcag143"],
				"nodes": [
					{"html": "<div id='a'>a</div>", "target": ["#a"], "impact": "serious"},
					{"html": "<div id='b'>b</div>", "target": ["#b"], "impact": "minor"}
				]
			}
		],
		"passes": [
			{
				"id": "aria-valid",
				"impact": "minor",
				"description": "ARIA",
				"help": "Looks good",
				"helpUrl": "https://example.com/rules/aria-valid",
				"tags": ["wcag412", "cat.aria"],
				"nodes": [
					{"html": "<button aria-label='ok'>OK</button>", "target": ["button"], "impact": "minor"}
				]
			}
		]
	}`)

	log, err := ExportSARIF(input, SARIFExportOptions{IncludePasses: true})
	if err != nil {
		t.Fatalf("ExportSARIF() error = %v", err)
	}

	if len(log.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(log.Runs))
	}

	run := log.Runs[0]
	if len(run.Tool.Driver.Rules) != 2 {
		t.Fatalf("expected 2 deduped rules, got %d", len(run.Tool.Driver.Rules))
	}
	if len(run.Results) != 3 {
		t.Fatalf("expected 3 results (2 violations + 1 pass), got %d", len(run.Results))
	}

	if run.Results[0].Level != "error" {
		t.Fatalf("expected first node level error, got %q", run.Results[0].Level)
	}
	if run.Results[1].Level != "note" {
		t.Fatalf("expected second node level note, got %q", run.Results[1].Level)
	}
	if run.Results[2].Level != "none" {
		t.Fatalf("expected pass level none, got %q", run.Results[2].Level)
	}

	var wcagTags []string
	for _, rule := range run.Tool.Driver.Rules {
		if rule.ID == "color-contrast" && rule.Properties != nil {
			wcagTags = rule.Properties.Tags
			break
		}
	}
	if len(wcagTags) == 0 {
		t.Fatalf("expected WCAG tags for color-contrast rule")
	}
	for _, tag := range wcagTags {
		if !strings.HasPrefix(tag, "wcag") {
			t.Fatalf("non-WCAG tag leaked into SARIF rule properties: %q", tag)
		}
	}
}

func TestExportSARIF_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ExportSARIF(json.RawMessage("{not-json"), SARIFExportOptions{})
	if err == nil {
		t.Fatal("expected parse error for invalid JSON, got nil")
	}
}

func TestAxeImpactToLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		impact string
		want   string
	}{
		{impact: "critical", want: "error"},
		{impact: "serious", want: "error"},
		{impact: "moderate", want: "warning"},
		{impact: "minor", want: "note"},
		{impact: "unknown", want: "warning"},
	}

	for _, tt := range tests {
		if got := axeImpactToLevel(tt.impact); got != tt.want {
			t.Fatalf("axeImpactToLevel(%q) = %q, want %q", tt.impact, got, tt.want)
		}
	}
}

func TestResolveExistingPath_ResolvesSymlinkedAncestor(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	realDir := filepath.Join(base, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir real dir: %v", err)
	}
	linkDir := filepath.Join(base, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	input := filepath.Join(linkDir, "nested", "report.sarif")
	got := resolveExistingPath(input)
	resolvedReal, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(realDir) error = %v", err)
	}
	want := filepath.Join(resolvedReal, "nested", "report.sarif")
	if got != want {
		t.Fatalf("resolveExistingPath(%q) = %q, want %q", input, got, want)
	}
}

func TestSaveSARIFToFile_PathGuardsAndWrite(t *testing.T) {
	t.Parallel()

	log := &SARIFLog{
		Schema:  sarifSchemaURL,
		Version: sarifSpecVersion,
		Runs:    []sarifRun{},
	}

	// Allowed: under current working directory
	cwdPath := filepath.Join(".tmp-sarif-tests", strings.ReplaceAll(t.Name(), "/", "_"), "audit.sarif")
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Dir(cwdPath))
	})
	if err := saveSARIFToFile(log, cwdPath); err != nil {
		t.Fatalf("save under cwd should succeed: %v", err)
	}
	absCwdPath, _ := filepath.Abs(cwdPath)
	if _, err := os.Stat(absCwdPath); err != nil {
		t.Fatalf("expected SARIF file under cwd to exist: %v", err)
	}

	// Allowed: under temp directory
	tmpPath := filepath.Join(os.TempDir(), "kaboom-test", "audit.sarif")
	if err := saveSARIFToFile(log, tmpPath); err != nil {
		t.Fatalf("save under temp should succeed: %v", err)
	}
	if _, err := os.Stat(tmpPath); err != nil {
		t.Fatalf("expected SARIF file under temp to exist: %v", err)
	}

	// Rejected: sibling of temp directory (outside temp + outside cwd)
	absCwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("filepath.Abs(.) error = %v", err)
	}
	volume := filepath.VolumeName(absCwd)
	outside := filepath.Join(volume+string(os.PathSeparator), "kaboom-outside", "audit.sarif")
	if strings.HasPrefix(outside, absCwd+string(os.PathSeparator)) || strings.HasPrefix(outside, os.TempDir()+string(os.PathSeparator)) {
		t.Fatalf("test path %q unexpectedly falls under cwd/temp", outside)
	}
	err = saveSARIFToFile(log, outside)
	if err == nil {
		t.Fatal("expected save outside cwd/temp to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "must be under the current working directory or temp directory") {
		t.Fatalf("unexpected error for outside path: %v", err)
	}
}
