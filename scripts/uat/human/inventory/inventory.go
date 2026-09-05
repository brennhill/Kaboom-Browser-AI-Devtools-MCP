// Purpose: Owns the human-UAT case inventory — its shape, its file, and the rules a case must satisfy.
// Why: The contract test, the runner, and the coverage ratchet must count the same cases.
// Docs: docs/features/feature/human-uat-rig/index.md

package inventory

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Kinds of case. A mode case is derived from the shipped tool schema; a surface
// case covers something a person uses that has no MCP mode at all.
const (
	KindMCPMode = "mcp_mode"
	KindSurface = "surface"
)

// Case is one thing a person is asked to judge.
type Case struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Tool and Mode are set for KindMCPMode and are what the schema is checked
	// against.
	Tool string `json:"tool,omitempty"`
	Mode string `json:"mode,omitempty"`
	// Setup is what the person does first. Its job is to create something for the
	// mode to find: a mode run against a page with nothing to report returns an
	// empty result, and an empty result is not an error — which is exactly how
	// most modes passed the reachability sweep this rig replaces.
	Setup string `json:"setup"`
	// Question is what they then look at. It must be answerable NO.
	Question string `json:"question"`
	// Arguments is the tool call to make, when the bare mode is not enough.
	// Absent means the runner calls the tool with only `what`.
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Inventory is the whole file.
type Inventory struct {
	Version int    `json:"version"`
	Cases   []Case `json:"cases"`
}

// RelativePath is where the inventory lives, from the repository root.
const RelativePath = "scripts/uat/human/cases.json"

// Load reads an inventory file.
//
// An empty inventory is an error rather than an empty result: every coverage
// number the rig reports is computed against this list, so an inventory that
// failed to load would make the gate divide by zero and the runner report
// nothing to do.
func Load(path string) (Inventory, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Inventory{}, fmt.Errorf("read case inventory: %w", err)
	}
	var loaded Inventory
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return Inventory{}, fmt.Errorf("parse case inventory %s: %w", path, err)
	}
	if len(loaded.Cases) == 0 {
		return Inventory{}, fmt.Errorf("%s holds no cases", path)
	}
	return loaded, nil
}

// SchemaMode is the tool/mode key a case is matched to.
func (c Case) SchemaMode() string { return c.Tool + "/" + c.Mode }

// CallArguments returns the arguments to send for a mode case.
//
// The bare `{"what": mode}` default is deliberate: it is the call an agent makes
// when it knows nothing else, so a mode that only works with extra parameters
// should be answered against that call, not against a hand-tuned one.
func (c Case) CallArguments() map[string]any {
	if len(c.Arguments) > 0 {
		return c.Arguments
	}
	return map[string]any{"what": c.Mode}
}

// RequiredSurfaces are the user-facing surfaces with no MCP mode at all.
//
// A function rather than a package var so the list cannot be mutated at runtime.
// These are hand-maintained because nothing in the schema knows about them —
// which is precisely why they are the parts most likely to ship untested.
func RequiredSurfaces() []string {
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

// UnfalsifiableWording is the vocabulary of questions that cannot come out NO.
//
// "Does it work?" is answered yes by a mode that returned an empty result, which
// is the exact failure this rig exists to end.
func UnfalsifiableWording() []string {
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

// UnfalsifiablePhrase reports the first banned phrase in a question, if any.
func UnfalsifiablePhrase(question string) (string, bool) {
	lower := strings.ToLower(question)
	for _, phrase := range UnfalsifiableWording() {
		if strings.Contains(lower, phrase) {
			return phrase, true
		}
	}
	return "", false
}
