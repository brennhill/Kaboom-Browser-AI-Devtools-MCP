// main_test.go — Pins cyclomatic complexity counting and gate scope rules.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComplexityCountsBranchNodes(t *testing.T) {
	source := `package pkg

func branches(a, b bool, n int) int {
	total := 1
	if a {
		total++
	}
	if a && b {
		total++
	}
	if a || b {
		total++
	}
	for i := 0; i < n; i++ {
		total++
	}
	for range []int{1} {
		total++
	}
	switch n {
	case 1:
		total++
	case 2, 3:
		total += 2
	default:
	}
	select {
	case <-make(chan int):
		total++
	default:
	}
	return total
}
`
	if got := complexityOf(t, source); got != 13 {
		t.Fatalf("complexity = %d, want 13 (1 base + 3 if + 3 logical + 2 for + 3 case + 1 comm + 0 default)", got)
	}
}

func TestComplexityAttributesNestedFuncLiteralsToEnclosingFunction(t *testing.T) {
	source := `package pkg

func outer(a bool) func() {
	if a {
		return func() {
			if a && a {
				panic("nested")
			}
		}
	}
	return nil
}
`
	if got := complexityOf(t, source); got != 4 {
		t.Fatalf("complexity = %d, want 4 (1 base + 1 if + nested 1 if + nested && counted like gocyclo)", got)
	}
}

func TestScanReportsOnlyFunctionsOverLimitInAuthoredGo(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join("internal", "pkg", "hot.go"), `package pkg

func Hot(v int) string {
	switch v {
	case 1:
		return "a"
	case 2:
		return "b"
	case 3:
		return "c"
	case 4:
		return "d"
	case 5:
		return "e"
	case 6:
		return "f"
	case 7:
		return "g"
	case 8:
		return "h"
	case 9:
		return "i"
	case 10:
		return "j"
	case 11:
		return "k"
	case 12:
		return "l"
	case 13:
		return "m"
	case 14:
		return "n"
	case 15:
		return "o"
	}
	return "z"
}
`)
	write(filepath.Join("internal", "pkg", "cool.go"), "package pkg\n\nfunc Cool() {}\n")
	write(filepath.Join("internal", "pkg", "hot_test.go"), "package pkg\n\nfunc TestHot(t *testing.T) {}\n")
	write(filepath.Join("internal", "pkg", "testdata", "ignored.go"), "package testdata\n")

	got, err := scan(root, 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("violations = %v, want only hot.go Hot", got)
	}
	if got[0].function != "Hot" {
		t.Fatalf("violation function = %q, want Hot", got[0].function)
	}
	if got[0].file != filepath.Join("internal", "pkg", "hot.go") {
		t.Fatalf("violation file = %q, want the authored hot.go path", got[0].file)
	}
	if got[0].line == 0 {
		t.Fatalf("violation line missing: %+v", got[0])
	}
}

func TestIgnoredDirSkipsGeneratedAndVendorTrees(t *testing.T) {
	for _, name := range []string{".git", ".claude", "node_modules", "vendor", "dist", "build", "coverage", "generated", "testdata"} {
		if !ignoredDir(name) {
			t.Errorf("ignoredDir(%q) = false, want it skipped", name)
		}
	}
}

