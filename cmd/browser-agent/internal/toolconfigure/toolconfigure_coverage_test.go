// toolconfigure_coverage_test.go — Behavior tests for the configure-local handlers:
// jitter, telemetry, security mode, network recording, noise rules, capability
// introspection, and tutorial context.

package toolconfigure

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// fakeDeps is a scriptable Deps that records every mutation so tests can assert
// that read-only calls do not write, and that writes carry the right value.
type fakeDeps struct {
	noiseCfg       *noise.NoiseConfig
	consoleEntries []noise.LogEntry
	networkBodies  []types.NetworkBody
	wsEvents       []capture.WebSocketEvent

	trackingEnabled bool
	trackedTabID    int
	trackedTabURL   string
	pilotStatus     any
	connected       bool
	hasCapture      bool

	tools    []mcp.MCPTool
	examples any

	securityMode     string
	productionParity bool
	rewrites         []string

	telemetryMode string
	jitterMs      int

	// Recorded mutations.
	setSecurityCalls  []securityCall
	setTelemetryCalls []string
	setJitterCalls    []int
}

type securityCall struct {
	mode     string
	rewrites []string
}

func (d *fakeDeps) NoiseConfig() *noise.NoiseConfig              { return d.noiseCfg }
func (d *fakeDeps) ConsoleEntries() []noise.LogEntry             { return d.consoleEntries }
func (d *fakeDeps) NetworkBodies() []types.NetworkBody           { return d.networkBodies }
func (d *fakeDeps) AllWebSocketEvents() []capture.WebSocketEvent { return d.wsEvents }
func (d *fakeDeps) GetTrackingStatus() (bool, int, string) {
	return d.trackingEnabled, d.trackedTabID, d.trackedTabURL
}
func (d *fakeDeps) GetPilotStatus() any        { return d.pilotStatus }
func (d *fakeDeps) IsExtensionConnected() bool { return d.connected }
func (d *fakeDeps) ToolsList() []mcp.MCPTool   { return d.tools }
func (d *fakeDeps) GetToolModuleExamples(string) any {
	return d.examples
}
func (d *fakeDeps) HasCapture() bool { return d.hasCapture }
func (d *fakeDeps) GetSecurityMode() (string, bool, []string) {
	return d.securityMode, d.productionParity, d.rewrites
}
func (d *fakeDeps) SetSecurityMode(mode string, rewrites []string) {
	d.setSecurityCalls = append(d.setSecurityCalls, securityCall{mode, rewrites})
	d.securityMode = mode
	d.rewrites = rewrites
}
func (d *fakeDeps) GetTelemetryMode() string { return d.telemetryMode }
func (d *fakeDeps) SetTelemetryMode(mode string) {
	d.setTelemetryCalls = append(d.setTelemetryCalls, mode)
	d.telemetryMode = mode
}
func (d *fakeDeps) InteractActionSetJitter(ms int) {
	d.setJitterCalls = append(d.setJitterCalls, ms)
	d.jitterMs = ms
}
func (d *fakeDeps) InteractActionGetJitter() int { return d.jitterMs }

func testReq() mcp.JSONRPCRequest {
	return mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: json.RawMessage(`1`)}
}

// toolResult unmarshals a JSONRPCResponse's MCP tool result.
func toolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var r mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("unmarshal tool result: %v (raw=%s)", err, string(resp.Result))
	}
	return r
}

// payload extracts the JSON object appended after the summary line.
func payload(t *testing.T, resp mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	r := toolResult(t, resp)
	if len(r.Content) == 0 {
		t.Fatal("response has no content blocks")
	}
	text := r.Content[0].Text
	i := strings.IndexByte(text, '{')
	if i < 0 {
		t.Fatalf("no JSON payload in %q", text)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(text[i:]), &m); err != nil {
		t.Fatalf("unmarshal payload %q: %v", text[i:], err)
	}
	return m
}

// structuredError decodes the StructuredError from an isError response.
func structuredError(t *testing.T, resp mcp.JSONRPCResponse) mcp.StructuredError {
	t.Helper()
	r := toolResult(t, resp)
	if !r.IsError {
		t.Fatalf("expected an error response, got %s", string(resp.Result))
	}
	text := r.Content[0].Text
	i := strings.IndexByte(text, '{')
	if i < 0 {
		t.Fatalf("no JSON payload in %q", text)
	}
	var se mcp.StructuredError
	if err := json.Unmarshal([]byte(text[i:]), &se); err != nil {
		t.Fatalf("unmarshal structured error %q: %v", text[i:], err)
	}
	return se
}

func summaryLine(t *testing.T, resp mcp.JSONRPCResponse) string {
	t.Helper()
	r := toolResult(t, resp)
	if len(r.Content) == 0 {
		t.Fatal("response has no content blocks")
	}
	return strings.SplitN(r.Content[0].Text, "\n", 2)[0]
}

// ---------------------------------------------------------------------------
// HandleActionJitter
// ---------------------------------------------------------------------------

func TestHandleActionJitter_ClampsToSupportedRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args string
		want float64
	}{
		{"in range", `{"action_jitter_ms":250}`, 250},
		{"zero allowed", `{"action_jitter_ms":0}`, 0},
		{"negative clamped to zero", `{"action_jitter_ms":-1}`, 0},
		{"upper bound accepted", `{"action_jitter_ms":5000}`, 5000},
		{"above bound clamped", `{"action_jitter_ms":999999}`, 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &fakeDeps{}
			resp := HandleActionJitter(d, testReq(), json.RawMessage(tt.args))

			got := payload(t, resp)["action_jitter_ms"]
			if got != tt.want {
				t.Errorf("action_jitter_ms = %v, want %v", got, tt.want)
			}
			if len(d.setJitterCalls) != 1 || float64(d.setJitterCalls[0]) != tt.want {
				t.Errorf("SetJitter calls = %v, want [%v]", d.setJitterCalls, tt.want)
			}
		})
	}
}

func TestHandleActionJitter_OmittedParamIsAReadNotAWrite(t *testing.T) {
	t.Parallel()

	for _, args := range []string{``, `{}`, `{"action_jitter_ms":null}`, `{"other":1}`} {
		d := &fakeDeps{jitterMs: 120}
		resp := HandleActionJitter(d, testReq(), json.RawMessage(args))

		if got := payload(t, resp)["action_jitter_ms"]; got != float64(120) {
			t.Errorf("args=%q: action_jitter_ms = %v, want 120", args, got)
		}
		// Reading the current jitter must never reset it.
		if len(d.setJitterCalls) != 0 {
			t.Errorf("args=%q: jitter was written %v times, want 0", args, len(d.setJitterCalls))
		}
	}
}

func TestHandleActionJitter_MalformedArgsFallBackToRead(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{jitterMs: 75}
	resp := HandleActionJitter(d, testReq(), json.RawMessage(`{"action_jitter_ms":`))

	if toolResult(t, resp).IsError {
		t.Fatal("action_jitter parses leniently; malformed args should not error")
	}
	if got := payload(t, resp)["action_jitter_ms"]; got != float64(75) {
		t.Errorf("action_jitter_ms = %v, want the unchanged 75", got)
	}
	if len(d.setJitterCalls) != 0 {
		t.Errorf("malformed args must not write jitter, got %v", d.setJitterCalls)
	}
}

// ---------------------------------------------------------------------------
// HandleTelemetry
// ---------------------------------------------------------------------------

