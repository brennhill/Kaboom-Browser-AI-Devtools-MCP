// eval_test.go — Tier 1 unit eval runner for all hooks.

package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/hook"
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
			result := runContractFixture(fix, repoRoot)
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
		results = append(results, runContractFixture(fix, repoRoot))
	}

	report := Aggregate(results)
	t.Log("\n" + FormatReport(report))

	if report.Failed > 0 {
		t.Errorf("%d/%d fixtures failed", report.Failed, report.Total)
	}
}

func TestEval_ProductionLatency(t *testing.T) {
	fixtures, err := LoadFixtures("testdata")
	if err != nil {
		t.Fatalf("LoadFixtures: %v", err)
	}
	repoRoot := findRepoRoot(mustAbs(t, "testdata"))
	if repoRoot == "" {
		t.Fatal("cannot find repo root (go.mod)")
	}

	for _, fix := range fixtures {
		if fix.Expect.MaxLatencyMs == 0 || strings.Contains(fix.Description, "ASPIRATIONAL") || strings.Contains(fix.FixturePath, "ASPIRATIONAL") {
			continue
		}
		result := runPerformanceFixture(fix, repoRoot)
		for _, failure := range result.Failures {
			if strings.HasPrefix(failure, "latency ") {
				t.Errorf("%s/%s: %s", fix.Hook, fix.Description, failure)
			}
		}
	}
}

func TestEval_ContractIgnoresSchedulerDelay(t *testing.T) {
	fix := &Fixture{Expect: Expectation{MaxLatencyMs: 1}}
	result := runFixture(fix, "", contractEvaluation, func(string, hook.Input, string, string) string {
		time.Sleep(5 * time.Millisecond)
		return ""
	})
	if !result.Passed {
		t.Fatalf("contract evaluation failed on scheduler delay: %v", result.Failures)
	}
}

func TestEval_PerformanceEnforcesLatencyBudget(t *testing.T) {
	if raceDetectorActive {
		t.Skip("race instrumentation intentionally disables wall-clock SLO assertions")
	}
	fix := &Fixture{Expect: Expectation{MaxLatencyMs: 1}}
	result := runFixture(fix, "", performanceEvaluation, func(string, hook.Input, string, string) string {
		time.Sleep(5 * time.Millisecond)
		return ""
	})
	if result.Passed || len(result.Failures) != 1 || !strings.HasPrefix(result.Failures[0], "latency ") {
		t.Fatalf("performance evaluation failures = %v, want latency failure", result.Failures)
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("absolute path: %v", err)
	}
	return abs
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

func TestValidateReportsEveryContractFailure(t *testing.T) {
	t.Parallel()
	failures := validate(Expectation{
		HasOutput: false, Contains: []string{"required"}, NotContains: []string{"secret"},
		MaxTokens: 1, MaxLatencyMs: 1,
	}, "secret and too long", 10*time.Millisecond, true)
	wantFailures := 5
	if raceDetectorActive {
		wantFailures-- // Latency contracts are intentionally disabled under race instrumentation.
	}
	if len(failures) != wantFailures {
		t.Fatalf("validate failures = %#v", failures)
	}
	if got := validate(Expectation{HasOutput: true}, "", 0, false); len(got) != 1 || !strings.Contains(got[0], "expected output") {
		t.Fatalf("missing-output failures = %#v", got)
	}
}

func TestResolveToolInputPathsHandlesMalformedMissingAndAbsolutePaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, raw := range []json.RawMessage{json.RawMessage(`not-json`), json.RawMessage(`{"other":true}`), json.RawMessage(`{"file_path":7}`)} {
		if got := resolveToolInputPaths(raw, root); string(got) != string(raw) {
			t.Fatalf("resolveToolInputPaths(%s) = %s", raw, got)
		}
	}
	absolute := filepath.Join(root, "already.go")
	raw, _ := json.Marshal(map[string]string{"file_path": absolute})
	if got := resolveToolInputPaths(raw, root); !strings.Contains(string(got), absolute) {
		t.Fatalf("absolute path changed: %s", got)
	}
	relative := json.RawMessage(`{"file_path":"nested/file.go"}`)
	if got := resolveToolInputPaths(relative, root); !strings.Contains(string(got), filepath.Join(root, "nested", "file.go")) {
		t.Fatalf("relative path was not resolved: %s", got)
	}
}

func TestRunFixtureUsesExplicitRootAndUnknownHookIsSilent(t *testing.T) {
	t.Parallel()
	fix := &Fixture{
		Hook: "unknown", ProjectRoot: "/explicit/project",
		Input:  FixtureInput{ToolName: "Read", ToolInput: json.RawMessage(`{}`)},
		Expect: Expectation{HasOutput: false},
	}
	result := runFixture(fix, "/repository", contractEvaluation, func(name string, _ hook.Input, root, _ string) string {
		if name != "unknown" || root != "/explicit/project" {
			t.Fatalf("runner inputs = %q, %q", name, root)
		}
		return runHook(name, hook.Input{}, root, "")
	})
	if !result.Passed || result.Output != "" {
		t.Fatalf("unknown hook result = %#v", result)
	}
}

func TestLoadFixturesHandlesMissingSkippedMalformedAndUnreadableEntries(t *testing.T) {
	original := fixtureDirs
	t.Cleanup(func() { fixtureDirs = original })
	root := t.TempDir()

	fixtureDirs = []string{"missing"}
	fixtures, err := LoadFixtures(root)
	if err != nil || len(fixtures) != 0 {
		t.Fatalf("missing fixture directory = %#v, %v", fixtures, err)
	}

	fixtureDirs = []string{"blocked"}
	if err := os.WriteFile(filepath.Join(root, "blocked"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFixtures(root); err == nil {
		t.Fatal("non-directory fixture root was accepted")
	}

	fixtureDirs = []string{"cases"}
	cases := filepath.Join(root, "cases")
	if err := os.Mkdir(cases, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.Mkdir(filepath.Join(cases, "nested.json"), 0o700)
	_ = os.WriteFile(filepath.Join(cases, "notes.txt"), []byte("ignored"), 0o600)
	_ = os.WriteFile(filepath.Join(cases, "broken.json"), []byte(`not-json`), 0o600)
	if _, err := LoadFixtures(root); err == nil || !strings.Contains(err.Error(), "parse fixture") {
		t.Fatalf("malformed fixture error = %v", err)
	}

	if err := os.Remove(filepath.Join(cases, "broken.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cases, "absent"), filepath.Join(cases, "unreadable.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFixtures(root); err == nil || !strings.Contains(err.Error(), "read fixture") {
		t.Fatalf("unreadable fixture error = %v", err)
	}
}

func TestAggregateCountsFailures(t *testing.T) {
	t.Parallel()
	report := Aggregate([]*Result{{Fixture: &Fixture{Hook: "quality-gate"}, Passed: false, LatencyMs: 3}})
	if report.Total != 1 || report.Failed != 1 || report.Passed != 0 {
		t.Fatalf("aggregate = %#v", report)
	}
}

func TestEvalFilesystemFailuresFallBackSafely(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"file_path":"EVAL_OVERSIZED_FILE"}`)
	if got, cleanup := materializeFixtureFile(raw, blocked); cleanup != nil || string(got) != string(raw) {
		t.Fatalf("materialize blocked path = %s, cleanup=%v", got, cleanup != nil)
	}
	t.Setenv("TMPDIR", blocked)
	if output := runHook("session-track", hook.Input{ToolName: "Read", ToolInput: json.RawMessage(`{}`)}, "", ""); output != "" {
		t.Fatalf("session-track tempdir failure output = %q", output)
	}
}
