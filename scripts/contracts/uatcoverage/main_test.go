// main_test.go — Makes the reachability-only count a gate that runs on every
// commit, not one that runs when somebody has a browser attached.
//
// PURPOSE: cat-33 already refuses to let the reachability-only count grow, but
// cat-33 needs a connected extension, and only 1 of 34 UAT categories runs in
// CI. A ratchet nobody runs is not a ratchet: 95 modes accumulated
// reachability-only coverage with nothing noticing. Everything checked here is
// derivable from two checked-in files — the shipped tool schema and the
// expectations table — so it runs under `go test ./...` with no browser.
//
// CONTRACT: the baseline equals the real count exactly, every expectation names
// a mode that still ships, and a new mode without an expectation fails.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	expectationsFile = "scripts/tests/browser/mode-content-expectations.sh"
	schemaGolden     = "cmd/browser-agent/testdata/mcp-tools-list.golden.json"
)

// repoRoot walks up from the test's directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find the repository root above the test's working directory")
	return ""
}

func readFile(t *testing.T, relative string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), relative))
	if err != nil {
		t.Fatalf("%s: %v", relative, err)
	}
	return string(raw)
}

// shippedModes reads every tool/mode out of the golden tool list.
func shippedModes(t *testing.T) map[string]bool {
	t.Helper()
	var tools []struct {
		Name        string `json:"name"`
		InputSchema struct {
			Properties struct {
				What struct {
					Enum []string `json:"enum"`
				} `json:"what"`
			} `json:"properties"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal([]byte(readFile(t, schemaGolden)), &tools); err != nil {
		t.Fatal(err)
	}
	modes := map[string]bool{}
	for _, tool := range tools {
		for _, mode := range tool.InputSchema.Properties.What.Enum {
			modes[tool.Name+"/"+mode] = true
		}
	}
	if len(modes) == 0 {
		t.Fatal("no modes were read out of the schema, so every count below would be zero and every assertion would pass")
	}
	return modes
}

// expectationPattern matches one `tool/mode) echo '...' ;;` case arm.
var expectationPattern = regexp.MustCompile(`(?m)^\s+([a-z_]+/[a-z0-9_]+)\)\s+echo\s`)

// expectedModes reads the modes that have a content expectation.
func expectedModes(t *testing.T) map[string]bool {
	t.Helper()
	table := readFile(t, expectationsFile)
	expected := map[string]bool{}
	for _, match := range expectationPattern.FindAllStringSubmatch(table, -1) {
		expected[match[1]] = true
	}
	if len(expected) == 0 {
		t.Fatalf("no expectations were parsed out of %s; the parser is broken and every count below would be wrong", expectationsFile)
	}
	return expected
}

// baselinePattern reads the ratchet out of the same file the sweep reads it from.
var baselinePattern = regexp.MustCompile(`UAT_REACHABILITY_BASELINE="\$\{UAT_REACHABILITY_BASELINE:-(\d+)\}"`)

func declaredBaseline(t *testing.T) int {
	t.Helper()
	match := baselinePattern.FindStringSubmatch(readFile(t, expectationsFile))
	if match == nil {
		t.Fatalf("could not find UAT_REACHABILITY_BASELINE in %s; the sweep and this gate would ratchet against different numbers", expectationsFile)
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// reachabilityOnly lists the shipped modes with no content expectation.
func reachabilityOnly(t *testing.T) []string {
	t.Helper()
	expected := expectedModes(t)
	var only []string
	for mode := range shippedModes(t) {
		if !expected[mode] {
			only = append(only, mode)
		}
	}
	sort.Strings(only)
	return only
}

func TestTheBaselineIsExactlyTheRealCount(t *testing.T) {
	t.Parallel()
	actual := len(reachabilityOnly(t))
	baseline := declaredBaseline(t)

	if actual > baseline {
		t.Fatalf("%d modes pass on reachability alone, above the baseline of %d. A new mode joined the untested majority: give it a content expectation in %s.",
			actual, baseline, expectationsFile)
	}
	// Slack in the ratchet is a mode's worth of free coverage: with the baseline
	// above the real count, the next mode added with no expectation passes both
	// this gate and cat-33, which is exactly how the count grew to 131.
	if actual < baseline {
		t.Fatalf("%d modes pass on reachability alone but the baseline still says %d. Lower UAT_REACHABILITY_BASELINE to %d in %s so the improvement is locked in.",
			actual, baseline, actual, expectationsFile)
	}
}

func TestEveryExpectationNamesAModeThatStillShips(t *testing.T) {
	t.Parallel()
	shipped := shippedModes(t)

	var stale []string
	for mode := range expectedModes(t) {
		if !shipped[mode] {
			stale = append(stale, mode)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		// A stale entry is worse than a missing one: it counts toward the asserted
		// total and lowers the reachability-only count, so the sweep reports
		// coverage for a mode that no longer exists.
		t.Errorf("%d expectation(s) name a mode the schema no longer exposes, and each one is counted as coverage that does not exist:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}
}

func TestTheUntestedModesAreNamedNotJustCounted(t *testing.T) {
	t.Parallel()
	only := reachabilityOnly(t)
	if len(only) == 0 {
		t.Skip("nothing is reachability-only any more")
	}

	// Control for the two tests above: they both count a set this parser
	// produces, so a parser that produced nothing would make them pass
	// vacuously. This asserts the set is real and printable, and gives whoever
	// reads a failure the actual list to work from.
	if len(only) > len(shippedModes(t)) {
		t.Fatalf("more untested modes (%d) than modes (%d): the parser is wrong", len(only), len(shippedModes(t)))
	}
	for _, mode := range only {
		if !strings.Contains(mode, "/") {
			t.Fatalf("parsed %q as a mode", mode)
		}
	}
	t.Logf("%d of %d modes still pass on reachability alone:\n  %s",
		len(only), len(shippedModes(t)), strings.Join(only, "\n  "))
}

func TestEveryUntestedModeStillHasAHumanCase(t *testing.T) {
	t.Parallel()
	// The two layers are complements, not alternatives: a mode with no automated
	// content assertion is exactly the one that must be judged by a person. If
	// this ever fails, a mode is covered by neither.
	var cases struct {
		Cases []struct {
			Tool string `json:"tool"`
			Mode string `json:"mode"`
		} `json:"cases"`
	}
	if err := json.Unmarshal([]byte(readFile(t, "scripts/uat/human/cases.json")), &cases); err != nil {
		t.Fatal(err)
	}
	human := map[string]bool{}
	for _, c := range cases.Cases {
		if c.Tool != "" {
			human[c.Tool+"/"+c.Mode] = true
		}
	}

	var uncovered []string
	for _, mode := range reachabilityOnly(t) {
		if !human[mode] {
			uncovered = append(uncovered, mode)
		}
	}
	if len(uncovered) > 0 {
		t.Errorf("%d mode(s) have neither an automated content assertion nor a human case:\n  %s",
			len(uncovered), strings.Join(uncovered, "\n  "))
	}
	if len(human) == 0 {
		t.Fatal("control: no human cases were parsed, so the check above proved nothing")
	}
}
