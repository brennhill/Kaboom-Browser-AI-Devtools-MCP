// harness_test.go — Produces REAL MCP responses in-process so the declared
// response shapes are DERIVED, never hand-typed.
//
// PURPOSE: a hand-written field list drifts away from the handler that produces
// it — that is exactly why the 32 regexes in mode-content-expectations.sh became the
// only written statement of what any MCP response contains. Every shape frozen
// in .mcp-response-contract.json is derived by invoking the shipped dispatcher
// over a seeded in-memory fixture, so renaming a field in a handler changes the
// derived shape on the next run and the gate goes red.
//
// NO DAEMON, NO BROWSER. A mode whose answer can only come from a live
// extension produces no shape here and falls to the undeclared ratchet instead.
// Recording a degraded "extension not connected" reply as the contract would
// pin the wrong shape forever — the failure the snapshot harness in
// scripts/contracts/response/ calls out in capitals.
//
// WHY THIS PACKAGE LIVES HERE and not beside the gate in
// scripts/contracts/responsecontract: Go forbids anything outside
// cmd/browser-agent from importing cmd/browser-agent/internal/**, and the
// shipped observe dispatcher and async lifecycle owner both live there. This
// package is test-only and holds no production symbol; the contract library and
// the ratchet live in scripts/contracts/responsecontract with the other gates.
//
// Docs: docs/features/feature/quality-gates/index.md
package responsegate

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asynccommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolobserve"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	observecore "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/contracts/responsecontract"
)

// seededCorrelationID names the resolved command the fixture plants so that
// observe(command_result) answers with a real resolved lifecycle envelope.
const seededCorrelationID = "dom_fixture_correlation"

// fixtureNow pins the clock. A shape carries no values, but a handler that
// branches on freshness would otherwise pick a different branch per run.
var fixtureNow = time.Unix(1700000000, 0).UTC()

// modeCase is one invocable mode and the response it produced.
type modeCase struct {
	mode     string
	args     string
	response mcp.JSONRPCResponse
}

// harness owns the seeded in-memory state every case runs against.
type harness struct {
	captured *capture.Capture
	observe  *toolobserve.Dispatcher
	async    *asynccommand.Handler
	request  mcp.JSONRPCRequest
}

// newHarness builds the fixture. Callers must close it.
func newHarness() *harness {
	captured := capture.NewCapture()
	h := &harness{
		captured: captured,
		async:    asynccommand.New(asynccommand.Deps{Capture: captured}),
		request:  mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: "tools/call"},
	}
	h.seed()
	h.observe = toolobserve.NewDispatcher(toolobserve.Config{
		Observe:              observeDeps(captured),
		IsExtensionConnected: func() bool { return true },
		Commands:             captured.Queries(),
		InjectSummary:        func(args json.RawMessage) json.RawMessage { return args },
		DrainAlerts:          func() []types.Alert { return nil },
		FormatCommand:        h.async.FormatCommandResult,
	})
	return h
}

func (h *harness) close() { h.captured.Close() }

// fixtureLogEntries is the console buffer every log-reading mode pages over.
// It is not empty on purpose: omitempty and "no data" branches drop fields, and
// a shape captured over an empty buffer would declare a contract several fields
// short of the one a caller with real data receives.
func fixtureLogEntries() ([]types.LogEntry, []time.Time) {
	stamp := fixtureNow.UTC().Format(time.RFC3339)
	// Both "ts" and "timestamp" are set: the error projection reads "ts" and
	// the log projection reads "timestamp", and a key the projection cannot
	// find is declared as a null-typed field that says nothing.
	base := types.LogEntry{
		"source": "console", "url": "https://fixture.test/",
		"ts": stamp, "timestamp": stamp, "tabId": 7, "line": 12, "column": 4,
	}
	entries := []types.LogEntry{
		merged(base, types.LogEntry{"level": "error", "sequence": 1,
			"message": "ReferenceError: fixture is not defined", "stack": "at fixture.js:1"}),
		merged(base, types.LogEntry{"level": "warn", "sequence": 2, "message": "deprecated API in use"}),
		merged(base, types.LogEntry{"level": "log", "sequence": 3, "message": "fixture ready"}),
	}
	stamps := []time.Time{fixtureNow, fixtureNow, fixtureNow}
	return entries, stamps
}

// merged copies base and overlays the per-entry fields.
func merged(base, overlay types.LogEntry) types.LogEntry {
	entry := types.LogEntry{}
	for key, value := range base {
		entry[key] = value
	}
	for key, value := range overlay {
		entry[key] = value
	}
	return entry
}

