// drift_test.go — The gate. Fails when a shipped response no longer matches
// its declared shape, and names the mode and the field that moved.
//
// CONTRACT: for every mode declared in .mcp-response-contract.json, the shape
// derived from the response the shipped handler produces right now must equal
// the frozen one. A dropped field, a renamed field, an added field or a changed
// type all fail here with the mode and field named.
//
// Regenerate deliberately, never to silence a failure without reading it:
//
//	make response-contract-update
//
// Docs: docs/features/feature/quality-gates/index.md
package responsegate

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/contracts/responsecontract"
)

// repoRoot resolves the checkout root the contract file lives at.
func repoRoot(t *testing.T) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := responsecontract.RepoRoot(working)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// loadContract reads the checked-in contract, failing the test if it cannot.
func loadContract(t *testing.T) *responsecontract.Contract {
	t.Helper()
	contract, err := responsecontract.Load(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

// TestDeclaredResponseShapesStillShip is the drift gate.
func TestDeclaredResponseShapesStillShip(t *testing.T) {
	root := repoRoot(t)
	swept := sweep(t)
	shapes, refused := swept.shapes, swept.refusals
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		refreeze(t, root, shapes)
		return
	}

	contract := loadContract(t)

	var failures []string
	for mode, declared := range contract.Modes {
		shipped, produced := shapes[mode]
		if !produced {
			// A declared mode that stopped answering is drift too: the mode was
			// declarable when the contract was frozen and is not any more.
			failures = append(failures, mode+": declared, but the shipped handler no longer answers with a payload here — "+refusalOf(refused, mode))
			continue
		}
		failures = append(failures, responsecontract.Details(responsecontract.Diff(mode, declared, shipped))...)
	}
	failures = append(failures, envelopeDrift(contract, shapes)...)

	sort.Strings(failures)
	if len(failures) > 0 {
		t.Fatalf("%d MCP response shape(s) drifted from the declared contract in %s:\n  %s\n\nIf the change is intended, re-freeze with `make response-contract-update` and review the diff.",
			len(failures), responsecontract.ContractPath, strings.Join(failures, "\n  "))
	}
}

// envelopeDrift compares the declared async lifecycle envelope.
func envelopeDrift(contract *responsecontract.Contract, shapes map[string]responsecontract.Shape) []string {
	declared, isDeclared := contract.Envelopes[responsecontract.EnvelopeQueued]
	if !isDeclared {
		return []string{responsecontract.EnvelopeQueued + ": the async lifecycle envelope is not declared; every browser-mediated mode returns it and nothing pins its shape"}
	}
	shipped, produced := shapes[responsecontract.EnvelopeQueued]
	if !produced {
		return []string{responsecontract.EnvelopeQueued + ": the async owner no longer mints a queued envelope"}
	}
	return responsecontract.Details(responsecontract.Diff(responsecontract.EnvelopeQueued, declared, shipped))
}

func refusalOf(refused map[string]refusalRecord, mode string) string {
	if record, present := refused[mode]; present {
		return record.kind + ": " + firstLine(record.detail)
	}
	return "the harness produced no case for it at all"
}

// refreeze writes the derived shapes back to the contract, recomputing the
// ratchet baseline from the shipped tool document so the two can never disagree.
func refreeze(t *testing.T, root string, shapes map[string]responsecontract.Shape) {
	t.Helper()
	shipped, err := responsecontract.ShippedModes(root)
	if err != nil {
		t.Fatal(err)
	}
	contract := &responsecontract.Contract{
		Envelopes: map[string]responsecontract.Shape{},
		Modes:     map[string]responsecontract.Shape{},
	}
	var unadvertised []string
	for key, shape := range shapes {
		if key == responsecontract.EnvelopeQueued {
			contract.Envelopes[key] = shape
			continue
		}
		if !shipped[key] {
			// Reachable through the dispatcher but absent from the tool
			// document — observe's four "silent" modes. The ratchet counts
			// only what the schema advertises, so declaring these would let a
			// mode nobody can call pay down the baseline.
			unadvertised = append(unadvertised, key)
			continue
		}
		contract.Modes[key] = shape
	}
	sort.Strings(unadvertised)
	contract.UndeclaredBaseline = len(responsecontract.Undeclared(shipped, contract))
	if err := responsecontract.Save(root, contract); err != nil {
		t.Fatal(err)
	}
	t.Logf("re-froze %d mode shape(s) and %d envelope(s); undeclared_baseline=%d of %d shipped modes; skipped %d unadvertised dispatcher mode(s): %v",
		len(contract.Modes), len(contract.Envelopes), contract.UndeclaredBaseline,
		len(shipped), len(unadvertised), unadvertised)
}

// TestTheHarnessAnswersWithoutABrowser is the control for the gate above. If
// the harness produced nothing, every comparison in the drift gate would be
// vacuous and a real drift would pass unnoticed.
func TestTheHarnessAnswersWithoutABrowser(t *testing.T) {
	swept := sweep(t)
	shapes, refused := swept.shapes, swept.refusals
	if len(shapes) == 0 {
		t.Fatalf("the harness derived no shapes at all, so the drift gate proves nothing; %d mode(s) were refused: %v", len(refused), refused)
	}
	if _, ok := shapes[responsecontract.EnvelopeQueued]; !ok {
		t.Fatalf("the async lifecycle envelope was not derived: %s", refusalOf(refused, responsecontract.EnvelopeQueued))
	}
	for mode, shape := range shapes {
		if len(shape.Fields) == 0 {
			t.Errorf("%s derived an empty shape; an empty declaration cannot detect a dropped field", mode)
		}
	}
	t.Logf("derived %d shape(s) with no browser; %d mode(s) refused", len(shapes), len(refused))
}