func TestParamsCountsNamedUnnamedAndExcludesReceiver(t *testing.T) {
	source := `package pkg

type Receiver struct{}

func (r *Receiver) Multi(a, b string, c int, _ bool, d []byte, e map[string]int, f func()) error {
	return nil
}

func Unnamed(string, int, bool, []byte, map[string]int, func(), error) {}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "probe.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := scan(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for _, f := range findings {
		byName[f.function] = f.params
	}
	if byName["Multi"] != 7 {
		t.Fatalf("Multi params = %d, want 7 (grouped names counted individually, receiver excluded)", byName["Multi"])
	}
	if byName["Unnamed"] != 7 {
		t.Fatalf("Unnamed params = %d, want 7 (type-only fields count as one each)", byName["Unnamed"])
	}
}

func writeGoFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParamBudgetIsHard(t *testing.T) {
	root := t.TempDir()
	writeGoFile(t, root, filepath.Join("internal", "wide.go"), `package internal

func Wide(a, b, c, d, e, f, g string) {}
`)
	violations, err := scan(root, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || violations[0].function != "Wide" {
		t.Fatalf("violations = %+v, want only the 7-param Wide", violations)
	}
	if violations[0].kind != "params" {
		t.Fatalf("violation kind = %q, want params", violations[0].kind)
	}
}

func TestLengthBudgetRatchetsViaAllowanceFn(t *testing.T) {
	root := t.TempDir()
	body := "package internal\n\nfunc Long() {\n"
	for i := 0; i < 100; i++ {
		body += "\t_ = " + itoa(i) + "\n"
	}
	body += "}\n"
	writeGoFile(t, root, filepath.Join("internal", "long.go"), body)

	longKey := filepath.Join("internal", "long.go") + ":Long"

	// Frozen at its current 102 lines: allowed.
	frozen := func(key string) int {
		if key == longKey {
			return 102
		}
		return maxLength
	}
	if violations, err := scanWithAllowance(root, 1000, frozen); err != nil || len(violations) != 0 {
		t.Fatalf("frozen-length violations = %v (err %v), want none", violations, err)
	}

	// One line past the allowance: flagged.
	tightened := func(key string) int {
		if key == longKey {
			return 101
		}
		return maxLength
	}
	violations, err := scanWithAllowance(root, 1000, tightened)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || violations[0].kind != "length" || violations[0].lines != 102 {
		t.Fatalf("violations = %+v, want one length violation of 102 lines", violations)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func TestLoadLengthBaselineHandlesMissingCorruptAndWrongVersion(t *testing.T) {
	root := t.TempDir()

	missing, err := loadLengthBaseline(filepath.Join(root, "absent.json"))
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing baseline = (%v, %v), want empty map without error", missing, err)
	}

	corrupt := filepath.Join(root, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadLengthBaseline(corrupt); err == nil {
		t.Fatal("corrupt baseline must fail to load")
	}

	wrongVersion := filepath.Join(root, "wrong-version.json")
	if err := os.WriteFile(wrongVersion, []byte(`{"version":2,"functions":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadLengthBaseline(wrongVersion); err == nil {
		t.Fatal("unsupported baseline version must fail to load")
	}
}

func TestRunExitsZeroOnCleanRootAndNonZeroOnViolations(t *testing.T) {
	clean := t.TempDir()
	writeGoFile(t, clean, "ok.go", "package main\n\nfunc Fine() {}\n")
	if code := run(clean, map[string]int{}); code != 0 {
		t.Fatalf("clean run exit = %d, want 0", code)
	}

	dirty := t.TempDir()
	writeGoFile(t, dirty, "wide.go", "package main\n\nfunc Wide(a, b, c, d, e, f, g string) {}\n")
	if code := run(dirty, map[string]int{}); code != 1 {
		t.Fatalf("violating run exit = %d, want 1", code)
	}
	hot := t.TempDir()
	writeGoFile(t, hot, "hot.go", `package main

func Hot(v int) string {
	switch v {
	case 1:
		return "a"
	case 2:
		return "b"
	case 3:
		return "c"
	case 4:
		return "d"
	case 5:
		return "e"
	case 6:
		return "f"
	case 7:
		return "g"
	case 8:
		return "h"
	case 9:
		return "i"
	case 10:
		return "j"
	case 11:
		return "k"
	case 12:
		return "l"
	case 13:
		return "m"
	case 14:
		return "n"
	case 15:
		return "o"
	}
	return "z"
}
`)
	if code := run(hot, map[string]int{}); code != 1 {
		t.Fatalf("complexity-violating run exit = %d, want 1", code)
	}
	unparseable := t.TempDir()
	writeGoFile(t, unparseable, "broken.go", "this is not go source")
	if code := run(unparseable, map[string]int{}); code != 1 {
		t.Fatalf("scan-error run exit = %d, want 1", code)
	}

	// A ratcheted function at its frozen allowance passes, and one line past it fails.
	long := t.TempDir()
	body := "package main\n\nfunc Long() {\n"
	for i := 0; i < 100; i++ {
		body += "\t_ = " + itoa(i) + "\n"
	}
	body += "}\n"
	writeGoFile(t, long, "long.go", body)
	longKey := "long.go:Long"
	if code := run(long, map[string]int{longKey: 102}); code != 0 {
		t.Fatalf("frozen-length run exit = %d, want 0", code)
	}
	if code := run(long, map[string]int{longKey: 101}); code != 1 {
		t.Fatalf("over-allowance run exit = %d, want 1", code)
	}
}

func TestUpdateBaselineWritesAndCountsOverlongFunctions(t *testing.T) {
	root := t.TempDir()
	body := "package main\n\nfunc Long() {\n"
	for i := 0; i < 100; i++ {
		body += "\t_ = " + itoa(i) + "\n"
	}
	body += "}\n"
	writeGoFile(t, root, "long.go", body)
	writeGoFile(t, root, "short.go", "package main\n\nfunc Short() {}\n")

	baselinePath := filepath.Join(root, ".function-length-baseline-go.json")
	if err := updateBaseline([]string{root}, baselinePath); err != nil {
		t.Fatalf("updateBaseline() error = %v", err)
	}

	baseline, err := loadLengthBaseline(baselinePath)
	if err != nil {
		t.Fatalf("load updated baseline: %v", err)
	}
	if got, ok := baseline["long.go:Long"]; !ok || got != 102 {
		t.Fatalf("baseline entry = (%d, %v), want 102 lines for long.go:Long", got, ok)
	}
	if _, ok := baseline["short.go:Short"]; ok {
		t.Fatal("compliant function must not enter the baseline")
	}
	if got := countEntries(baselinePath); got != 1 {
		t.Fatalf("countEntries = %d, want 1", got)
	}
	if got := countEntries(filepath.Join(root, "missing.json")); got != 0 {
		t.Fatalf("countEntries on missing file = %d, want 0", got)
	}
	corrupt := filepath.Join(root, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := countEntries(corrupt); got != 0 {
		t.Fatalf("countEntries on corrupt file = %d, want 0", got)
	}

	badRoot := t.TempDir()
	writeGoFile(t, badRoot, "broken.go", "not go source")
	if err := updateBaseline([]string{badRoot}, baselinePath); err == nil {
		t.Fatal("updateBaseline on unparseable root must fail")
	}
}

func complexityOf(t *testing.T, source string) int {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := scan(filepath.Dir(path), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly one function", findings)
	}
	return findings[0].complexity
}
