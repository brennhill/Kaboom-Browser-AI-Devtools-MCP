// offline_ci_test.go — Keeps the offline UAT suite scheduled, complete, and wired into CI.
//
// PURPOSE: 23 of the 34 UAT categories need neither a browser nor an extension,
// and until the offline-uat job none of them ran in CI. Adding the job removes
// one failure mode and creates another. A failing category is now loud — the
// runner exits non-zero. A category that stops being *scheduled* is silent:
// deleting an id from OFFLINE_CAT_IDS is a one-token edit to a shell string,
// after which the suite reports a smaller total and passes. That is the cheapest
// way to make a red build green, and it is the one this file forbids.
//
// CONTRACT: every category script on disk is scheduled by the runner except the
// human-only one it names; the offline list does not shrink; and ci.yml runs the
// offline suite in a job that is allowed to fail the build.

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const ciWorkflowFile = ".github/workflows/ci.yml"

// offlineFloor is the number of categories the offline suite ran on the day it
// became a required job. Categories are added, never removed, so this only ever
// moves up. Deleting a script and its id together defeats the completeness check
// below; it does not defeat this one.
const offlineFloor = 23

// humanOnlyCategories are the ids that intentionally have no place in either
// suite. Category 27 blocks on `read -r` for a person to look at browser
// overlays, so scheduling it would hang the run. Every other script on disk must
// be scheduled: an unscheduled script reads as coverage while never running,
// which is how category 32 sat at 8/8 green with every call failing to parse.
var humanOnlyCategories = map[string]string{
	"27": "pauses for human visual verification of browser overlays",
}

// suiteCategoryIDs reads one named id list out of the runner.
func suiteCategoryIDs(t *testing.T, suite string) []string {
	t.Helper()
	raw := readFile(t, runnerFile)
	pattern := regexp.MustCompile(`(?m)^` + suite + `_CAT_IDS="([^"]*)"`)
	match := pattern.FindStringSubmatch(raw)
	if match == nil {
		t.Fatalf("no %s_CAT_IDS assignment in %s; this test would otherwise check nothing", suite, runnerFile)
	}
	ids := strings.Fields(match[1])
	if len(ids) == 0 {
		t.Fatalf("%s_CAT_IDS is empty, so the %s suite runs no categories at all", suite, strings.ToLower(suite))
	}
	return ids
}

// categoryScriptIDs lists the ids that have a script checked in.
func categoryScriptIDs(t *testing.T) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(repoRoot(t), testsRoot, "*", "cat-*-*.sh"))
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`^cat-(\d+)-`)
	seen := map[string]bool{}
	var ids []string
	for _, path := range found {
		match := pattern.FindStringSubmatch(filepath.Base(path))
		if match == nil || seen[match[1]] {
			continue
		}
		seen[match[1]] = true
		ids = append(ids, match[1])
	}
	if len(ids) == 0 {
		t.Fatalf("no category scripts were found under %s, so every assertion below would pass vacuously", testsRoot)
	}
	sort.Strings(ids)
	return ids
}

func TestEveryCategoryScriptIsScheduledBySomeSuite(t *testing.T) {
	t.Parallel()
	scheduled := map[string]bool{}
	for _, id := range append(suiteCategoryIDs(t, "OFFLINE"), suiteCategoryIDs(t, "CONNECTED")...) {
		scheduled[id] = true
	}

	var unscheduled []string
	for _, id := range categoryScriptIDs(t) {
		if scheduled[id] || humanOnlyCategories[id] != "" {
			continue
		}
		unscheduled = append(unscheduled, id)
	}
	if len(unscheduled) > 0 {
		t.Errorf("%d category script(s) exist but no suite runs them, so they count as coverage that never executes:\n  %s\nAdd each id to OFFLINE_CAT_IDS or CONNECTED_CAT_IDS in %s, or document it in humanOnlyCategories here with the reason it cannot be automated.",
			len(unscheduled), strings.Join(unscheduled, "\n  "), runnerFile)
	}
}

func TestNoScheduledCategoryIsAlsoDeclaredHumanOnly(t *testing.T) {
	t.Parallel()
	// Control for the test above: an exclusion list that grew to cover every id
	// would satisfy it while excusing the whole suite.
	scheduled := map[string]bool{}
	for _, id := range append(suiteCategoryIDs(t, "OFFLINE"), suiteCategoryIDs(t, "CONNECTED")...) {
		scheduled[id] = true
	}
	for id, reason := range humanOnlyCategories {
		if scheduled[id] {
			t.Errorf("category %s is listed here as human-only (%q) and is also scheduled by the runner; one of the two is wrong", id, reason)
		}
	}
	if len(humanOnlyCategories) >= len(categoryScriptIDs(t)) {
		t.Fatal("every category on disk is excused as human-only, so the completeness check proves nothing")
	}
}

