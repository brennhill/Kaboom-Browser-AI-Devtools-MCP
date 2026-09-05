// control_test.go — Proves the drift gate catches a REAL drift.
//
// The tests in scripts/contracts/responsecontract mutate a shape. These mutate
// the actual response a shipped handler produced, using the production
// mutator, then run the same comparison the gate runs. If the gate could be
// fooled, it would be fooled here.
//
// Docs: docs/features/feature/quality-gates/index.md
package responsegate

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/contracts/responsecontract"
)

const controlMode = "observe/errors"

// shippedAndDeclared returns the live response for controlMode and the shape
// frozen for it in the checked-in contract.
func shippedAndDeclared(t *testing.T) (mcp.JSONRPCResponse, responsecontract.Shape) {
	t.Helper()
	fixture := newHarness()
	t.Cleanup(fixture.close)

	var live mcp.JSONRPCResponse
	cases, _ := fixture.cases()
	for _, testCase := range cases {
		if testCase.mode == controlMode {
			live = testCase.response
		}
	}
	if live.Result == nil {
		t.Fatalf("the harness produced no %s response, so this control proves nothing", controlMode)
	}
	contract, err := responsecontract.Load(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	declared, present := contract.Modes[controlMode]
	if !present {
		t.Fatalf("%s is not declared, so this control proves nothing", controlMode)
	}
	return live, declared
}

// driftOf runs the gate's own comparison over a possibly-mutated response.
func driftOf(t *testing.T, response mcp.JSONRPCResponse, declared responsecontract.Shape) []responsecontract.Drift {
	t.Helper()
	shipped, err := responsecontract.ShapeOfResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	return responsecontract.Diff(controlMode, declared, shipped)
}

// TestTheUnmutatedShippedResponseIsGreen is the negative half of the control.
// Without it, a gate that always failed would satisfy every test below.
func TestTheUnmutatedShippedResponseIsGreen(t *testing.T) {
	live, declared := shippedAndDeclared(t)

	if drifts := driftOf(t, live, declared); len(drifts) != 0 {
		t.Fatalf("the unmodified shipped response already reports drift, so a real drift could not be told apart:\n  %s",
			strings.Join(responsecontract.Details(drifts), "\n  "))
	}
}

// TestRenamingAFieldInTheShippedResponseTurnsTheGateRed is the positive half.
// The mutation is ordinary Go against the production payload mutator: it
// compiles, it runs, and it is exactly the edit a handler author would make.
func TestRenamingAFieldInTheShippedResponseTurnsTheGateRed(t *testing.T) {
	live, declared := shippedAndDeclared(t)

	renamed := mcp.MutateResultPayload(live, func(payload map[string]any) bool {
		value, present := payload["count"]
		if !present {
			t.Fatalf("%s no longer carries count; pick another field for the control", controlMode)
		}
		delete(payload, "count")
		payload["total"] = value
		return true
	})

	drifts := driftOf(t, renamed, declared)
	assertReports(t, drifts, controlMode, "count", "GONE")
	assertReports(t, drifts, controlMode, "total", "UNDECLARED")
}

// TestDroppingAFieldInsideAListTurnsTheGateRed covers the drift nothing before
// this contract could see: cat-33 matched a top-level key, so every key inside
// a list could change and the sweep stayed green.
func TestDroppingAFieldInsideAListTurnsTheGateRed(t *testing.T) {
	live, declared := shippedAndDeclared(t)

	stripped := mcp.MutateResultPayload(live, func(payload map[string]any) bool {
		entries, ok := payload["errors"].([]any)
		if !ok || len(entries) == 0 {
			t.Fatalf("%s returned no error entries, so a list-element control cannot run", controlMode)
		}
		first, ok := entries[0].(map[string]any)
		if !ok {
			t.Fatalf("%s error entries are not objects", controlMode)
		}
		delete(first, "message")
		return true
	})

	assertReports(t, driftOf(t, stripped, declared), controlMode, "errors[].message", "GONE")
}

// TestATypeChangeInTheShippedResponseTurnsTheGateRed covers a same-name field
// whose meaning changed — a count that became a string is a broken caller.
func TestATypeChangeInTheShippedResponseTurnsTheGateRed(t *testing.T) {
	live, declared := shippedAndDeclared(t)

	retyped := mcp.MutateResultPayload(live, func(payload map[string]any) bool {
		payload["count"] = "2"
		return true
	})

	assertReports(t, driftOf(t, retyped, declared), controlMode, "count", "CHANGED TYPE")
}

// assertReports requires a drift naming the mode, the field, and the move.
func assertReports(t *testing.T, drifts []responsecontract.Drift, mode, field, verb string) {
	t.Helper()
	for _, drift := range drifts {
		if drift.Mode == mode && drift.Field == field && strings.Contains(drift.Detail, verb) {
			t.Logf("gate output: %s", drift.Detail)
			return
		}
	}
	t.Fatalf("the gate did not name %s field %q as %s; it reported:\n  %s",
		mode, field, verb, strings.Join(responsecontract.Details(drifts), "\n  "))
}
