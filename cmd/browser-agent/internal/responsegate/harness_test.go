// harness_test.go — Produces REAL MCP responses in-process so the declared
// response shapes are DERIVED, never hand-typed.
//
// PURPOSE: a hand-written field list drifts away from the handler that produces
// it — that is exactly why the 32 regexes in mode-content-expectations.sh became the
// only written statement of what any MCP response contains. Every shape frozen
// in .mcp-response-contract.json is derived by composing the SHIPPED five-tool
// runtime (toolruntime.NewToolHandler, the production composition root) over a
// seeded in-memory fixture and calling it through the same HandleToolCall entry
// point an MCP client reaches, so renaming a field in a handler changes the
// derived shape on the next run and the gate goes red.
//
// NO DAEMON, NO BROWSER. A mode whose answer can only come from a live
// extension produces no shape here and falls to the undeclared ratchet instead.
// Recording a degraded "extension not connected" reply as the contract would
// pin the wrong shape forever. refusal_test.go proves every refusal is one of
// those, not an oversight in this fixture.
//
// WHY THIS PACKAGE LIVES HERE and not beside the gate in
// scripts/contracts/responsecontract: Go forbids anything outside
// cmd/browser-agent from importing cmd/browser-agent/internal/**, and the
// composition root now lives in cmd/browser-agent/internal/toolruntime. This
// package is test-only and holds no production symbol; the contract library and
// the ratchet live in scripts/contracts/responsecontract with the other gates.
//
// Docs: docs/features/feature/quality-gates/index.md
package responsegate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/asynccommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcptelemetry"
	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/intent"
	terminalstatus "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/status"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/activecodebase"
	annotationruntime "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation/runtime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/listenport"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/warningqueue"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/contracts/responsecontract"
)

// fixtureVersion is the build string the fixture daemon state reports. A shape
// carries no values, so the number is irrelevant — but it must not be empty, or
// the modes that omit an empty version would declare a field short.
const fixtureVersion = "0.0.0-fixture"

// fixtureNow pins the clock for the seeded records. A shape carries no values,
// but a handler that branches on freshness would otherwise pick a different
// branch per run.
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
	handler  *toolruntime.ToolHandler
	async    *asynccommand.Handler
	request  mcp.JSONRPCRequest
	cleanup  func()
}

// newHarness composes the shipped five-tool runtime over a freshly seeded
// fixture rooted in stateRoot. Callers must close it.
func newHarness(t *testing.T, stateRoot string) *harness {
	t.Helper()
	captured := capture.NewCapture()
	fixture := &harness{
		captured: captured,
		async:    asynccommand.New(asynccommand.Deps{Capture: captured}),
		request:  mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: "tools/call"},
	}
	state, closeState := fixtureState(t, stateRoot)
	fixture.cleanup = closeState
	fixture.seed(state)
	fixture.handler = toolruntime.NewToolHandler(state, captured)
	return fixture
}

func (h *harness) close() {
	h.handler.Close()
	h.captured.Close()
	h.cleanup()
}

// fixtureState builds the daemon-owned stores the runtime reads. Every store is
// the production type: a stub would let the fixture's own payload be frozen as
// the product's contract.
func fixtureState(t *testing.T, stateRoot string) (toolruntime.ServerState, func()) {
	t.Helper()
	dir, err := os.MkdirTemp(stateRoot, "fixture")
	if err != nil {
		t.Fatalf("create fixture state directory: %v", err)
	}
	warnings := warningqueue.New()
	logs := logstore.New(logstore.Config{
		LogFile:       filepath.Join(dir, "logs.jsonl"),
		MaxEntries:    1000,
		TelemetryMode: mcptelemetry.ModeAuto,
		AddWarning:    warnings.Add,
	})
	annotations := annotationruntime.New(10 * time.Minute)
	state := toolruntime.ServerState{
		Version:            fixtureVersion,
		Runtime:            appruntime.New(fixtureVersion),
		SessionProjectPath: dir,
		Logs:               logs,
		Warnings:           warnings,
		Incidents:          incident.NewStore(100, nil),
		PushInbox:          push.NewPushInbox(50),
		ActiveCodebase:     activecodebase.New(),
		AnnotationRuntime:  annotations,
		ListenPort:         listenport.New(),
		TerminalStatus:     terminalstatus.New(),
		IntentStore:        terminalintent.NewStore(),
		StateRecovery:      statediag.NewCollector(),
	}
	return state, annotations.Close
}

