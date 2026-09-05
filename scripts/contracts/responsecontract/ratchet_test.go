// ratchet_test.go — The ratchet. Modes with no declared response shape may only
// decrease, and an improvement must be locked in.
//
// PURPOSE: 173 modes ship and only some of them have a declared response shape.
// A gate that merely exists lets the undeclared majority sit there forever —
// which is how 131 modes accumulated reachability-only UAT coverage with
// nothing noticing. This holds the undeclared count at exactly the recorded
// baseline: above it a new mode joined the undeclared majority, below it
// somebody improved the contract without re-freezing, and the next mode added
// with no shape would have been paid for by that unclaimed slack.
//
// Everything here is derived from two checked-in files — the shipped tool
// document and the contract — so it runs under `go test ./...` with no browser.
//
// Docs: docs/features/feature/quality-gates/index.md
package responsecontract

import (
	"os"
	"strings"
	"testing"
)

// fixture loads the shipped modes and the declared contract together.
func fixture(t *testing.T) (map[string]bool, *Contract) {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := RepoRoot(working)
	if err != nil {
		t.Fatal(err)
	}
	shipped, err := ShippedModes(root)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return shipped, contract
}

const refreezeCommand = "UPDATE_GOLDEN=1 go test ./cmd/browser-agent/internal/responsegate"

func TestUndeclaredModesOnlyShrink(t *testing.T) {
	t.Parallel()
	shipped, contract := fixture(t)
	undeclared := Undeclared(shipped, contract)

	if len(undeclared) > contract.UndeclaredBaseline {
		t.Fatalf("%d of %d shipped modes have no declared response shape, above the baseline of %d. A new mode joined the undeclared majority: give it a shape by making it answerable in the response harness, then re-freeze with %s.",
			len(undeclared), len(shipped), contract.UndeclaredBaseline, refreezeCommand)
	}
	// Slack in a ratchet is a free pass: with the baseline above the real
	// count, the next mode added with no declared shape passes this gate.
	if len(undeclared) < contract.UndeclaredBaseline {
		t.Fatalf("%d of %d shipped modes have no declared response shape but the baseline still says %d. Lower undeclared_baseline to %d by re-freezing with %s so the improvement is locked in.",
			len(undeclared), len(shipped), contract.UndeclaredBaseline, len(undeclared), refreezeCommand)
	}
}

func TestEveryDeclaredModeStillShips(t *testing.T) {
	t.Parallel()
	shipped, contract := fixture(t)

	// A stale entry is worse than a missing one: it counts as coverage that
	// does not exist and holds the undeclared baseline artificially low, so
	// the next undeclared mode is paid for by a mode nobody can call.
	if stale := staleModes(shipped, contract); len(stale) > 0 {
		t.Errorf("%d declared shape(s) name a mode the tool document no longer exposes:\n  %s\nRe-freeze with %s.",
			len(stale), strings.Join(stale, "\n  "), refreezeCommand)
	}
}

func TestTheAsyncLifecycleEnvelopeIsDeclared(t *testing.T) {
	t.Parallel()
	_, contract := fixture(t)

	// Every browser-mediated mode answers with this envelope before the browser
	// replies. While it was undeclared, cat-33's analyze/feature_gates
	// expectation could only match "result", which proved the query was queued
	// and nothing about what came back.
	envelope, declared := contract.Envelopes[EnvelopeQueued]
	if !declared {
		t.Fatalf("the async lifecycle envelope %q is not declared; the dual response shape stays folklore", EnvelopeQueued)
	}
	if envelope.Kind != kindEnvelope {
		t.Fatalf("the declared async envelope has kind %q, want %q", envelope.Kind, kindEnvelope)
	}
	for _, required := range []string{"correlation_id", "lifecycle_status", "status", "final"} {
		if !hasPath(envelope, required) {
			t.Errorf("the declared async envelope has no %q field; a caller cannot tell a queued answer from a finished one", required)
		}
	}
}

func TestTheUndeclaredModesAreNamedNotJustCounted(t *testing.T) {
	t.Parallel()
	shipped, contract := fixture(t)
	undeclared := Undeclared(shipped, contract)

	// Control for the two ratchets above: both count a set these helpers
	// produce, so a helper that produced nothing would make them pass
	// vacuously. This asserts the sets are real, and prints the working list.
	if len(shipped) == 0 || len(contract.Modes) == 0 {
		t.Fatal("control: the shipped modes or the declared modes parsed empty, so every count above proved nothing")
	}
	if len(undeclared) > len(shipped) {
		t.Fatalf("more undeclared modes (%d) than shipped modes (%d): the parser is wrong", len(undeclared), len(shipped))
	}
	for _, mode := range undeclared {
		if !strings.Contains(mode, "/") {
			t.Fatalf("parsed %q as a mode", mode)
		}
	}
	t.Logf("%d of %d shipped modes have a declared response shape; %d do not:\n  %s",
		len(contract.Modes), len(shipped), len(undeclared), strings.Join(undeclared, "\n  "))
}

func TestEveryDeclaredShapeCanDetectADroppedField(t *testing.T) {
	t.Parallel()
	_, contract := fixture(t)

	// A declaration with no fields matches every response, so it would pass the
	// drift gate forever while proving nothing.
	for mode, shape := range contract.Modes {
		if len(shape.Fields) == 0 {
			t.Errorf("%s declares no fields; an empty declaration cannot detect a dropped field", mode)
		}
		if shape.Kind != kindDirect && shape.Kind != kindEnvelope {
			t.Errorf("%s declares kind %q, which is neither %q nor %q", mode, shape.Kind, kindDirect, kindEnvelope)
		}
	}
}

func hasPath(shape Shape, path string) bool {
	for _, field := range shape.Fields {
		if field.Path == path {
			return true
		}
	}
	return false
}