func TestHandleTelemetry_EmptyModeReadsWithoutWriting(t *testing.T) {
	t.Parallel()

	for _, args := range []string{``, `{}`, `{"telemetry_mode":""}`} {
		d := &fakeDeps{telemetryMode: "auto"}
		resp := HandleTelemetry(d, testReq(), json.RawMessage(args))

		m := payload(t, resp)
		if m["telemetry_mode"] != "auto" || m["status"] != "ok" {
			t.Errorf("args=%q: payload = %v, want status ok / telemetry_mode auto", args, m)
		}
		if len(d.setTelemetryCalls) != 0 {
			t.Errorf("args=%q: reading telemetry must not write it (calls=%v)", args, d.setTelemetryCalls)
		}
		if got := summaryLine(t, resp); got != "Telemetry mode" {
			t.Errorf("args=%q: summary = %q, want %q", args, got, "Telemetry mode")
		}
	}
}

func TestHandleTelemetry_ValidModesArePersistedAndEchoed(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"off", "auto", "full"} {
		d := &fakeDeps{telemetryMode: "auto"}
		resp := HandleTelemetry(d, testReq(), json.RawMessage(`{"telemetry_mode":"`+mode+`"}`))

		if got := payload(t, resp)["telemetry_mode"]; got != mode {
			t.Errorf("telemetry_mode = %v, want %q", got, mode)
		}
		if len(d.setTelemetryCalls) != 1 || d.setTelemetryCalls[0] != mode {
			t.Errorf("SetTelemetryMode calls = %v, want [%q]", d.setTelemetryCalls, mode)
		}
		if got := summaryLine(t, resp); got != "Telemetry mode updated" {
			t.Errorf("summary = %q, want %q", got, "Telemetry mode updated")
		}
	}
}

func TestHandleTelemetry_InvalidModeRejectedWithoutWriting(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"verbose", "ON", "0", "debug"} {
		d := &fakeDeps{telemetryMode: "auto"}
		resp := HandleTelemetry(d, testReq(), json.RawMessage(`{"telemetry_mode":"`+mode+`"}`))

		se := structuredError(t, resp)
		if se.ErrorCode != mcp.ErrInvalidParam {
			t.Errorf("mode=%q: error_code = %q, want %q", mode, se.ErrorCode, mcp.ErrInvalidParam)
		}
		if se.Param != "telemetry_mode" {
			t.Errorf("mode=%q: param = %q, want telemetry_mode", mode, se.Param)
		}
		if !strings.Contains(se.Message, mode) {
			t.Errorf("mode=%q: message %q should quote the rejected value", mode, se.Message)
		}
		if len(d.setTelemetryCalls) != 0 {
			t.Errorf("mode=%q: rejected mode must not be written (calls=%v)", mode, d.setTelemetryCalls)
		}
	}
}

// KNOWN BUG (documented, not fixed here): NormalizeTelemetryMode validates the
// trimmed string but returns the raw one, so a padded mode is stored verbatim.
func TestHandleTelemetry_PaddedModeIsStoredUntrimmed(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{telemetryMode: "auto"}
	resp := HandleTelemetry(d, testReq(), json.RawMessage(`{"telemetry_mode":"  off  "}`))

	if len(d.setTelemetryCalls) != 1 || d.setTelemetryCalls[0] != "  off  " {
		t.Errorf("SetTelemetryMode calls = %q, want the untrimmed \"  off  \"", d.setTelemetryCalls)
	}
	if got := payload(t, resp)["telemetry_mode"]; got != "  off  " {
		t.Errorf("telemetry_mode = %q, want the untrimmed value", got)
	}
}

// ---------------------------------------------------------------------------
// HandleSecurityMode
// ---------------------------------------------------------------------------

func TestHandleSecurityMode_RequiresCaptureSubsystem(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{hasCapture: false}
	se := structuredError(t, HandleSecurityMode(d, testReq(), json.RawMessage(`{"mode":"normal"}`)))

	if se.ErrorCode != mcp.ErrNotInitialized {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, mcp.ErrNotInitialized)
	}
	if len(d.setSecurityCalls) != 0 {
		t.Error("must not change security mode when capture is uninitialized")
	}
}

func TestHandleSecurityMode_EmptyModeReportsCurrentStateWithoutWriting(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{
		hasCapture: true, securityMode: "insecure_proxy",
		productionParity: false, rewrites: []string{"csp_headers"},
	}
	resp := HandleSecurityMode(d, testReq(), json.RawMessage(`{}`))

	m := payload(t, resp)
	if m["security_mode"] != "insecure_proxy" {
		t.Errorf("security_mode = %v, want insecure_proxy", m["security_mode"])
	}
	if m["production_parity"] != false {
		t.Errorf("production_parity = %v, want false", m["production_parity"])
	}
	rewrites, _ := m["insecure_rewrites_applied"].([]any)
	if len(rewrites) != 1 || rewrites[0] != "csp_headers" {
		t.Errorf("insecure_rewrites_applied = %v, want [csp_headers]", m["insecure_rewrites_applied"])
	}
	// Clients gate their confirm prompt on this flag.
	if m["requires_confirmation_for_insecure_mode"] != true {
		t.Errorf("requires_confirmation_for_insecure_mode = %v, want true", m["requires_confirmation_for_insecure_mode"])
	}
	if len(d.setSecurityCalls) != 0 {
		t.Error("reading the security mode must not write it")
	}
}

func TestHandleSecurityMode_NormalClearsRewrites(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"normal", "NORMAL", "  Normal  "} {
		d := &fakeDeps{hasCapture: true, securityMode: "insecure_proxy", rewrites: []string{"csp_headers"}}
		resp := HandleSecurityMode(d, testReq(), json.RawMessage(`{"mode":"`+raw+`"}`))

		m := payload(t, resp)
		if m["security_mode"] != capture.SecurityModeNormal {
			t.Errorf("mode=%q: security_mode = %v, want normal", raw, m["security_mode"])
		}
		if m["production_parity"] != true {
			t.Errorf("mode=%q: production_parity = %v, want true", raw, m["production_parity"])
		}
		if rewrites, _ := m["insecure_rewrites_applied"].([]any); len(rewrites) != 0 {
			t.Errorf("mode=%q: insecure_rewrites_applied = %v, want empty", raw, rewrites)
		}
		if len(d.setSecurityCalls) != 1 || d.setSecurityCalls[0].mode != capture.SecurityModeNormal || d.setSecurityCalls[0].rewrites != nil {
			t.Errorf("mode=%q: SetSecurityMode calls = %+v, want one call (normal, nil)", raw, d.setSecurityCalls)
		}
	}
}

func TestHandleSecurityMode_InsecureProxyRequiresExplicitConfirmation(t *testing.T) {
	t.Parallel()

	for _, args := range []string{`{"mode":"insecure_proxy"}`, `{"mode":"insecure_proxy","confirm":false}`} {
		d := &fakeDeps{hasCapture: true, securityMode: "normal", productionParity: true}
		se := structuredError(t, HandleSecurityMode(d, testReq(), json.RawMessage(args)))

		if se.ErrorCode != mcp.ErrInvalidParam {
			t.Errorf("args=%s: error_code = %q, want %q", args, se.ErrorCode, mcp.ErrInvalidParam)
		}
		if se.Param != "confirm" {
			t.Errorf("args=%s: param = %q, want confirm", args, se.Param)
		}
		// The whole point of the gate: no mutation without acknowledgement.
		if len(d.setSecurityCalls) != 0 {
			t.Errorf("args=%s: security mode was changed without confirmation: %+v", args, d.setSecurityCalls)
		}
	}
}

