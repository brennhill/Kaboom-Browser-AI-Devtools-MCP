// main_test.go — Holds the human UAT case inventory to being a real denominator.
//
// PURPOSE: the coverage ratchet, the release gate and the runner all count against
// this file. A denominator that can silently omit a mode measures nothing, so the
// inventory is checked against the shipped tool schema rather than maintained by
// hand, and a mode added to the schema fails this test until somebody writes the
// question a person answers for it.
//
// CONTRACT: every mode in the schema has exactly one case; every case names a mode
// the schema still has; every question is falsifiable by looking at something other
// than the tool's own output. The last one cannot be fully machine-checked, so what
// is checked is the shapes that make a question unfalsifiable — a duplicate
// question, an empty setup, and the vocabulary of assertions that cannot come out
// NO ("works", "verify", "as expected").

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Case is one thing a person is asked to judge.
type Case struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Tool and Mode are set for kind "mcp_mode" and are what the schema is
	// checked against.
	Tool string `json:"tool,omitempty"`
	Mode string `json:"mode,omitempty"`
	// Setup is what the person does first. Its job is to create something for the
	// mode to find: a mode run against a page with nothing to report returns an
	// empty result, and an empty result is not an error, which is exactly how 95
	// modes currently pass while asserting nothing.
	Setup string `json:"setup"`
	// Question is what they then look at. It must be answerable NO.
	Question string `json:"question"`
}

// Inventory is the whole file.
type Inventory struct {
	Version int    `json:"version"`
	Cases   []Case `json:"cases"`
}

const (
	kindMCPMode = "mcp_mode"
	kindSurface = "surface"
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

func loadInventory(t *testing.T) Inventory {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "uat", "human", "cases.json"))
	if err != nil {
		t.Fatalf("the case inventory is missing; every gate that counts coverage counts against it: %v", err)
	}
	var inventory Inventory
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatalf("the case inventory is not valid JSON: %v", err)
	}
	if len(inventory.Cases) == 0 {
		t.Fatal("the case inventory is empty, so every coverage number computed from it would be a division by zero")
	}
	return inventory
}

// schemaModes reads the shipped tool list and returns every tool/mode it exposes.
//
// The golden is the same document `tools/list` returns, so it is the same set the
// UAT harness enumerates at run time. Reading it here means the inventory is
// checked against what actually ships rather than against a second hand-kept list
// that would drift.
func schemaModes(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "cmd", "browser-agent", "testdata", "mcp-tools-list.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
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
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatal(err)
	}
	modes := map[string]bool{}
	for _, tool := range tools {
		for _, mode := range tool.InputSchema.Properties.What.Enum {
			modes[tool.Name+"/"+mode] = true
		}
	}
	if len(modes) == 0 {
		t.Fatal("no modes were read out of the tool schema, so this test would pass against an empty inventory")
	}
	return modes
}