// observeDeps supplies the deterministic dependency set observe modes read.
func observeDeps(captured *capture.Capture) observecore.Deps {
	return observecore.Deps{
		Capture:          captured,
		LogEntries:       fixtureLogEntries,
		LogTotalAdded:    func() int64 { return 3 },
		IsConsoleNoise:   func(types.LogEntry) bool { return false },
		ExecuteA11yQuery: func(string, []string, any, bool) (json.RawMessage, error) { return nil, nil },
		// A zero timeout takes the production default, which parks the sweep on
		// an extension refresh that will never answer here.
		WaterfallRefreshTimeout: 10 * time.Millisecond,
		DiagnosticHintString:    func() string { return "run configure({what:'doctor'})" },
		Now:                     func() time.Time { return fixtureNow },
	}
}

// seed fills the buffers the responses read. Empty buffers are not good enough:
// omitempty drops cursor, newest_timestamp and oldest_timestamp when there is
// nothing to page over, so a shape captured over an empty store would declare a
// contract three fields short of the one callers actually receive.
func (h *harness) seed() {
	telemetry := h.captured.Telemetry()
	telemetry.AddNetworkBodies([]types.NetworkBody{{URL: "https://fixture.test/api", Method: "GET", Status: 200}})
	telemetry.AddEnhancedActions([]types.EnhancedAction{{
		Type: "click", URL: "https://fixture.test/", Timestamp: fixtureNow.UnixMilli(),
		Selectors: map[string]any{"css": "#fixture"}, Source: "human",
		Classification: "toast", Role: "alert",
	}})
	h.captured.Queries().RegisterCommand(seededCorrelationID, "query_fixture", time.Minute)
	h.captured.Queries().ApplyCommandResult(
		seededCorrelationID, "complete", json.RawMessage(`{"elements":[{"tag":"div"}],"count":1}`), "")
}

// observeArgs is the argument document each observe mode is invoked with.
func observeArgs(mode string) string {
	if mode == "command_result" {
		return `{"what":"command_result","correlation_id":"` + seededCorrelationID + `"}`
	}
	return `{"what":"` + mode + `"}`
}

// answer invokes one mode. A mode the composition root owns (its handler is
// injected by cmd/browser-agent and is nil here) panics inside the router; that
// is reported as a refusal so the mode falls to the undeclared ratchet. It is
// NOT stubbed: a shape derived from a stub would declare the test's payload as
// the product's contract, which is worse than declaring nothing.
func (h *harness) answer(args string) (response mcp.JSONRPCResponse, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("not answerable outside the composition root (%v)", recovered)
		}
	}()
	return h.observe.Handle(h.request, json.RawMessage(args)), nil
}

// cases returns every mode this harness can answer plus the async envelope.
// The SET is derived too: the modes come from the shipped dispatcher's own
// registry, so a mode added to observe cannot be silently left out of the sweep.
func (h *harness) cases() ([]modeCase, map[string]string) {
	modes := h.observe.ValidModes()
	cases := make([]modeCase, 0, len(modes)+1)
	refused := map[string]string{}
	for _, mode := range modes {
		args := observeArgs(mode)
		response, err := h.answer(args)
		if err != nil {
			refused["observe/"+mode] = err.Error()
			continue
		}
		cases = append(cases, modeCase{mode: "observe/" + mode, args: args, response: response})
	}
	return append(cases, h.envelopeCase()), refused
}

// envelopeCase captures the async lifecycle envelope the production async owner
// mints for a browser-mediated mode that has not resolved yet. Declaring it is
// the point: the dual shape — daemon-local modes answer with their payload,
// browser-mediated modes answer with an envelope whose answer nests under
// .result — was folklore, which is why cat-33's analyze/feature_gates
// expectation matched only "result" and proved nothing about the payload.
func (h *harness) envelopeCase() modeCase {
	args := `{"background":true}`
	return modeCase{
		mode: responsecontract.EnvelopeQueued,
		args: args,
		response: h.async.MaybeWaitForCommand(
			h.request, seededCorrelationID, json.RawMessage(args), "DOM query queued"),
	}
}

// declarableShapes derives a shape for every case that answered with a real
// payload, and reports the modes that did not as undeclarable.
func (h *harness) declarableShapes() (map[string]responsecontract.Shape, map[string]string) {
	shapes := map[string]responsecontract.Shape{}
	cases, refused := h.cases()
	for _, testCase := range cases {
		shape, err := responsecontract.ShapeOfResponse(testCase.response)
		if err != nil {
			refused[testCase.mode] = err.Error()
			continue
		}
		shapes[testCase.mode] = shape
	}
	return shapes, refused
}