func TestHandleSecurityMode_ConfirmedInsecureProxyAppliesCSPRewriteAndWarns(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{hasCapture: true, securityMode: "normal", productionParity: true}
	resp := HandleSecurityMode(d, testReq(), json.RawMessage(`{"mode":"INSECURE_PROXY","confirm":true}`))

	m := payload(t, resp)
	if m["security_mode"] != capture.SecurityModeInsecureProxy {
		t.Errorf("security_mode = %v, want insecure_proxy", m["security_mode"])
	}
	if m["production_parity"] != false {
		t.Errorf("production_parity = %v, want false — findings are no longer prod-parity", m["production_parity"])
	}
	rewrites, _ := m["insecure_rewrites_applied"].([]any)
	if len(rewrites) != 1 || rewrites[0] != "csp_headers" {
		t.Errorf("insecure_rewrites_applied = %v, want [csp_headers]", m["insecure_rewrites_applied"])
	}
	warning, _ := m["warning"].(string)
	if !strings.Contains(warning, "Altered environment active") {
		t.Errorf("warning = %q, want the altered-environment notice", warning)
	}
	if len(d.setSecurityCalls) != 1 {
		t.Fatalf("SetSecurityMode calls = %+v, want 1", d.setSecurityCalls)
	}
	call := d.setSecurityCalls[0]
	if call.mode != capture.SecurityModeInsecureProxy || len(call.rewrites) != 1 || call.rewrites[0] != "csp_headers" {
		t.Errorf("SetSecurityMode(%q, %v), want (insecure_proxy, [csp_headers])", call.mode, call.rewrites)
	}
}

func TestHandleSecurityMode_UnknownModeRejected(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{hasCapture: true, securityMode: "normal"}
	se := structuredError(t, HandleSecurityMode(d, testReq(), json.RawMessage(`{"mode":"YOLO"}`)))

	if se.ErrorCode != mcp.ErrInvalidParam || se.Param != "mode" {
		t.Errorf("error = %+v, want invalid_param on 'mode'", se)
	}
	// The raw (uncased) input is echoed so the caller can see what they sent.
	if !strings.Contains(se.Message, "YOLO") {
		t.Errorf("message = %q, want it to quote YOLO", se.Message)
	}
	if len(d.setSecurityCalls) != 0 {
		t.Error("unknown mode must not mutate state")
	}
}

// ---------------------------------------------------------------------------
// HandleNetworkRecording
// ---------------------------------------------------------------------------

type fakeBodyProvider struct{ bodies []types.NetworkBody }

func (f *fakeBodyProvider) GetNetworkBodies() []types.NetworkBody { return f.bodies }

func TestHandleNetworkRecording_MalformedArgsRejected(t *testing.T) {
	t.Parallel()

	state := &NetworkRecordingState{}
	resp := HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(`{"operation":`))

	se := structuredError(t, resp)
	if se.ErrorCode != mcp.ErrInvalidJSON {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, mcp.ErrInvalidJSON)
	}
	if state.Info().Active {
		t.Error("a malformed request must not start a recording")
	}
}

func TestHandleNetworkRecording_StartReportsFiltersAndTimestamp(t *testing.T) {
	t.Parallel()

	state := &NetworkRecordingState{}
	resp := HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(),
		json.RawMessage(`{"operation":"start","domain":"api.example.com","method":"POST"}`))

	m := payload(t, resp)
	if m["status"] != "recording" {
		t.Errorf("status = %v, want recording", m["status"])
	}
	startedAt, _ := m["started_at"].(string)
	if _, err := time.Parse(time.RFC3339, startedAt); err != nil {
		t.Errorf("started_at = %q is not RFC3339: %v", startedAt, err)
	}
	if m["domain_filter"] != "api.example.com" {
		t.Errorf("domain_filter = %v, want api.example.com", m["domain_filter"])
	}
	if m["method_filter"] != "POST" {
		t.Errorf("method_filter = %v, want POST", m["method_filter"])
	}
	if info := state.Info(); !info.Active || info.Domain != "api.example.com" || info.Method != "POST" {
		t.Errorf("state = %+v, want active with both filters", info)
	}
}

func TestHandleNetworkRecording_StartWithoutFiltersOmitsFilterKeys(t *testing.T) {
	t.Parallel()

	state := &NetworkRecordingState{}
	m := payload(t, HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(`{"operation":"start"}`)))

	if _, ok := m["domain_filter"]; ok {
		t.Error("domain_filter must be omitted when no domain filter was given")
	}
	if _, ok := m["method_filter"]; ok {
		t.Error("method_filter must be omitted when no method filter was given")
	}
}

func TestHandleNetworkRecording_SecondStartRejectedAndFirstSessionSurvives(t *testing.T) {
	t.Parallel()

	state := &NetworkRecordingState{}
	HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(`{"operation":"start","domain":"first.example"}`))
	resp := HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(`{"operation":"start","domain":"second.example"}`))

	se := structuredError(t, resp)
	if se.ErrorCode != mcp.ErrInvalidParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, mcp.ErrInvalidParam)
	}
	// The rejected start must not clobber the running session's filters.
	if got := state.Info().Domain; got != "first.example" {
		t.Errorf("domain filter = %q, want first.example", got)
	}
}

func TestHandleNetworkRecording_StopWithoutStartRejected(t *testing.T) {
	t.Parallel()

	state := &NetworkRecordingState{}
	se := structuredError(t, HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(`{"operation":"stop"}`)))

	if se.ErrorCode != mcp.ErrInvalidParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, mcp.ErrInvalidParam)
	}
	if !strings.Contains(se.Message, "No active network recording") {
		t.Errorf("message = %q, want it to say there is no active recording", se.Message)
	}
}

func TestHandleNetworkRecording_StopReturnsOnlyMatchingRequests(t *testing.T) {
	t.Parallel()

	state := &NetworkRecordingState{}
	HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(),
		json.RawMessage(`{"operation":"start","domain":"api.example.com","method":"POST"}`))

	later := time.Now().Add(time.Minute).Format(time.RFC3339Nano)
	earlier := time.Now().Add(-time.Minute).Format(time.RFC3339Nano)
	provider := &fakeBodyProvider{bodies: []types.NetworkBody{
		{Timestamp: later, URL: "https://api.example.com/v1/orders", Method: "POST", Status: 201, RequestBody: `{"id":1}`},
		{Timestamp: later, URL: "https://api.example.com/v1/orders", Method: "GET", Status: 200},   // wrong method
		{Timestamp: later, URL: "https://cdn.other.com/app.js", Method: "POST", Status: 200},       // wrong domain
		{Timestamp: earlier, URL: "https://api.example.com/v1/login", Method: "POST", Status: 200}, // before start
	}}

	resp := HandleNetworkRecording(provider, state, testReq(), json.RawMessage(`{"operation":"stop"}`))
	m := payload(t, resp)

	if m["status"] != "stopped" {
		t.Errorf("status = %v, want stopped", m["status"])
	}
	if m["count"] != float64(1) {
		t.Errorf("count = %v, want 1", m["count"])
	}
	requests, ok := m["requests"].([]any)
	if !ok || len(requests) != 1 {
		t.Fatalf("requests = %v, want exactly the one matching request", m["requests"])
	}
	entry, _ := requests[0].(map[string]any)
	if entry["url"] != "https://api.example.com/v1/orders" || entry["method"] != "POST" {
		t.Errorf("recorded entry = %v, want the POST /v1/orders request", entry)
	}
	if entry["request_body"] != `{"id":1}` {
		t.Errorf("request_body = %v, want the captured body", entry["request_body"])
	}
	if _, ok := m["duration_ms"].(float64); !ok {
		t.Errorf("duration_ms = %v, want a number", m["duration_ms"])
	}
	if state.Info().Active {
		t.Error("state should be inactive after stop")
	}
}

