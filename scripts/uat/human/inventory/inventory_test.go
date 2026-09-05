// inventory_test.go — Proves a missing or empty inventory is an error, never an
// empty run.

package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func writeInventory(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cases.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRefusesAnInventoryWithNothingInIt(t *testing.T) {
	t.Parallel()
	// Every coverage number is computed against this list. An empty one would
	// make the runner report nothing to do and the gate divide by zero — both of
	// which look like success.
	for name, body := range map[string]string{
		"empty case list": `{"version":1,"cases":[]}`,
		"not json":        `{`,
	} {
		if _, err := Load(writeInventory(t, body)); err == nil {
			t.Errorf("%s: loaded without error", name)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("a missing inventory loaded without error")
	}

	// Control: a real inventory loads, so the assertions above are not passing
	// because Load always fails.
	loaded, err := Load(writeInventory(t, `{"version":1,"cases":[{"id":"observe/page","kind":"mcp_mode","tool":"observe","mode":"page","setup":"Open a page.","question":"Does it describe the page you are looking at?"}]}`))
	if err != nil {
		t.Fatalf("a valid inventory failed to load: %v", err)
	}
	if len(loaded.Cases) != 1 {
		t.Fatalf("cases = %d, want 1", len(loaded.Cases))
	}
}

func TestCallArgumentsDefaultToTheBareMode(t *testing.T) {
	t.Parallel()
	bare := Case{Kind: KindMCPMode, Tool: "observe", Mode: "screenshot"}
	if got := bare.CallArguments()["what"]; got != "screenshot" {
		t.Errorf("what = %v, want the mode", got)
	}

	// A case that names its own arguments must have them used verbatim: a mode
	// like interact/click cannot be judged from `{"what":"click"}` alone, and
	// silently dropping the selector would present a failing call as the tool's
	// normal behaviour.
	explicit := Case{Kind: KindMCPMode, Tool: "interact", Mode: "click",
		Arguments: map[string]any{"what": "click", "selector": "#submit"}}
	args := explicit.CallArguments()
	if args["selector"] != "#submit" {
		t.Errorf("arguments = %v, want the case's own", args)
	}
}

func TestUnfalsifiablePhraseNamesTheOffendingWords(t *testing.T) {
	t.Parallel()
	phrase, banned := UnfalsifiablePhrase("Does the screenshot work as expected?")
	if !banned {
		t.Fatal("a question nobody can answer NO was accepted")
	}
	if phrase == "" {
		t.Error("the phrase was not named, so an author cannot tell what to rewrite")
	}
	// Control: a real question is not rejected, or every case would be flagged.
	if _, banned := UnfalsifiablePhrase("Does the image show the part of the page that is on screen right now?"); banned {
		t.Error("a falsifiable question was rejected")
	}
}

func TestSchemaModeMatchesTheToolListKey(t *testing.T) {
	t.Parallel()
	c := Case{Tool: "analyze", Mode: "design_audit"}
	if c.SchemaMode() != "analyze/design_audit" {
		t.Errorf("SchemaMode() = %q; the contract matches cases to the schema on this key", c.SchemaMode())
	}
}
