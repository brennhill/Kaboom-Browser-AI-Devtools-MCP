// discovery_test.go — Tests project-wide convention discovery, its cache, and the
// summary the quality gate injects.
//
// Split from conventions_test.go because the two halves fail for different
// reasons: detection answers "does this edit match something the project already
// does", discovery answers "what does the project do at all", and the second one
// owns the persisted cache and the ranking cut.

package conventions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// writeFile creates a file and the directories above it.
func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupDiscoverableProject builds a project whose call sites clear
// discoveryMinFiles, so discovery has something to return and a cache has
// something worth holding.
func setupDiscoverableProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < discoveryMinFiles+1; i++ {
		writeFile(t, root, fmt.Sprintf("store%d.go", i),
			fmt.Sprintf("package main\n\nfunc use%d() {\n\tdb.Query(\"SELECT %d\")\n\tcache.Get(\"k%d\")\n}\n", i, i, i))
	}
	return root
}

// forgetDiscoveredConventions drops the in-process cache so a test can observe
// what a freshly started hook process would see. Each hook is its own
// short-lived process in production, so this is the state that actually matters.
func forgetDiscoveredConventions(t *testing.T) {
	t.Helper()
	discoveryCache.mu.Lock()
	discoveryCache.entries = make(map[string]*discoveryCacheEntry)
	discoveryCache.mu.Unlock()
}

// TestDiscover_SurvivesProcessRestart is the regression guard for the
// cache that never worked. The process-level map is written and then discarded
// when the hook exits, so before conventions were persisted every edit re-walked
// the project. Deleting the sources after the first discovery proves the second
// answer came from the cache and not from a second walk.
func TestDiscover_SurvivesProcessRestart(t *testing.T) {
	root := setupDiscoverableProject(t)

	first := Discover(root, ".go")
	if len(first) == 0 {
		t.Fatal("expected conventions from the initial walk")
	}

	forgetDiscoveredConventions(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Remove(filepath.Join(root, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}

	second := Discover(root, ".go")
	if len(second) != len(first) {
		t.Fatalf("after restart got %d conventions, want the %d cached ones; a walk of the emptied project would find none",
			len(second), len(first))
	}
	for i := range first {
		if second[i] != first[i] {
			t.Errorf("cached convention %d = %+v, want %+v", i, second[i], first[i])
		}
	}
}

// TestDiscover_CacheIsScopedPerLanguage proves one language's cache
// file cannot answer for another, which is why each extension gets its own file.
func TestDiscover_CacheIsScopedPerLanguage(t *testing.T) {
	root := setupDiscoverableProject(t)
	for i := 0; i < discoveryMinFiles+1; i++ {
		writeFile(t, root, fmt.Sprintf("view%d.ts", i),
			fmt.Sprintf("export const v%d = 1;\nrouter.push('/p%d');\nstore.commit('m%d');\n", i, i, i))
	}

	goConventions := Discover(root, ".go")
	tsConventions := Discover(root, ".ts")

	goPath, err := conventionCachePath(root, ".go")
	if err != nil {
		t.Fatal(err)
	}
	tsPath, err := conventionCachePath(root, ".ts")
	if err != nil {
		t.Fatal(err)
	}
	if goPath == tsPath {
		t.Fatalf("both languages share cache file %q", goPath)
	}
	for _, path := range []string{goPath, tsPath} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected a cache file at %s: %v", path, err)
		}
	}

	forgetDiscoveredConventions(t)
	if got := Discover(root, ".go"); len(got) != len(goConventions) {
		t.Errorf("Go cache returned %d conventions, want %d", len(got), len(goConventions))
	}
	forgetDiscoveredConventions(t)
	if got := Discover(root, ".ts"); len(got) != len(tsConventions) {
		t.Errorf("TS cache returned %d conventions, want %d", len(got), len(tsConventions))
	}
}