func TestHandleNetworkRecording_StopWithNoMatchesReturnsEmptyArrayNotNull(t *testing.T) {
	t.Parallel()

	state := &NetworkRecordingState{}
	HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(`{"operation":"start"}`))
	m := payload(t, HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(`{"operation":"stop"}`)))

	requests, ok := m["requests"].([]any)
	if !ok {
		t.Fatalf("requests = %v (%T), want [] — null breaks clients that iterate", m["requests"], m["requests"])
	}
	if len(requests) != 0 {
		t.Errorf("requests = %v, want empty", requests)
	}
	if m["count"] != float64(0) {
		t.Errorf("count = %v, want 0", m["count"])
	}
}

func TestHandleNetworkRecording_StatusIsTheDefaultOperation(t *testing.T) {
	t.Parallel()

	for _, args := range []string{``, `{}`, `{"operation":""}`, `{"operation":"status"}`} {
		state := &NetworkRecordingState{}
		m := payload(t, HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(args)))

		if m["active"] != false {
			t.Errorf("args=%q: active = %v, want false", args, m["active"])
		}
		if _, ok := m["started_at"]; ok {
			t.Errorf("args=%q: started_at must be omitted while inactive", args)
		}
		if _, ok := m["duration_ms"]; ok {
			t.Errorf("args=%q: duration_ms must be omitted while inactive", args)
		}
	}
}

func TestHandleNetworkRecording_StatusWhileActiveReportsSession(t *testing.T) {
	t.Parallel()

	state := &NetworkRecordingState{}
	HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(),
		json.RawMessage(`{"operation":"start","domain":"api.example.com","method":"PUT"}`))

	m := payload(t, HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(`{"operation":"status"}`)))
	if m["active"] != true {
		t.Fatalf("active = %v, want true", m["active"])
	}
	if _, err := time.Parse(time.RFC3339, m["started_at"].(string)); err != nil {
		t.Errorf("started_at = %v is not RFC3339", m["started_at"])
	}
	if m["domain_filter"] != "api.example.com" || m["method_filter"] != "PUT" {
		t.Errorf("filters = %v/%v, want api.example.com/PUT", m["domain_filter"], m["method_filter"])
	}
	// A status read must not stop the recording.
	if !state.Info().Active {
		t.Error("status must not end the session")
	}
}

func TestHandleNetworkRecording_UnknownOperationRejected(t *testing.T) {
	t.Parallel()

	state := &NetworkRecordingState{}
	se := structuredError(t, HandleNetworkRecording(&fakeBodyProvider{}, state, testReq(), json.RawMessage(`{"operation":"pause"}`)))

	if se.ErrorCode != mcp.ErrInvalidParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, mcp.ErrInvalidParam)
	}
	if !strings.Contains(se.Message, "pause") {
		t.Errorf("message = %q, want it to quote the unknown operation", se.Message)
	}
	if !strings.Contains(se.RecoveryPlaybook, "start") || !strings.Contains(se.RecoveryPlaybook, "stop") {
		t.Errorf("recovery_playbook = %q, want it to list the valid operations", se.RecoveryPlaybook)
	}
}

// ---------------------------------------------------------------------------
// HandleNoise
// ---------------------------------------------------------------------------

func TestHandleNoise_UninitializedConfigIsAHardError(t *testing.T) {
	t.Parallel()

	se := structuredError(t, HandleNoise(&fakeDeps{noiseCfg: nil}, testReq(), json.RawMessage(`{"action":"list"}`)))
	if se.ErrorCode != mcp.ErrNotInitialized {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, mcp.ErrNotInitialized)
	}
}

func TestHandleNoise_UnknownActionRejected(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"", "delete", "ADD", "auto-detect"} {
		d := &fakeDeps{noiseCfg: noise.NewNoiseConfig()}
		se := structuredError(t, HandleNoise(d, testReq(), json.RawMessage(`{"action":"`+action+`"}`)))

		if se.ErrorCode != mcp.ErrUnknownMode {
			t.Errorf("action=%q: error_code = %q, want %q", action, se.ErrorCode, mcp.ErrUnknownMode)
		}
		if se.Param != "noise_action" {
			t.Errorf("action=%q: param = %q, want noise_action", action, se.Param)
		}
	}
}

func TestHandleNoise_AddAppendsRuleAndReportsTotals(t *testing.T) {
	t.Parallel()

	nc := noise.NewNoiseConfig()
	builtins := len(nc.ListRules())
	d := &fakeDeps{noiseCfg: nc}

	args := `{"action":"add","rules":[
	  {"category":"console","classification":"cosmetic","match_spec":{"message_regex":"^\\[hmr\\]"}},
	  {"category":"network","classification":"infrastructure","match_spec":{"url_regex":"/healthz","status_min":200,"status_max":299}}
	]}`
	m := payload(t, HandleNoise(d, testReq(), json.RawMessage(args)))

	if m["status"] != "ok" {
		t.Errorf("status = %v, want ok", m["status"])
	}
	if m["rules_added"] != float64(2) {
		t.Errorf("rules_added = %v, want 2", m["rules_added"])
	}
	if m["total_rules"] != float64(builtins+2) {
		t.Errorf("total_rules = %v, want %d", m["total_rules"], builtins+2)
	}

	// The match spec must survive the local NoiseRuleArgs -> noise.NoiseRule copy.
	rules := nc.ListRules()
	added := rules[len(rules)-2]
	if added.Category != "console" || added.Classification != "cosmetic" || added.MatchSpec.MessageRegex != `^\[hmr\]` {
		t.Errorf("added console rule = %+v, want the submitted category/classification/regex", added)
	}
	network := rules[len(rules)-1]
	if network.MatchSpec.URLRegex != "/healthz" || network.MatchSpec.StatusMin != 200 || network.MatchSpec.StatusMax != 299 {
		t.Errorf("added network rule match_spec = %+v, want /healthz 200-299", network.MatchSpec)
	}
}

func TestHandleNoise_AddRejectsCatastrophicRegex(t *testing.T) {
	t.Parallel()

	nc := noise.NewNoiseConfig()
	before := len(nc.ListRules())
	d := &fakeDeps{noiseCfg: nc}

	resp := HandleNoise(d, testReq(), json.RawMessage(`{"action":"add","rules":[{"category":"console","match_spec":{"message_regex":"(a+)+"}}]}`))

	se := structuredError(t, resp)
	if se.ErrorCode != mcp.ErrInvalidParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, mcp.ErrInvalidParam)
	}
	if len(nc.ListRules()) != before {
		t.Error("a rejected rule must not be added")
	}
}

func TestHandleNoise_RemoveRequiresRuleID(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{noiseCfg: noise.NewNoiseConfig()}
	se := structuredError(t, HandleNoise(d, testReq(), json.RawMessage(`{"action":"remove"}`)))

	if se.ErrorCode != mcp.ErrMissingParam {
		t.Errorf("error_code = %q, want %q", se.ErrorCode, mcp.ErrMissingParam)
	}
	if se.Param != "rule_id" {
		t.Errorf("param = %q, want rule_id", se.Param)
	}
}

func TestHandleNoise_RemoveDeletesUserRule(t *testing.T) {
	t.Parallel()

	nc := noise.NewNoiseConfig()
	d := &fakeDeps{noiseCfg: nc}
	HandleNoise(d, testReq(), json.RawMessage(`{"action":"add","rules":[{"category":"console","match_spec":{"message_regex":"zzz"}}]}`))

	rules := nc.ListRules()
	id := rules[len(rules)-1].ID
	m := payload(t, HandleNoise(d, testReq(), json.RawMessage(`{"action":"remove","rule_id":"`+id+`"}`)))

	if m["status"] != "ok" || m["removed"] != id {
		t.Errorf("payload = %v, want status ok and removed=%s", m, id)
	}
	for _, r := range nc.ListRules() {
		if r.ID == id {
			t.Fatalf("rule %s still present after remove", id)
		}
	}
}