func TestEverySchemaModeHasExactlyOneCase(t *testing.T) {
	t.Parallel()
	inventory := loadInventory(t)
	modes := schemaModes(t)

	seen := map[string]int{}
	for _, c := range inventory.Cases {
		if c.Kind != kindMCPMode {
			continue
		}
		seen[c.Tool+"/"+c.Mode]++
	}

	var missing, duplicated []string
	for mode := range modes {
		switch seen[mode] {
		case 1:
		case 0:
			missing = append(missing, mode)
		default:
			duplicated = append(duplicated, fmt.Sprintf("%s (%d cases)", mode, seen[mode]))
		}
	}
	sort.Strings(missing)
	sort.Strings(duplicated)
	if len(missing) > 0 {
		t.Errorf("%d mode(s) ship with no human UAT case, so nothing anywhere asks a person what they do:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
	if len(duplicated) > 0 {
		t.Errorf("%d mode(s) have more than one case, so the coverage denominator counts them twice:\n  %s",
			len(duplicated), strings.Join(duplicated, "\n  "))
	}
}

func TestNoCaseNamesAModeThatNoLongerShips(t *testing.T) {
	t.Parallel()
	inventory := loadInventory(t)
	modes := schemaModes(t)

	var stale []string
	for _, c := range inventory.Cases {
		if c.Kind != kindMCPMode {
			continue
		}
		if !modes[c.Tool+"/"+c.Mode] {
			stale = append(stale, c.ID)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("%d case(s) name a mode the schema no longer exposes; a tester sent to run them would be testing nothing:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}
}

// requiredSurfaces are the user-facing surfaces with no MCP mode at all.
//
// A function rather than a package var so the list cannot be mutated at runtime.
// These are hand-maintained because nothing in the schema knows about them — which
// is precisely why they are the parts most likely to ship untested.
func requiredSurfaces() []string {
	return []string{
		"popup/track_this_tab",
		"popup/pilot_toggle",
		"popup/connection_status",
		"popup/recording_controls",
		"supervision/overlay",
		"supervision/stop_button",
		"supervision/phantom_cursor",
		"tab_group/adoption",
		"tab_group/session_end",
		"terminal_panel/open",
		"terminal_panel/reconnect",
		"terminal_panel/folder_picker",
		"side_panel/open",
		"draw_mode/annotate",
		"draw_mode/exit",
		"keyboard/recording_shortcut",
		"context_menu/track_tab",
		"screenshots/error_capture",
		"doctor/diagnostics",
		"install/first_run",
	}
}

func TestEveryNonMCPSurfaceHasACase(t *testing.T) {
	t.Parallel()
	inventory := loadInventory(t)

	present := map[string]bool{}
	for _, c := range inventory.Cases {
		if c.Kind == kindSurface {
			present[c.ID] = true
		}
	}
	var missing []string
	for _, surface := range requiredSurfaces() {
		if !present[surface] {
			missing = append(missing, surface)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d user-facing surface(s) have no case. They have no MCP mode either, so nothing else covers them at all:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// unfalsifiableWording is the vocabulary of questions that cannot come out NO.
//
// "Does it work?" is answered yes by a mode that returned an empty result, which is
// the exact failure this rig exists to end: 151 of 174 modes currently pass on
// "the response is not an MCP error".
func unfalsifiableWording() []string {
	return []string{
		"does it work",
		"work correctly",
		"work as expected",
		"as expected",
		"verify that",
		"validate that",
		"successfully",
		"without errors",
		"without error",
		"no errors occur",
		"behaves correctly",
		"is correct",
	}
}

func TestNoQuestionIsUnfalsifiable(t *testing.T) {
	t.Parallel()
	inventory := loadInventory(t)

	var problems []string
	for _, c := range inventory.Cases {
		lower := strings.ToLower(c.Question)
		for _, phrase := range unfalsifiableWording() {
			if strings.Contains(lower, phrase) {
				problems = append(problems, fmt.Sprintf("%s: %q contains %q", c.ID, c.Question, phrase))
			}
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Errorf("%d question(s) a tester can only answer yes:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}

func TestNoTwoCasesAskTheSameQuestion(t *testing.T) {
	t.Parallel()
	inventory := loadInventory(t)

	byQuestion := map[string][]string{}
	for _, c := range inventory.Cases {
		key := strings.ToLower(strings.TrimSpace(c.Question))
		byQuestion[key] = append(byQuestion[key], c.ID)
	}
	var shared []string
	for question, ids := range byQuestion {
		if len(ids) > 1 {
			sort.Strings(ids)
			shared = append(shared, fmt.Sprintf("%q asked by %s", question, strings.Join(ids, ", ")))
		}
	}
	sort.Strings(shared)
	if len(shared) > 0 {
		// Two modes sharing a question means the question does not distinguish them,
		// so at most one of the two is actually being judged.
		t.Errorf("%d question(s) are reused across cases, so the modes sharing them are not being told apart:\n  %s",
			len(shared), strings.Join(shared, "\n  "))
	}
}

func TestEveryCaseIsAnswerableAsWritten(t *testing.T) {
	t.Parallel()
	inventory := loadInventory(t)

	var problems []string
	seenID := map[string]bool{}
	for _, c := range inventory.Cases {
		if c.ID == "" {
			problems = append(problems, "a case has no id")
			continue
		}
		if seenID[c.ID] {
			problems = append(problems, c.ID+": duplicate id")
		}
		seenID[c.ID] = true
		if c.Kind != kindMCPMode && c.Kind != kindSurface {
			problems = append(problems, fmt.Sprintf("%s: unknown kind %q", c.ID, c.Kind))
		}
		// The setup is what creates something to find. Without it the mode runs
		// against a page with nothing to report and every question answers yes.
		if len(strings.TrimSpace(c.Setup)) < 15 {
			problems = append(problems, fmt.Sprintf("%s: setup %q does not say what to do first", c.ID, c.Setup))
		}
		if !strings.HasSuffix(strings.TrimSpace(c.Question), "?") {
			problems = append(problems, fmt.Sprintf("%s: %q is not a question", c.ID, c.Question))
		}
		if len(strings.TrimSpace(c.Question)) < 25 {
			problems = append(problems, fmt.Sprintf("%s: question %q is too short to name what to look at", c.ID, c.Question))
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		t.Errorf("%d case(s) cannot be run as written:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}

func TestTheContractItselfDiscriminates(t *testing.T) {
	t.Parallel()
	// Control for every test above: they all read one file, and a file that failed
	// to load would make each of them pass vacuously. loadInventory fails the test
	// rather than returning empty, and this asserts the inventory it returns is the
	// real one — enough cases to cover the schema, not a stub.
	inventory := loadInventory(t)
	modes := schemaModes(t)
	if len(inventory.Cases) < len(modes) {
		t.Fatalf("the inventory holds %d cases for %d shipped modes plus %d non-MCP surfaces; it cannot be covering them",
			len(inventory.Cases), len(modes), len(requiredSurfaces()))
	}
}