// fixtureLogEntries is the console buffer every log-reading mode pages over.
// It is not empty on purpose: omitempty and "no data" branches drop fields, and
// a shape captured over an empty buffer would declare a contract several fields
// short of the one a caller with real data receives.
func fixtureLogEntries() []types.LogEntry {
	stamp := fixtureNow.UTC().Format(time.RFC3339)
	// Both "ts" and "timestamp" are set: the error projection reads "ts" and
	// the log projection reads "timestamp", and a key the projection cannot
	// find is declared as a null-typed field that says nothing.
	base := types.LogEntry{
		"source": "console", "url": "https://fixture.test/",
		"ts": stamp, "timestamp": stamp, "tabId": 7, "line": 12, "column": 4,
	}
	return []types.LogEntry{
		merged(base, types.LogEntry{"level": "error", "sequence": 1,
			"message": "ReferenceError: fixture is not defined", "stack": "at fixture.js:1"}),
		merged(base, types.LogEntry{"level": "warn", "sequence": 2, "message": "deprecated API in use"}),
		merged(base, types.LogEntry{"level": "log", "sequence": 3, "message": "fixture ready"}),
	}
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

// seed fills the buffers the responses read. Empty buffers are not good enough:
// omitempty drops cursor, newest_timestamp and oldest_timestamp when there is
// nothing to page over, so a shape captured over an empty store would declare a
// contract three fields short of the one callers actually receive.
func (h *harness) seed(state toolruntime.ServerState) {
	state.Logs.AddEntries(fixtureLogEntries())
	telemetry := h.captured.Telemetry()
	telemetry.AddNetworkBodies([]types.NetworkBody{{URL: "https://fixture.test/api", Method: "GET", Status: 200}})
	telemetry.AddEnhancedActions([]types.EnhancedAction{{
		Type: "click", URL: "https://fixture.test/", Timestamp: fixtureNow.UnixMilli(),
		Selectors: map[string]any{"css": "#fixture"}, Source: "human",
		Classification: "toast", Role: "alert",
	}})
	// observe(network_waterfall) asks the extension for a refresh when its newest
	// entry is over a second old, and then waits 5s for an answer that cannot
	// arrive here. A buffer stamped at seed time keeps the handler on the
	// no-refresh branch, which is the branch a caller with live traffic is on.
	telemetry.NetworkWaterfall().Add([]types.NetworkWaterfallEntry{{
		Name: "api", URL: "https://fixture.test/api", InitiatorType: "fetch",
		Duration: 12, TransferSize: 512, Status: 200, Timestamp: time.Now(),
	}}, "https://fixture.test/")
	h.seedCommandLifecycle()
}

// Correlation ids for the three command lifecycle states the fixture plants.
const (
	seededCorrelationID  = "dom_fixture_correlation"
	pendingCorrelationID = "pending_fixture_correlation"
	failedCorrelationID  = "failed_fixture_correlation"
)

// seedCommandLifecycle plants one command in each terminal and non-terminal
// state. observe(pending_commands) reports all three lists and omits any that
// is empty, so a fixture with only a completed command would declare nothing
// about a pending or failed entry — the two a caller most needs to recognise.
func (h *harness) seedCommandLifecycle() {
	commands := h.captured.Queries()
	commands.RegisterCommand(seededCorrelationID, "query_fixture", time.Minute)
	commands.ApplyCommandResult(
		seededCorrelationID, "complete", json.RawMessage(`{"elements":[{"tag":"div"}],"count":1}`), "")

	// Created through the production enqueue path so the command carries the
	// query_id a hand-registered one omits.
	for _, planted := range []struct {
		correlationID string
		status        string
	}{
		{pendingCorrelationID, ""},
		{failedCorrelationID, "error"},
	} {
		if _, err := commands.CreatePendingQueryWithTimeout(queries.PendingQuery{
			Type:          "dom",
			Params:        json.RawMessage(`{"selector":"#fixture"}`),
			CorrelationID: planted.correlationID,
		}, time.Hour, ""); err != nil {
			panic("seed a " + planted.correlationID + " command: " + err.Error())
		}
		if planted.status != "" {
			commands.ApplyCommandResult(planted.correlationID, planted.status, nil, "fixture failure")
		}
	}
}

// extraArgs supplies the arguments a mode needs before it can produce a payload
// at all. It is deliberately small, and every entry is an argument a real caller
// supplies: an argument invented here that the handler then echoes back would
// put the fixture's own input into the product's contract.
var extraArgs = map[string]map[string]any{
	// The resolved command the fixture planted, so the mode answers with a real
	// lifecycle envelope instead of "no such correlation id".
	"observe/command_result": {"correlation_id": seededCorrelationID},
	// Both annotation readers block for 15s waiting on a draw session that can
	// never arrive here. timeout_ms is the caller's own bound on that wait, and
	// the empty-result branch it lands on is the same one the 15s wait reaches.
	"observe/annotations": {"timeout_ms": 1},
	"analyze/annotations": {"timeout_ms": 1},
	// Sub-action selectors. Each of these modes is a small dispatcher of its
	// own: with no sub-action it answers "which one?", and with one it runs
	// entirely against daemon-local state.
	"analyze/api_validation":        {"operation": "report"},
	"configure/streaming":           {"streaming_action": "status"},
	"configure/test_boundary_start": {"test_id": "response-contract-fixture"},
	"configure/test_boundary_end":   {"test_id": "response-contract-fixture"},
	// Narrowing arguments. These modes answer a QUESTION, and the shape of the
	// answer depends on which question: describe_capabilities returns the whole
	// five-tool catalog when unnarrowed and one mode's parameter list when
	// narrowed, and qa_fixture returns a transaction list for "status" and a
	// verdict for "validate". The arguments below are the ones cat-33 already
	// drives these modes with (scripts/tests/browser/cat-33-connected-action-
	// coverage.sh), so the frozen shape and the sweep's expectation describe the
	// same call rather than two different ones.
	"configure/describe_capabilities": {"tool": "interact", "mode": "click"},
	"configure/qa_fixture": {
		"fixture_action": "validate",
		"fixture":        map[string]any{"version": 1},
	},
	"configure/save_sequence": {
		"name":  "response-contract-fixture",
		"steps": []any{map[string]any{"what": "navigate", "url": "https://fixture.test/"}},
	},
}

// argsFor renders the argument document one mode is invoked with.
func argsFor(mode string) string {
	document := map[string]any{"what": modeName(mode)}
	for key, value := range extraArgs[mode] {
		document[key] = value
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// modeName strips the "tool/" prefix from a contract key.
func modeName(mode string) string {
	if index := strings.IndexByte(mode, '/'); index >= 0 {
		return mode[index+1:]
	}
	return mode
}

// answer invokes one mode through the same entry point an MCP client reaches.
// A panic is reported as a refusal rather than crashing the sweep, so one
// unanswerable mode cannot hide the shapes of the other 170.
func (h *harness) answer(tool, args string) (response mcp.JSONRPCResponse, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("the handler panicked: %v", recovered)
		}
	}()
	response, handled := h.handler.HandleToolCall(h.request, tool, json.RawMessage(args))
	if !handled {
		return mcp.JSONRPCResponse{}, fmt.Errorf("the tool catalog does not dispatch %q", tool)
	}
	return response, nil
}

// refusalRecord is one mode that produced no declarable shape, and why.
type refusalRecord struct {
	kind   string
	detail string
}

// sweepResult is the whole five-tool sweep: the shapes it derived and the
// classified reason for every mode it could not.
type sweepResult struct {
	shapes   map[string]responsecontract.Shape
	cases    map[string]modeCase
	refusals map[string]refusalRecord
}

var (
	sweepOnce   sync.Once
	sweepShared *sweepResult
)

// sweep runs the sweep exactly once per test binary and shares the result. Each
// run composes the runtime and invokes ~175 modes; repeating it per test would
// multiply that cost by the number of tests for identical answers. The shared
// value is read-only — mcp.MutateResultPayload returns a new response rather
// than editing the one it is given, so the drift controls cannot corrupt it.
func sweep(t *testing.T) *sweepResult {
	t.Helper()
	sweepOnce.Do(func() { sweepShared = runSweep(t) })
	if sweepShared == nil {
		t.Fatal("the sweep produced no result, so every check over it would pass vacuously")
	}
	return sweepShared
}

// runSweep invokes every dispatchable mode and derives a shape from each
// response that carries a real payload.
//
// EVERY MODE GETS ITS OWN FIXTURE. Sharing one would make a declared shape
// depend on what ran before it: a browser-mediated mode leaves a command
// pending in the store observe(pending_commands) reports, and a handler that
// logs leaves an entry in the buffer observe(summarized_logs) analyses. Both
// shapes changed between freezes while one fixture was shared. A fresh fixture
// per mode costs about 11ms and makes the frozen contract a function of the
// handlers alone.
func runSweep(t *testing.T) *sweepResult {
	t.Helper()
	stateRoot := t.TempDir()
	result := &sweepResult{
		shapes:   map[string]responsecontract.Shape{},
		cases:    map[string]modeCase{},
		refusals: map[string]refusalRecord{},
	}

	registry, envelope := sweepRegistry(t, stateRoot)
	for _, tool := range sortedTools(registry) {
		for _, mode := range registry[tool] {
			key := tool + "/" + mode
			if reason, blocked := notDriveable[key]; blocked {
				result.refusals[key] = refusalRecord{kind: refusedNotDriveable, detail: reason}
				continue
			}
			result.invoke(t, stateRoot, tool, key)
		}
	}
	result.record(responsecontract.EnvelopeQueued, envelope)
	return result
}

// sweepRegistry reads the dispatchable mode set and the async envelope off one
// throwaway fixture, so neither costs a composition inside the loop.
func sweepRegistry(t *testing.T, stateRoot string) (map[string][]string, modeCase) {
	t.Helper()
	fixture := newHarness(t, stateRoot)
	defer fixture.close()
	return fixture.handler.DispatchableModes(), fixture.envelopeCase()
}

func sortedTools(registry map[string][]string) []string {
	tools := make([]string, 0, len(registry))
	for tool := range registry {
		tools = append(tools, tool)
	}
	sort.Strings(tools)
	return tools
}

// toolCall is one tool invocation: which tool, and the exact argument document.
type toolCall struct {
	tool string
	args string
}

// precondition names the calls a mode's contract genuinely depends on, run on
// its own fixture before the mode itself. A real caller makes the same call
// first: configure(event_recording_stop) answers "nothing is recording" until a
// recording has been started, so with no precondition the sweep would freeze
// that refusal instead of the mode's actual response.
var precondition = map[string][]toolCall{
	"configure/event_recording_stop": {{
		tool: "configure",
		args: `{"what":"event_recording_start","name":"response-contract-fixture"}`,
	}},
}

// invoke runs one mode against a fixture nothing else has touched.
func (r *sweepResult) invoke(t *testing.T, stateRoot, tool, key string) {
	t.Helper()
	fixture := newHarness(t, stateRoot)
	defer fixture.close()

	for _, setup := range precondition[key] {
		if _, err := fixture.answer(setup.tool, setup.args); err != nil {
			r.refusals[key] = refusalRecord{
				kind:   refusedUnexplained,
				detail: "precondition " + setup.args + " failed: " + err.Error(),
			}
			return
		}
	}

	args := argsFor(key)
	response, err := fixture.answer(tool, args)
	if err != nil {
		r.refusals[key] = refusalRecord{kind: refusedUnexplained, detail: err.Error()}
		return
	}
	r.record(key, modeCase{mode: key, args: args, response: response})
}

// record derives the shape of one answered mode, or files a classified refusal.
func (r *sweepResult) record(mode string, answered modeCase) {
	r.cases[mode] = answered
	shape, err := responsecontract.ShapeOfResponse(answered.response)
	if err != nil {
		r.refusals[mode] = refusalRecord{
			kind:   classifyRefusal(answered.response),
			detail: err.Error() + " :: " + responseText(answered.response),
		}
		return
	}
	r.shapes[mode] = shape
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