func TestHandleNoise_RemoveRejectsUnknownAndBuiltinRules(t *testing.T) {
	t.Parallel()

	nc := noise.NewNoiseConfig()
	builtinID := ""
	for _, r := range nc.ListRules() {
		if strings.HasPrefix(r.ID, "builtin_") {
			builtinID = r.ID
			break
		}
	}
	if builtinID == "" {
		t.Fatal("expected at least one builtin_ rule to exist")
	}
	before := len(nc.ListRules())
	d := &fakeDeps{noiseCfg: nc}

	for _, id := range []string{"user_9999", builtinID} {
		se := structuredError(t, HandleNoise(d, testReq(), json.RawMessage(`{"action":"remove","rule_id":"`+id+`"}`)))
		if se.ErrorCode != mcp.ErrInvalidParam {
			t.Errorf("id=%q: error_code = %q, want %q", id, se.ErrorCode, mcp.ErrInvalidParam)
		}
	}
	if len(nc.ListRules()) != before {
		t.Error("failed removals must leave the rule set untouched")
	}
}

func TestHandleNoise_ListReturnsRulesAndSnakeCaseStatistics(t *testing.T) {
	t.Parallel()

	nc := noise.NewNoiseConfig()
	d := &fakeDeps{noiseCfg: nc}
	HandleNoise(d, testReq(), json.RawMessage(`{"action":"add","rules":[{"category":"console","classification":"cosmetic","match_spec":{"message_regex":"zzz-marker"}}]}`))

	m := payload(t, HandleNoise(d, testReq(), json.RawMessage(`{"action":"list"}`)))

	rules, ok := m["rules"].([]any)
	if !ok || len(rules) != len(nc.ListRules()) {
		t.Fatalf("rules = %T len %d, want %d entries", m["rules"], len(rules), len(nc.ListRules()))
	}
	found := false
	for _, raw := range rules {
		r, _ := raw.(map[string]any)
		spec, _ := r["match_spec"].(map[string]any)
		if spec != nil && spec["message_regex"] == "zzz-marker" {
			found = true
			if r["classification"] != "cosmetic" {
				t.Errorf("classification = %v, want cosmetic", r["classification"])
			}
			if id, _ := r["id"].(string); !strings.HasPrefix(id, "user_") {
				t.Errorf("id = %v, want a user_ prefixed id", r["id"])
			}
		}
	}
	if !found {
		t.Error("the rule added in this test is missing from the list output")
	}

	stats, ok := m["statistics"].(map[string]any)
	if !ok {
		t.Fatalf("statistics = %T, want an object", m["statistics"])
	}
	for _, key := range []string{"total_filtered", "per_rule", "last_signal_at", "last_noise_at"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("statistics missing snake_case key %q (got %v)", key, stats)
		}
	}
}

func TestHandleNoise_ResetDropsUserRulesOnly(t *testing.T) {
	t.Parallel()

	nc := noise.NewNoiseConfig()
	builtins := len(nc.ListRules())
	d := &fakeDeps{noiseCfg: nc}
	HandleNoise(d, testReq(), json.RawMessage(`{"action":"add","rules":[{"category":"console","match_spec":{"message_regex":"zzz"}}]}`))

	m := payload(t, HandleNoise(d, testReq(), json.RawMessage(`{"action":"reset"}`)))

	if m["total_rules"] != float64(builtins) {
		t.Errorf("total_rules = %v, want the %d built-ins", m["total_rules"], builtins)
	}
	if m["message"] != "Reset to built-in rules only" {
		t.Errorf("message = %v, want the built-ins-only notice", m["message"])
	}
	for _, r := range nc.ListRules() {
		if strings.HasPrefix(r.ID, "user_") {
			t.Fatalf("user rule %s survived reset", r.ID)
		}
	}
}

func TestHandleNoise_AutoDetectAppliesHighConfidenceProposals(t *testing.T) {
	t.Parallel()

	nc := noise.NewNoiseConfig()
	builtins := len(nc.ListRules())

	// 30 repeats gives confidence 0.99, above the 0.9 auto-apply threshold.
	entries := make([]noise.LogEntry, 0, 30)
	for i := 0; i < 30; i++ {
		entries = append(entries, noise.LogEntry{"message": "zzz-marker-app-log", "source": "app.js"})
	}
	d := &fakeDeps{noiseCfg: nc, consoleEntries: entries}

	m := payload(t, HandleNoise(d, testReq(), json.RawMessage(`{"action":"auto_detect"}`)))

	if m["proposals_count"] != float64(1) {
		t.Fatalf("proposals_count = %v, want 1 (payload=%v)", m["proposals_count"], m)
	}
	proposals, _ := m["proposals"].([]any)
	if len(proposals) != 1 {
		t.Fatalf("proposals = %v, want 1", proposals)
	}
	p, _ := proposals[0].(map[string]any)
	if conf, _ := p["confidence"].(float64); conf < 0.9 {
		t.Errorf("confidence = %v, want >= 0.9", conf)
	}
	if reason, _ := p["reason"].(string); !strings.Contains(reason, "repeated 30 times") {
		t.Errorf("reason = %q, want it to cite the repeat count", reason)
	}
	// >= 0.9 proposals are applied immediately, which is what the message promises.
	if m["total_rules"] != float64(builtins+1) {
		t.Errorf("total_rules = %v, want %d — a high-confidence proposal should be auto-applied", m["total_rules"], builtins+1)
	}
	if !strings.Contains(m["message"].(string), "auto-applied") {
		t.Errorf("message = %v, want it to mention auto-application", m["message"])
	}
}

func TestHandleNoise_AutoDetectWithNoDataProposesNothing(t *testing.T) {
	t.Parallel()

	nc := noise.NewNoiseConfig()
	builtins := len(nc.ListRules())
	d := &fakeDeps{noiseCfg: nc}

	m := payload(t, HandleNoise(d, testReq(), json.RawMessage(`{"action":"auto_detect"}`)))

	if m["proposals_count"] != float64(0) {
		t.Errorf("proposals_count = %v, want 0", m["proposals_count"])
	}
	if m["total_rules"] != float64(builtins) {
		t.Errorf("total_rules = %v, want %d", m["total_rules"], builtins)
	}
}

// ---------------------------------------------------------------------------
// HandleDescribeCapabilities
// ---------------------------------------------------------------------------

// fixtureTools uses names that are absent from the production mode-spec table so
// the assertions below pin the generic schema-derived path, not curated specs.
func fixtureTools() []mcp.MCPTool {
	return []mcp.MCPTool{
		{
			Name:        "zeta",
			Description: "Zeta tool",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"what"},
				"properties": map[string]any{
					"what":  map[string]any{"type": "string", "enum": []string{"errors", "logs"}},
					"limit": map[string]any{"type": "integer"},
				},
			},
		},
		{
			Name:        "alpha",
			Description: "Alpha tool",
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"what"},
				"properties": map[string]any{
					"what": map[string]any{"type": "string", "enum": []string{"go"}},
				},
			},
		},
	}
}

func TestHandleDescribeCapabilities_ModeWithoutToolIsRejected(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: fixtureTools()}
	se := structuredError(t, HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{"mode":"errors"}`), "1.2.3"))

	if se.ErrorCode != mcp.ErrInvalidParam || se.Param != "tool" {
		t.Errorf("error = %+v, want invalid_param on 'tool'", se)
	}
}

func TestHandleDescribeCapabilities_UnknownToolListsValidNamesSorted(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: fixtureTools()}
	se := structuredError(t, HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{"tool":"nope"}`), "1.2.3"))

	if se.ErrorCode != mcp.ErrInvalidParam || se.Param != "tool" {
		t.Errorf("error = %+v, want invalid_param on 'tool'", se)
	}
	if se.Hint != "Valid tools: alpha, zeta" {
		t.Errorf("hint = %q, want the tool names sorted alphabetically", se.Hint)
	}
	if !strings.Contains(se.Message, "nope") {
		t.Errorf("message = %q, want it to quote the unknown tool", se.Message)
	}
}