// TestDiscover_RewalksWhenCacheIsStaleOrCorrupt keeps a bad cache
// from becoming a wrong answer: both an expired entry and an unreadable file
// must fall back to walking the project.
func TestDiscover_RewalksWhenCacheIsStaleOrCorrupt(t *testing.T) {
	root := setupDiscoverableProject(t)
	fresh := Discover(root, ".go")
	if len(fresh) == 0 {
		t.Fatal("expected conventions from the initial walk")
	}
	path, err := conventionCachePath(root, ".go")
	if err != nil {
		t.Fatal(err)
	}

	expired, err := json.Marshal(persistedConventions{
		Conventions: []Discovered{{Pattern: "stale.Call(", FileCount: 99}},
		BuiltAt:     time.Now().Add(-2 * discoveryCacheTTL),
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string][]byte{"expired": expired, "corrupt": []byte("{not json")} {
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		forgetDiscoveredConventions(t)
		got := Discover(root, ".go")
		if len(got) != len(fresh) {
			t.Errorf("%s cache: got %d conventions, want the %d a real walk finds", name, len(got), len(fresh))
		}
		for _, c := range got {
			if c.Pattern == "stale.Call(" {
				t.Errorf("%s cache: served the stale entry instead of re-walking", name)
			}
		}
	}
}

// TestSearchProject_OneWalkServesEveryTerm covers the contract that lets probe
// search read the project once instead of once per probe: each term is capped
// independently at maxExamplesPerProbe, a file contributes at most one example
// per term, the edited file is excluded, and a repeated term is not counted twice.
func TestSearchProject_OneWalkServesEveryTerm(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Every file carries both patterns twice, so per-file and per-term caps are
	// both exercised at once.
	const body = "package p\n\nfunc a() { alpha.Do() }\nfunc b() { beta.Do() }\nfunc c() { alpha.Do() }\n"
	for i := 0; i < maxExamplesPerProbe+3; i++ {
		writeFile(t, root, fmt.Sprintf("file%d.go", i), body)
	}
	edited := filepath.Join(root, "file0.go")

	got := searchProject(root, []string{"alpha.Do(", "beta.Do(", "alpha.Do("}, edited, []string{".go"})

	for _, term := range []string{"alpha.Do(", "beta.Do("} {
		if len(got[term]) != maxExamplesPerProbe {
			t.Errorf("%q got %d examples, want %d", term, len(got[term]), maxExamplesPerProbe)
		}
		for _, example := range got[term] {
			if strings.Contains(example, "file0.go") {
				t.Errorf("%q reported the edited file: %s", term, example)
			}
		}
	}
	// A duplicated term is one term: the file that supplies an example supplies
	// exactly one, so the count matches the non-duplicated term above.
	if len(got) != 2 {
		t.Errorf("searchProject returned %d distinct terms, want 2", len(got))
	}
}

func TestDiscover_GoProject(t *testing.T) {
	t.Parallel()

	// Find the repo root (the real kaboom codebase).
	root := findRepoRoot(t)

	conventions := Discover(root, ".go")
	if len(conventions) == 0 {
		t.Fatal("expected discovered conventions for Go codebase, got none")
	}

	// Sanity: should find patterns we know exist in kaboom.
	found := make(map[string]bool)
	for _, c := range conventions {
		found[c.Pattern] = true
	}

	// These are real kaboom patterns that appear in many files.
	wantSome := []string{
		"json.Unmarshal(",
		"json.Marshal(",
	}
	for _, w := range wantSome {
		if !found[w] {
			t.Errorf("expected to discover %q — it's a real pattern in this codebase", w)
		}
	}

	// Noise should be filtered out.
	noisePatterns := []string{
		"t.Fatalf(",
		"t.Errorf(",
		"strings.Contains(",
		"fmt.Sprintf(",
		"filepath.Join(",
		"mu.Lock(",
	}
	for _, n := range noisePatterns {
		if found[n] {
			t.Errorf("noise pattern %q should be filtered, but was discovered", n)
		}
	}
}

func TestDiscover_TSProject(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)

	conventions := Discover(root, ".ts")
	if len(conventions) == 0 {
		t.Fatal("expected discovered conventions for TS files, got none")
	}

	// Noise should be filtered.
	found := make(map[string]bool)
	for _, c := range conventions {
		found[c.Pattern] = true
	}
	tsNoise := []string{"Date.now(", "Math.min(", "JSON.stringify(", "console.log("}
	for _, n := range tsNoise {
		if found[n] {
			t.Errorf("noise pattern %q should be filtered, but was discovered", n)
		}
	}
}

func TestDiscover_EmptyDir(t *testing.T) {
	t.Parallel()

	empty := t.TempDir()
	conventions := Discover(empty, ".go")
	if len(conventions) != 0 {
		t.Errorf("expected no conventions in empty dir, got %d", len(conventions))
	}
}