func TestTheOfflineSuiteDoesNotShrink(t *testing.T) {
	t.Parallel()
	ids := suiteCategoryIDs(t, "OFFLINE")
	if len(ids) < offlineFloor {
		t.Fatalf("OFFLINE_CAT_IDS lists %d categories, below the %d that ran when the offline suite became a required job. Removing a category from the list is how a red suite is quietly made green — fix the category instead. If a category was genuinely retired, delete its script and lower offlineFloor in the same change.",
			len(ids), offlineFloor)
	}
}

// ciJobBlock returns the text of one top-level job in ci.yml.
func ciJobBlock(t *testing.T, job string) string {
	t.Helper()
	raw := readFile(t, ciWorkflowFile)
	start := regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(job) + `:$`).FindStringIndex(raw)
	if start == nil {
		t.Fatalf("%s defines no job named %q. The offline UAT categories run in CI only if a job runs them.", ciWorkflowFile, job)
	}
	rest := raw[start[1]:]
	if next := regexp.MustCompile(`(?m)^  [a-z][a-z0-9-]*:$`).FindStringIndex(rest); next != nil {
		return rest[:next[0]]
	}
	return rest
}

func TestCIRunsTheOfflineSuite(t *testing.T) {
	t.Parallel()
	block := ciJobBlock(t, "offline-uat")

	for _, required := range []string{
		"./scripts/uat/runners/test-all-tools-comprehensive.sh --suite offline",
		// Without an explicit wrapper the runner falls back to whatever
		// kaboom-agentic-browser is on PATH, and the job would test a binary it
		// did not build.
		"KABOOM_UAT_WRAPPER=",
		"go build -o dist/kaboom-agentic-browser ./cmd/browser-agent",
	} {
		if !strings.Contains(block, required) {
			t.Errorf("the offline-uat job in %s does not contain %q, so it does not run the offline suite against a daemon it built", ciWorkflowFile, required)
		}
	}
}

func TestTheOfflineJobCanFailTheBuild(t *testing.T) {
	t.Parallel()
	block := ciJobBlock(t, "offline-uat")

	if strings.Contains(block, "continue-on-error") {
		t.Error("the offline-uat job sets continue-on-error, so a failing category no longer fails the build — which is the state this job exists to end")
	}
	for _, line := range strings.Split(block, "\n") {
		if !strings.Contains(line, "test-all-tools-comprehensive.sh --suite offline") {
			continue
		}
		if strings.Contains(line, "|| true") || strings.Contains(line, "|| exit 0") || strings.Contains(line, "|| echo") {
			t.Errorf("the runner invocation swallows its own exit status: %q", strings.TrimSpace(line))
		}
		return
	}
	t.Fatal("control: no line in the offline-uat job invokes the offline suite, so this check inspected nothing")
}

func TestTheOfflineSuiteNeedsNoBrowser(t *testing.T) {
	t.Parallel()
	// The offline job runs on a bare ubuntu runner with no Chrome and no
	// extension. A category moved from the connected list to the offline one
	// without being made browser-free would fail there every time, so the job
	// must not be given a browser launcher to lean on.
	block := ciJobBlock(t, "offline-uat")
	for _, forbidden := range []string{"uat-browser-launch.sh", "KABOOM_UAT_REPLAY", "KABOOM_UAT_REQUIRE_CONNECTED"} {
		if strings.Contains(block, forbidden) {
			t.Errorf("the offline-uat job references %s; the offline categories are the ones that need no browser, and wiring one in hides a category that stopped being offline", forbidden)
		}
	}
}

func TestTheRunnerRefusesToPassWithoutAssertions(t *testing.T) {
	t.Parallel()
	// The exit status and the printed verdict must come from the one shared
	// rule. They were written separately and disagreed: the verdict required a
	// passing assertion, the exit code did not, so a suite scheduling zero
	// categories exited 0.
	runner := readFile(t, runnerFile)
	if !strings.Contains(runner, "uat_suite_passed ") {
		t.Fatalf("%s no longer consults uat_suite_passed, so its exit status and its printed verdict can disagree again", runnerFile)
	}
	if regexp.MustCompile(`(?m)^exit 0$`).MatchString(runner) {
		t.Errorf("%s ends with an unconditional `exit 0`", runnerFile)
	}
	lib, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts/uat/orchestration/uat-result-lib.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lib), "uat_suite_passed()") {
		t.Error("uat_suite_passed is not defined in uat-result-lib.sh, so sourcing the library leaves the runner calling a missing function")
	}
}