func TestHandleDescribeCapabilities_UnknownModeListsValidModes(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: fixtureTools()}
	se := structuredError(t, HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{"tool":"zeta","mode":"bogus"}`), "1.2.3"))

	if se.ErrorCode != mcp.ErrInvalidParam || se.Param != "mode" {
		t.Errorf("error = %+v, want invalid_param on 'mode'", se)
	}
	if se.Hint != "Valid modes: errors, logs" {
		t.Errorf("hint = %q, want the tool's enum modes", se.Hint)
	}
}

func TestHandleDescribeCapabilities_ToolWithoutModesStillErrorsCleanly(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: []mcp.MCPTool{{
		Name:        "modeless",
		Description: "No dispatch param",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}},
	}}}
	se := structuredError(t, HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{"tool":"modeless","mode":"anything"}`), "1.2.3"))

	if se.ErrorCode != mcp.ErrInvalidParam || se.Param != "mode" {
		t.Errorf("error = %+v, want invalid_param on 'mode'", se)
	}
	if se.Hint != "Valid modes: " {
		t.Errorf("hint = %q, want an empty mode list rather than a panic", se.Hint)
	}
}

func TestHandleDescribeCapabilities_ToolFilterReturnsSingleToolAndVersion(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: fixtureTools()}
	m := payload(t, HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{"tool":"zeta"}`), "9.9.9"))

	if m["version"] != "9.9.9" {
		t.Errorf("version = %v, want the injected 9.9.9", m["version"])
	}
	// Pinned MCP protocol revision — clients branch on it.
	if m["protocol_version"] != "2024-11-05" {
		t.Errorf("protocol_version = %v, want 2024-11-05", m["protocol_version"])
	}
	tools, _ := m["tools"].(map[string]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v, want only the requested tool", tools)
	}
	zeta, _ := tools["zeta"].(map[string]any)
	if zeta["dispatch_param"] != "what" {
		t.Errorf("dispatch_param = %v, want what", zeta["dispatch_param"])
	}
	modes, _ := zeta["modes"].([]any)
	if len(modes) != 2 || modes[0] != "errors" || modes[1] != "logs" {
		t.Errorf("modes = %v, want [errors logs] in schema order", modes)
	}
	params, _ := zeta["params"].([]any)
	if len(params) != 1 || params[0] != "limit" {
		t.Errorf("params = %v, want [limit] (the dispatch param is excluded)", params)
	}
	if _, ok := m["examples"]; ok {
		t.Error("examples must be omitted when the tool module has none")
	}
}

func TestHandleDescribeCapabilities_ExamplesIncludedWhenAvailable(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: fixtureTools(), examples: []any{map[string]any{"goal": "read errors"}}}
	m := payload(t, HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{"tool":"zeta"}`), "1.0.0"))

	examples, ok := m["examples"].([]any)
	if !ok || len(examples) != 1 {
		t.Fatalf("examples = %v, want the module examples passed through", m["examples"])
	}
	first, _ := examples[0].(map[string]any)
	if first["goal"] != "read errors" {
		t.Errorf("examples[0] = %v, want the injected example", first)
	}
}

func TestHandleDescribeCapabilities_ToolAndModeReturnsFlatModeEntry(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: fixtureTools()}
	m := payload(t, HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{"tool":"zeta","mode":"errors"}`), "1.0.0"))

	if m["tool"] != "zeta" || m["mode"] != "errors" {
		t.Errorf("payload = %v, want tool=zeta mode=errors", m)
	}
	required, _ := m["required"].([]any)
	if len(required) != 1 || required[0] != "what" {
		t.Errorf("required = %v, want [what]", required)
	}
	optional, _ := m["optional"].([]any)
	if len(optional) != 1 || optional[0] != "limit" {
		t.Errorf("optional = %v, want [limit]", optional)
	}
	// The per-mode response is deliberately flat — no version envelope, so the
	// payload stays small enough to be worth requesting per mode.
	if _, ok := m["version"]; ok {
		t.Error("per-mode response must not carry the version envelope")
	}
	if _, ok := m["params"].(map[string]any); !ok {
		t.Errorf("params = %v, want a param-details object", m["params"])
	}
}

func TestHandleDescribeCapabilities_SummaryOmitsPerParamDetail(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: fixtureTools()}
	m := payload(t, HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{"summary":true}`), "1.0.0"))

	tools, _ := m["tools"].(map[string]any)
	if len(tools) != 2 {
		t.Fatalf("tools = %v, want both tools", tools)
	}
	zeta, _ := tools["zeta"].(map[string]any)
	if zeta["description"] != "Zeta tool" || zeta["dispatch_param"] != "what" {
		t.Errorf("summary entry = %v, want description + dispatch_param", zeta)
	}
	// In summary form modes is a mode -> hint index, not a list.
	modes, ok := zeta["modes"].(map[string]any)
	if !ok {
		t.Fatalf("summary modes = %T, want an object keyed by mode", zeta["modes"])
	}
	if _, ok := modes["errors"]; !ok {
		t.Errorf("summary modes = %v, want an 'errors' key", modes)
	}
	if _, ok := zeta["param_details"]; ok {
		t.Error("summary must omit param_details — that is the whole point of summary=true")
	}
}