func TestDiscover_SmallProject(t *testing.T) {
	t.Parallel()

	// Build a minimal project where `db.Query(` appears in 3 files.
	root := t.TempDir()
	files := map[string]string{
		"a.go": "package main\nfunc a() { db.Query(\"SELECT 1\") }\n",
		"b.go": "package main\nfunc b() { db.Query(\"SELECT 2\") }\n",
		"c.go": "package main\nfunc c() { db.Query(\"SELECT 3\") }\n",
		"d.go": "package main\nfunc d() { db.Query(\"SELECT 4\"); db.Exec(\"INSERT\") }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Clear cache so we don't get stale results.
	discoveryCache.mu.Lock()
	delete(discoveryCache.entries, root+"\x00.go")
	discoveryCache.mu.Unlock()

	conventions := Discover(root, ".go")

	found := make(map[string]bool)
	for _, c := range conventions {
		found[c.Pattern] = true
	}

	if !found["db.Query("] {
		t.Error("expected to discover db.Query( in 4 files")
	}
	// db.Exec( only appears in 1 file — below threshold.
	if found["db.Exec("] {
		t.Error("db.Exec( appears in 1 file — should be below threshold")
	}
}

func TestProbes_ReturnStrings(t *testing.T) {
	t.Parallel()

	root := findRepoRoot(t)
	probes := Probes(root, ".go")
	if len(probes) == 0 {
		t.Fatal("expected probes, got none")
	}

	// Every probe should end with (.
	for _, p := range probes {
		if !strings.HasSuffix(p, "(") {
			t.Errorf("probe %q should end with (", p)
		}
	}
}

func TestNoiseFiltering_Comprehensive(t *testing.T) {
	t.Parallel()

	// Verify all noise entries match the regex they're supposed to filter.
	for pattern := range goNoise {
		if !goCallSite.MatchString(pattern) {
			t.Errorf("Go noise pattern %q doesn't match goCallSite regex", pattern)
		}
	}
	for pattern := range tsNoise {
		if !tsCallSite.MatchString(pattern) {
			t.Errorf("TS noise pattern %q doesn't match tsCallSite regex", pattern)
		}
	}
}

// findRepoRoot walks up from the test file to find go.mod.
func TestPersistConventionsRoundTripAndWriteFailure(t *testing.T) {
	root := t.TempDir()
	want := []Discovered{{Pattern: "svc.Query(", FileCount: 4}}

	persistConventions(root, ".go", want)
	got, ok := loadPersistedConventions(root, ".go")
	if !ok || len(got) != 1 || got[0] != want[0] {
		t.Fatalf("persisted conventions = (%#v, %v), want the cached round trip", got, ok)
	}

	// A cache path occupied by a directory makes the write fail; persistence
	// must report and return rather than panic or block the edit.
	path, err := conventionCachePath(root, ".ts")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	persistConventions(root, ".ts", want)
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("blocked cache path disturbed: %v", statErr)
	}

	// A FILE named like the cache directory makes MkdirAll fail.
	blockedRoot := t.TempDir()
	projectDir, err := state.ProjectDir(blockedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	blockedDir := filepath.Join(projectDir, "hook-conventions")
	if err := os.WriteFile(blockedDir, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	persistConventions(blockedRoot, ".go", want)
	if entry, statErr := os.Stat(blockedDir); statErr != nil || entry.IsDir() {
		t.Fatalf("blocked cache entry disturbed: (%v, %v)", entry, statErr)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}

// tiedAtTheCut builds a ranked list where the 10th and 11th entries appear in
// the same number of files, which is the shape that made the shipped summary
// flicker: `os.WriteFile(` and `context.Background(` were both in 32 files, one
// on each side of rank 10, and one file added anywhere in the repository swapped
// which of them a reviewer was told the project uses.
func tiedAtTheCut() []Discovered {
	conventions := make([]Discovered, 0, 13)
	for i := 0; i < 9; i++ {
		conventions = append(conventions, Discovered{
			Pattern:   fmt.Sprintf("clear%02d(", i),
			FileCount: 100 - i,
		})
	}
	for _, name := range []string{"tiedA(", "tiedB(", "tiedC("} {
		conventions = append(conventions, Discovered{Pattern: name, FileCount: 32})
	}
	conventions = append(conventions, Discovered{Pattern: "below(", FileCount: 31})
	return conventions
}

func TestSummaryCutNeverSplitsATieGroup(t *testing.T) {
	t.Parallel()
	conventions := tiedAtTheCut()

	cut := summaryCut(conventions)
	if cut != 12 {
		t.Fatalf("cut = %d, want 12: the three patterns tied at 32 files must be reported together or not at all", cut)
	}
	// The pattern below the tie stays out — the cut extends over the tie, it does
	// not simply report everything.
	if conventions[cut-1].FileCount != 32 {
		t.Errorf("the cut reached a pattern in %d files; it must stop at the end of the tie group",
			conventions[cut-1].FileCount)
	}

	// Discriminating control: the naive cut this replaced lands inside the group,
	// which is what made a one-file change swap the reported conventions. Without
	// this arm the assertion above would hold for a cut that reported everything.
	if maxSummaryConventions >= len(conventions) {
		t.Fatalf("control: the fixture must be longer than the %d-item target for the cut to matter", maxSummaryConventions)
	}
	if conventions[maxSummaryConventions-1].FileCount != conventions[maxSummaryConventions].FileCount {
		t.Fatal("control: the fixture no longer ties across the cut, so it cannot prove the tie is kept whole")
	}
}

func TestSummaryCutStopsAtTheProbeCeiling(t *testing.T) {
	t.Parallel()
	// A pathological tie — every pattern in the same number of files — must not
	// grow the summary without limit. discoveryMaxProbes is the ceiling.
	conventions := make([]Discovered, 40)
	for i := range conventions {
		conventions[i] = Discovered{Pattern: fmt.Sprintf("p%02d(", i), FileCount: 7}
	}
	if cut := summaryCut(conventions); cut != discoveryMaxProbes {
		t.Errorf("cut = %d, want the %d-probe ceiling", cut, discoveryMaxProbes)
	}
}

func TestSummaryCutReportsEverythingWhenThereIsLittle(t *testing.T) {
	t.Parallel()
	conventions := []Discovered{
		{Pattern: "a(", FileCount: 9},
		{Pattern: "b(", FileCount: 4},
	}
	if cut := summaryCut(conventions); cut != 2 {
		t.Errorf("cut = %d, want 2: a project with two conventions reports both", cut)
	}
}
