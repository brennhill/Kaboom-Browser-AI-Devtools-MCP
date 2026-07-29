// eval_test.go — Tier 1 unit eval runner for all hooks.

package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findRepoRoot walks up from dir looking for go.mod.
func findRepoRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func TestEval_AllFixtures(t *testing.T) {
	testdataDir := filepath.Join("testdata")

	fixtures, err := LoadFixtures(testdataDir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixtures found")
	}

	absTestdata, _ := filepath.Abs(testdataDir)
	repoRoot := findRepoRoot(absTestdata)
	if repoRoot == "" {
		t.Fatal("cannot find repo root (go.mod)")
	}

	for _, fix := range fixtures {
		fix := fix
		t.Run(fix.Hook+"/"+fix.Description, func(t *testing.T) {
			if strings.Contains(fix.Description, "ASPIRATIONAL") || strings.Contains(fix.FixturePath, "ASPIRATIONAL") {
				t.Skip("aspirational fixture — not yet implemented")
			}
			t.Parallel()
			result := RunFixture(fix, repoRoot)
			if !result.Passed {
				for _, f := range result.Failures {
					t.Error(f)
				}
				if result.Output != "" {
					t.Logf("Output: %s", truncate(result.Output, 500))
				}
			}
			t.Logf("Latency: %dms", result.LatencyMs)
		})
	}
}

func TestEval_Report(t *testing.T) {
	testdataDir := filepath.Join("testdata")

	fixtures, err := LoadFixtures(testdataDir)
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}

	absTestdata, _ := filepath.Abs(testdataDir)
	repoRoot := findRepoRoot(absTestdata)
	if repoRoot == "" {
		t.Fatal("cannot find repo root (go.mod)")
	}

	var results []*Result
	for _, fix := range fixtures {
		if strings.Contains(fix.Description, "ASPIRATIONAL") || strings.Contains(fix.FixturePath, "ASPIRATIONAL") {
			continue
		}
		results = append(results, RunFixture(fix, repoRoot))
	}

	report := Aggregate(results)
	t.Log("\n" + FormatReport(report))

	if report.Failed > 0 {
		t.Errorf("%d/%d fixtures failed", report.Failed, report.Total)
	}
}

func TestMaterializeOversizedFixtureRemainsValidGoWhileVisible(t *testing.T) {
	root := t.TempDir()
	raw := json.RawMessage(`{"file_path":"EVAL_OVERSIZED_FILE"}`)

	materialized, cleanup := materializeFixtureFile(raw, root)
	if cleanup == nil {
		t.Fatal("materializeFixtureFile did not create the oversized fixture")
	}
	defer cleanup()

	var fields struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(materialized, &fields); err != nil {
		t.Fatalf("decode materialized fixture: %v", err)
	}
	content, err := os.ReadFile(fields.FilePath)
	if err != nil {
		t.Fatalf("read materialized fixture: %v", err)
	}
	if !strings.HasPrefix(string(content), "package evalfixture\n") {
		t.Fatalf("temporary Go fixture is not valid source: %q", content[:min(len(content), 40)])
	}
	if lines := strings.Count(string(content), "\n"); lines <= 800 {
		t.Fatalf("temporary fixture has %d lines, want more than 800", lines)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	if got := truncate("short", 5); got != "short" {
		t.Fatalf("truncate exact string = %q, want short", got)
	}
	if got := truncate("longer", 4); got != "long..." {
		t.Fatalf("truncate long string = %q, want long...", got)
	}
}