func TestHandleDescribeCapabilities_FullResponseCarriesDetailAndDeprecatedList(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: fixtureTools()}
	m := payload(t, HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{}`), "2.0.0"))

	if m["version"] != "2.0.0" || m["protocol_version"] != "2024-11-05" {
		t.Errorf("envelope = %v, want version 2.0.0 and protocol 2024-11-05", m)
	}
	deprecated, ok := m["deprecated"].([]any)
	if !ok || len(deprecated) != 0 {
		t.Errorf("deprecated = %v, want an empty array", m["deprecated"])
	}
	tools, _ := m["tools"].(map[string]any)
	zeta, _ := tools["zeta"].(map[string]any)
	if _, ok := zeta["param_details"].(map[string]any); !ok {
		t.Errorf("param_details = %v, want an object in the full response", zeta["param_details"])
	}
	modeParams, _ := zeta["mode_params"].(map[string]any)
	if _, ok := modeParams["errors"]; !ok {
		t.Errorf("mode_params = %v, want a per-mode entry for errors", modeParams)
	}
}

func TestHandleDescribeCapabilities_MalformedArgsFallBackToFullResponse(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{tools: fixtureTools()}
	resp := HandleDescribeCapabilities(d, testReq(), json.RawMessage(`{"tool":`), "1.0.0")

	if toolResult(t, resp).IsError {
		t.Fatal("capabilities parses leniently; malformed args should return the full catalog")
	}
	tools, _ := payload(t, resp)["tools"].(map[string]any)
	if len(tools) != 2 {
		t.Errorf("tools = %v, want the full catalog", tools)
	}
}

// ---------------------------------------------------------------------------
// TutorialContext / TutorialIssues / TutorialNextSteps
// ---------------------------------------------------------------------------

func TestTutorialContext_NilDepsReturnsOptimisticDefaults(t *testing.T) {
	t.Parallel()

	ctx := TutorialContext(nil)
	want := map[string]any{
		"pilot_enabled": true, "pilot_state": "assumed_enabled", "pilot_authoritative": false,
		"extension_connected": false, "tracking_enabled": false,
		"tracked_tab_id": 0, "tracked_tab_url": "",
	}
	for key, wantVal := range want {
		if ctx[key] != wantVal {
			t.Errorf("ctx[%q] = %v, want %v", key, ctx[key], wantVal)
		}
	}
}

func TestTutorialContext_ReadsPilotAndTrackingFromDeps(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{
		connected: true, trackingEnabled: true, trackedTabID: 7, trackedTabURL: "https://app.test/",
		pilotStatus: map[string]any{"enabled": false, "state": "explicitly_disabled", "authoritative": true},
	}
	ctx := TutorialContext(d)

	want := map[string]any{
		"pilot_enabled": false, "pilot_state": "explicitly_disabled", "pilot_authoritative": true,
		"extension_connected": true, "tracking_enabled": true,
		"tracked_tab_id": 7, "tracked_tab_url": "https://app.test/",
	}
	for key, wantVal := range want {
		if ctx[key] != wantVal {
			t.Errorf("ctx[%q] = %v, want %v", key, ctx[key], wantVal)
		}
	}
}

func TestTutorialContext_UnusablePilotStatusKeepsDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status any
	}{
		{"nil status", nil},
		{"wrong type", "enabled"},
		{"empty map", map[string]any{}},
		{"wrong field types", map[string]any{"enabled": "yes", "state": 3, "authoritative": 1}},
		{"blank state string", map[string]any{"state": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := TutorialContext(&fakeDeps{pilotStatus: tt.status, connected: true})
			if ctx["pilot_enabled"] != true || ctx["pilot_state"] != "assumed_enabled" || ctx["pilot_authoritative"] != false {
				t.Errorf("pilot fields = %v/%v/%v, want the assumed-enabled defaults",
					ctx["pilot_enabled"], ctx["pilot_state"], ctx["pilot_authoritative"])
			}
			// Non-pilot fields still come from deps.
			if ctx["extension_connected"] != true {
				t.Errorf("extension_connected = %v, want true", ctx["extension_connected"])
			}
		})
	}
}

func issueCodes(issues []map[string]any) []string {
	out := make([]string, 0, len(issues))
	for _, iss := range issues {
		code, _ := iss["code"].(string)
		out = append(out, code)
	}
	return out
}

func TestTutorialIssues_ReportsHighestPriorityBlockerOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  map[string]any
		want []string
	}{
		{
			// Pilot being off outranks everything else: the other warnings would
			// be noise while interact cannot run at all.
			name: "explicitly disabled pilot outranks a disconnected extension",
			ctx: map[string]any{
				"pilot_enabled": true, "pilot_state": "explicitly_disabled",
				"extension_connected": false, "tracking_enabled": false,
				"tracked_tab_id": 0, "tracked_tab_url": "",
			},
			want: []string{"pilot_disabled"},
		},
		{
			name: "pilot disabled with no state string",
			ctx: map[string]any{
				"pilot_enabled": false, "pilot_state": "",
				"extension_connected": true, "tracking_enabled": true,
				"tracked_tab_id": 1, "tracked_tab_url": "https://a.test/",
			},
			want: []string{"pilot_disabled"},
		},
		{
			name: "disconnected extension outranks a missing tracked tab",
			ctx: map[string]any{
				"pilot_enabled": true, "pilot_state": "enabled",
				"extension_connected": false, "tracking_enabled": false,
				"tracked_tab_id": 0, "tracked_tab_url": "",
			},
			want: []string{"extension_disconnected"},
		},
		{
			name: "tracking off",
			ctx: map[string]any{
				"pilot_enabled": true, "pilot_state": "enabled",
				"extension_connected": true, "tracking_enabled": false,
				"tracked_tab_id": 5, "tracked_tab_url": "https://a.test/",
			},
			want: []string{"no_tracked_tab"},
		},
		{
			name: "tracking on but no tab id",
			ctx: map[string]any{
				"pilot_enabled": true, "pilot_state": "enabled",
				"extension_connected": true, "tracking_enabled": true,
				"tracked_tab_id": 0, "tracked_tab_url": "https://a.test/",
			},
			want: []string{"no_tracked_tab"},
		},
		{
			name: "tracking on but no url",
			ctx: map[string]any{
				"pilot_enabled": true, "pilot_state": "enabled",
				"extension_connected": true, "tracking_enabled": true,
				"tracked_tab_id": 5, "tracked_tab_url": "",
			},
			want: []string{"no_tracked_tab"},
		},
		{
			name: "fully healthy",
			ctx: map[string]any{
				"pilot_enabled": true, "pilot_state": "enabled",
				"extension_connected": true, "tracking_enabled": true,
				"tracked_tab_id": 5, "tracked_tab_url": "https://a.test/",
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := TutorialIssues(tt.ctx)
			got := issueCodes(issues)
			if len(got) != len(tt.want) {
				t.Fatalf("issues = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("issues = %v, want %v", got, tt.want)
				}
			}
			for _, iss := range issues {
				if iss["severity"] != "warning" {
					t.Errorf("issue %v severity = %v, want warning", iss["code"], iss["severity"])
				}
				if fix, _ := iss["fix"].(string); fix == "" {
					t.Errorf("issue %v has no fix instruction", iss["code"])
				}
				if ex, _ := iss["example"].(string); ex == "" {
					t.Errorf("issue %v has no example call", iss["code"])
				}
			}
		})
	}
}

func TestTutorialIssues_HealthyContextReturnsNonNilEmptySlice(t *testing.T) {
	t.Parallel()

	// json.Marshal turns a nil slice into null; the tutorial payload promises [].
	issues := TutorialIssues(map[string]any{
		"pilot_enabled": true, "pilot_state": "enabled",
		"extension_connected": true, "tracking_enabled": true,
		"tracked_tab_id": 5, "tracked_tab_url": "https://a.test/",
	})
	if issues == nil {
		t.Fatal("TutorialIssues returned nil, want an empty non-nil slice")
	}
	raw, _ := json.Marshal(issues)
	if string(raw) != "[]" {
		t.Errorf("marshalled issues = %s, want []", raw)
	}
}

func TestTutorialIssues_EmptyContextReportsPilotDisabled(t *testing.T) {
	t.Parallel()

	// Missing keys read as zero values, so pilot_enabled=false + pilot_state=""
	// trips the pilot branch. A caller that hand-builds a partial context gets
	// "pilot_disabled" rather than a silent all-clear — worth knowing before
	// anyone starts passing contexts that did not come from TutorialContext.
	got := issueCodes(TutorialIssues(map[string]any{}))
	if len(got) != 1 || got[0] != "pilot_disabled" {
		t.Errorf("issues = %v, want [pilot_disabled]", got)
	}
}

func TestTutorialNextSteps_SwitchesOnIssuePresence(t *testing.T) {
	t.Parallel()

	broken := TutorialNextSteps(map[string]any{"extension_connected": false})
	if len(broken) != 3 || broken[0] != "Run configure doctor to verify environment status" {
		t.Errorf("next steps with issues = %v, want the doctor-first list", broken)
	}

	healthy := TutorialNextSteps(map[string]any{
		"pilot_enabled": true, "pilot_state": "enabled",
		"extension_connected": true, "tracking_enabled": true,
		"tracked_tab_id": 5, "tracked_tab_url": "https://a.test/",
	})
	if len(healthy) != 3 || healthy[0] != "Run observe errors to inspect current page issues" {
		t.Errorf("next steps when healthy = %v, want the observe-first list", healthy)
	}
}

// ---------------------------------------------------------------------------
// HandleTutorial
// ---------------------------------------------------------------------------

func TestHandleTutorial_ModeReflectsRequestedWhat(t *testing.T) {
	t.Parallel()

	tests := []struct{ args, want string }{
		{`{"what":"examples"}`, "examples"},
		{`{"what":"tutorial"}`, "tutorial"},
		{`{"what":"anything_else"}`, "tutorial"},
		{`{}`, "tutorial"},
		{``, "tutorial"},
	}
	for _, tt := range tests {
		m := payload(t, HandleTutorial(&fakeDeps{}, testReq(), json.RawMessage(tt.args), nil))
		if m["mode"] != tt.want {
			t.Errorf("args=%q: mode = %v, want %q", tt.args, m["mode"], tt.want)
		}
		if m["status"] != "ok" {
			t.Errorf("args=%q: status = %v, want ok", tt.args, m["status"])
		}
	}
}

func TestHandleTutorial_EmbedsContextIssuesAndInjectedPlaybooks(t *testing.T) {
	t.Parallel()

	d := &fakeDeps{connected: false, trackingEnabled: false}
	injected := map[string]any{
		"element_not_found": map[string]any{"retry_guidance": "list_interactive then retry"},
	}
	m := payload(t, HandleTutorial(d, testReq(), json.RawMessage(`{"what":"tutorial"}`), injected))

	ctx, _ := m["context"].(map[string]any)
	if ctx["extension_connected"] != false {
		t.Errorf("context.extension_connected = %v, want false", ctx["extension_connected"])
	}
	issues, _ := m["issues"].([]any)
	if len(issues) != 1 {
		t.Fatalf("issues = %v, want the extension_disconnected warning", issues)
	}
	if issues[0].(map[string]any)["code"] != "extension_disconnected" {
		t.Errorf("issue code = %v, want extension_disconnected", issues[0])
	}

	// Failure playbooks are injected by the caller (they live in the playbooks
	// sub-package); the handler must pass them through untouched.
	pb, _ := m["failure_recovery_playbooks"].(map[string]any)
	entry, _ := pb["element_not_found"].(map[string]any)
	if entry["retry_guidance"] != "list_interactive then retry" {
		t.Errorf("failure_recovery_playbooks = %v, want the injected map", pb)
	}

	snippets, _ := m["snippets"].([]any)
	if len(snippets) != len(TutorialSnippets()) {
		t.Errorf("snippets = %d entries, want %d", len(snippets), len(TutorialSnippets()))
	}
	if loop, _ := m["safe_automation_loop"].(map[string]any); loop["title"] != "Deterministic safe automation loop" {
		t.Errorf("safe_automation_loop.title = %v", loop["title"])
	}
	if csp, _ := m["csp_fallback_playbook"].(map[string]any); csp["exact_retry_guidance"] != CSPRetryNavigationGuidance {
		t.Errorf("csp_fallback_playbook.exact_retry_guidance = %v, want the shared constant", csp["exact_retry_guidance"])
	}
	if bp, _ := m["best_practices"].([]any); len(bp) != 4 {
		t.Errorf("best_practices = %v, want 4 entries", bp)
	}
	nextSteps, _ := m["next_steps"].([]any)
	if len(nextSteps) != 3 || nextSteps[0] != "Run configure doctor to verify environment status" {
		t.Errorf("next_steps = %v, want the doctor-first list for a disconnected extension", nextSteps)
	}
}

func TestHandleTutorial_NilPlaybooksSerializeAsNull(t *testing.T) {
	t.Parallel()

	m := payload(t, HandleTutorial(&fakeDeps{}, testReq(), json.RawMessage(`{}`), nil))
	if v, ok := m["failure_recovery_playbooks"]; !ok || v != nil {
		t.Errorf("failure_recovery_playbooks = %v (present=%v), want a null placeholder", v, ok)
	}
}

// ---------------------------------------------------------------------------
// Tutorial playbook content contracts
// ---------------------------------------------------------------------------

func TestTutorialCSPFallbackPlaybook_ExposesDetectionSignalsAgentsMatchOn(t *testing.T) {
	t.Parallel()

	pb := TutorialCSPFallbackPlaybook()
	signals, _ := pb["detect_signals"].([]string)
	want := []string{"error=csp_blocked_all_worlds", "failure_cause=csp", "csp_blocked=true"}
	if len(signals) != len(want) {
		t.Fatalf("detect_signals = %v, want %v", signals, want)
	}
	for i := range want {
		if signals[i] != want[i] {
			t.Errorf("detect_signals = %v, want %v", signals, want)
			break
		}
	}
	if pb["exact_retry_guidance"] != CSPRetryNavigationGuidance {
		t.Errorf("exact_retry_guidance = %v, want the CSPRetryNavigationGuidance constant", pb["exact_retry_guidance"])
	}
	seq, _ := pb["fallback_sequence"].([]map[string]any)
	if len(seq) != 4 {
		t.Fatalf("fallback_sequence = %v, want 4 ordered steps", seq)
	}
	for i, step := range seq {
		if step["step"] != i+1 {
			t.Errorf("fallback_sequence[%d].step = %v, want %d", i, step["step"], i+1)
		}
	}
}

func TestTutorialSafeAutomationLoop_StepsAreNumberedInOrder(t *testing.T) {
	t.Parallel()

	pb := TutorialSafeAutomationLoop()
	steps, _ := pb["steps"].([]map[string]any)
	wantNames := []string{
		"scope_selection", "list_interactive_in_scope", "candidate_verification",
		"action_execution", "post_action_verification",
	}
	if len(steps) != len(wantNames) {
		t.Fatalf("steps = %d, want %d", len(steps), len(wantNames))
	}
	for i, step := range steps {
		if step["step"] != i+1 {
			t.Errorf("steps[%d].step = %v, want %d", i, step["step"], i+1)
		}
		if step["name"] != wantNames[i] {
			t.Errorf("steps[%d].name = %v, want %q", i, step["name"], wantNames[i])
		}
	}

	scenarios, _ := pb["scenarios"].([]map[string]any)
	ids := map[string]bool{}
	for _, s := range scenarios {
		id, _ := s["id"].(string)
		ids[id] = true
	}
	for _, want := range []string{"multi_dialog", "iframe", "csp_restricted_page"} {
		if !ids[want] {
			t.Errorf("scenarios missing %q; got %v", want, ids)
		}
	}
}

// ---------------------------------------------------------------------------
// MatchesRecordingFilter — timestamp formats
// ---------------------------------------------------------------------------

func TestMatchesRecordingFilter_AcceptsEpochMillisTimestamps(t *testing.T) {
	t.Parallel()

	start := time.Now()
	tests := []struct {
		name      string
		timestamp string
		want      bool
	}{
		{"epoch millis after start", strconv.FormatInt(start.Add(time.Minute).UnixMilli(), 10), true},
		{"epoch millis before start", strconv.FormatInt(start.Add(-time.Minute).UnixMilli(), 10), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The extension sends numeric timestamps on some paths; treating those
			// as unparseable would let pre-recording traffic into every snapshot.
			body := types.NetworkBody{Timestamp: tt.timestamp, URL: "https://a.test/x", Method: "GET"}
			if got := MatchesRecordingFilter(body, start, "", ""); got != tt.want {
				t.Errorf("MatchesRecordingFilter(ts=%q) = %v, want %v", tt.timestamp, got, tt.want)
			}
		})
	}
}

func TestMatchesRecordingFilter_UnparseableTimestampIsIncluded(t *testing.T) {
	t.Parallel()

	// Best-effort: an unrecognised timestamp must not silently drop the request.
	for _, ts := range []string{"yesterday", "2026-13-45", "12:00:00"} {
		body := types.NetworkBody{Timestamp: ts, URL: "https://a.test/x", Method: "GET"}
		if !MatchesRecordingFilter(body, time.Now(), "", "") {
			t.Errorf("MatchesRecordingFilter(ts=%q) = false, want true (include when the format is unknown)", ts)
		}
	}
}
