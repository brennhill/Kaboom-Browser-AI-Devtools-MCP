// Purpose: Coverage tests for the GET /telemetry dispatcher in telemetry.go.
// Docs: docs/features/feature/mcp-persistent-server/index.md
//
// Every assertion here checks the *payload* (type echo, count, item identity),
// not just the status code: a handler that returned 200 with an empty body
// would satisfy a status-only test while serving nothing.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// lifecovTelemetryCall issues GET /telemetry?<query> against the handler and
// decodes the JSON body. Returns the status and the decoded object.
func lifecovTelemetryCall(t *testing.T, h http.HandlerFunc, query string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/telemetry?"+query, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	var body map[string]any
	if rr.Body.Len() > 0 {
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("telemetry?%s: body is not JSON (%v): %s", query, err, rr.Body.String())
		}
	}
	return rr.Code, body
}

// lifecovTelemetryFixture builds a server+capture pair pre-loaded with exactly
// three entries in every buffer /telemetry can serve, so count assertions
// distinguish "served the right buffer" from "served an empty one".
func lifecovTelemetryFixture(t *testing.T) (*Server, *capture.Store) {
	t.Helper()
	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)

	srv.logs.addEntries([]LogEntry{
		{"message": "log-1"}, {"message": "log-2"}, {"message": "log-3"},
	})
	cap.AddNetworkWaterfallEntries([]capture.NetworkWaterfallEntry{
		{URL: "https://example.test/wf-1"},
		{URL: "https://example.test/wf-2"},
		{URL: "https://example.test/wf-3"},
	}, "https://example.test")
	cap.AddNetworkBodies([]capture.NetworkBody{
		{URL: "https://example.test/body-1", Status: 200},
		{URL: "https://example.test/body-2", Status: 200},
		{URL: "https://example.test/body-3", Status: 200},
	})
	cap.AddWebSocketEvents([]capture.WebSocketEvent{
		{Event: "open", URL: "wss://example.test/ws", ID: "ws-1"},
		{Event: "message", URL: "wss://example.test/ws", ID: "ws-1", Direction: "incoming", Data: "a"},
		{Event: "message", URL: "wss://example.test/ws", ID: "ws-1", Direction: "outgoing", Data: "b"},
	})
	cap.AddEnhancedActions([]capture.EnhancedAction{
		{Type: "click", URL: "https://example.test", Timestamp: 1},
		{Type: "input", URL: "https://example.test", Timestamp: 2},
		{Type: "submit", URL: "https://example.test", Timestamp: 3},
	})
	cap.AddPerformanceSnapshots([]performance.PerformanceSnapshot{
		{URL: "https://example.test/p1"},
		{URL: "https://example.test/p2"},
		{URL: "https://example.test/p3"},
	})
	cap.AddExtensionLogs([]types.ExtensionLog{
		{Level: "info", Message: "ext-1", Source: "background", Timestamp: time.Now()},
		{Level: "warn", Message: "ext-2", Source: "background", Timestamp: time.Now()},
		{Level: "error", Message: "ext-3", Source: "content", Timestamp: time.Now()},
	})
	return srv, cap
}

func TestTelemetryEndpoint_RejectsNonGET(t *testing.T) {
	t.Parallel()

	srv, cap := lifecovTelemetryFixture(t)
	h := handleTelemetry(srv, cap)

	req := httptest.NewRequest(http.MethodPost, "/telemetry?type=logs", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /telemetry status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
	// The 405 must short-circuit before any buffer is read: a body here would
	// mean a rejected method still leaked telemetry.
	if rr.Body.Len() != 0 {
		t.Errorf("POST /telemetry returned a body on 405: %q", rr.Body.String())
	}
}

func TestTelemetryEndpoint_MissingTypeIsRejectedWithHint(t *testing.T) {
	t.Parallel()

	srv, cap := lifecovTelemetryFixture(t)
	code, body := lifecovTelemetryCall(t, handleTelemetry(srv, cap), "")

	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", code, http.StatusBadRequest)
	}
	if got, _ := body["error"].(string); got != "Missing required 'type' parameter" {
		t.Errorf("error = %q, want the missing-type message", got)
	}
	hint, _ := body["hint"].(string)
	for _, want := range []string{"logs", "network_waterfall", "websocket_status"} {
		if !strings.Contains(hint, want) {
			t.Errorf("hint %q does not mention valid type %q", hint, want)
		}
	}
}

func TestTelemetryEndpoint_UnknownTypeEchoesTheRejectedValue(t *testing.T) {
	t.Parallel()

	srv, cap := lifecovTelemetryFixture(t)
	code, body := lifecovTelemetryCall(t, handleTelemetry(srv, cap), "type=not_a_buffer")

	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", code, http.StatusBadRequest)
	}
	// Echoing the value is what makes the error actionable for a typo.
	if got, _ := body["error"].(string); got != "Unknown telemetry type: not_a_buffer" {
		t.Errorf("error = %q, want it to name the rejected type", got)
	}
	if _, served := body["items"]; served {
		t.Error("unknown type returned an items payload; it must serve nothing")
	}
}

func TestTelemetryEndpoint_EachTypeServesItsOwnBuffer(t *testing.T) {
	t.Parallel()

	srv, cap := lifecovTelemetryFixture(t)
	h := handleTelemetry(srv, cap)

	// Every buffer was seeded with exactly 3 entries, so a handler that
	// dispatched `actions` to the logs buffer would still report 3. The
	// per-type item probes below are what actually pin the dispatch.
	for _, telType := range []string{
		"logs", "network_waterfall", "network_bodies", "websocket_events",
		"actions", "performance_snapshots", "extension_logs",
	} {
		t.Run(telType, func(t *testing.T) {
			code, body := lifecovTelemetryCall(t, h, "type="+telType)
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body: %v)", code, body)
			}
			if got, _ := body["type"].(string); got != telType {
				t.Errorf("type = %q, want %q", got, telType)
			}
			count, ok := body["count"].(float64)
			if !ok {
				t.Fatalf("count missing or not a number: %v", body["count"])
			}
			if int(count) != 3 {
				t.Errorf("count = %d, want 3 seeded entries", int(count))
			}
			items, ok := body["items"].([]any)
			if !ok {
				t.Fatalf("items missing or not an array: %v", body["items"])
			}
			if len(items) != int(count) {
				t.Errorf("len(items) = %d but count = %d; the two must agree", len(items), int(count))
			}
		})
	}
}

func TestTelemetryEndpoint_ActionsAndLogsAreDistinctBuffers(t *testing.T) {
	t.Parallel()

	srv, cap := lifecovTelemetryFixture(t)
	h := handleTelemetry(srv, cap)

	// Pins the switch arms against each other: if `actions` fell through to the
	// logs buffer (or vice versa) the counts would still be 3, but the payload
	// shape would swap.
	_, actionsBody := lifecovTelemetryCall(t, h, "type=actions")
	firstAction, _ := actionsBody["items"].([]any)
	if len(firstAction) == 0 {
		t.Fatal("actions returned no items")
	}
	action, _ := firstAction[0].(map[string]any)
	if _, hasType := action["type"]; !hasType {
		t.Errorf("actions item has no `type` field, so it is not an EnhancedAction: %v", action)
	}

	_, logsBody := lifecovTelemetryCall(t, h, "type=logs")
	logItems, _ := logsBody["items"].([]any)
	if len(logItems) == 0 {
		t.Fatal("logs returned no items")
	}
	logEntry, _ := logItems[0].(map[string]any)
	if got, _ := logEntry["message"].(string); got != "log-1" {
		t.Errorf("first log message = %q, want %q (logs buffer, oldest first)", got, "log-1")
	}
}

func TestTelemetryEndpoint_LimitKeepsTheNewestEntries(t *testing.T) {
	t.Parallel()

	srv, cap := lifecovTelemetryFixture(t)
	h := handleTelemetry(srv, cap)

	code, body := lifecovTelemetryCall(t, h, "type=logs&limit=2")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if got, _ := body["count"].(float64); int(got) != 2 {
		t.Fatalf("count = %d, want 2", int(got))
	}
	items, _ := body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	// entries[len-limit:] — the TAIL. A regression to entries[:limit] would
	// silently start serving the oldest entries and this is the only assertion
	// that would catch it.
	first, _ := items[0].(map[string]any)
	last, _ := items[1].(map[string]any)
	if got, _ := first["message"].(string); got != "log-2" {
		t.Errorf("items[0].message = %q, want %q (newest 2 of 3)", got, "log-2")
	}
	if got, _ := last["message"].(string); got != "log-3" {
		t.Errorf("items[1].message = %q, want %q", got, "log-3")
	}
}

func TestTelemetryEndpoint_NonPositiveAndMalformedLimitsAreIgnored(t *testing.T) {
	t.Parallel()

	srv, cap := lifecovTelemetryFixture(t)
	h := handleTelemetry(srv, cap)

	// limit is parsed with `err == nil && v > 0`; anything else must leave the
	// full buffer intact rather than truncating to zero.
	for _, limit := range []string{"0", "-1", "abc", "", "9999"} {
		t.Run("limit="+limit, func(t *testing.T) {
			code, body := lifecovTelemetryCall(t, h, "type=logs&limit="+limit)
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			if got, _ := body["count"].(float64); int(got) != 3 {
				t.Errorf("count = %d, want all 3 entries for limit=%q", int(got), limit)
			}
		})
	}
}

func TestTelemetryEndpoint_WebSocketStatusUsesConnectionShapeNotItems(t *testing.T) {
	t.Parallel()

	srv, cap := lifecovTelemetryFixture(t)
	code, body := lifecovTelemetryCall(t, handleTelemetry(srv, cap), "type=websocket_status")

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %v)", code, body)
	}
	// websocket_status returns early with its own envelope; the generic
	// {items,count} envelope must NOT appear or clients would parse it wrong.
	if _, hasItems := body["items"]; hasItems {
		t.Error("websocket_status returned an `items` key; it must use connections/closed")
	}
	conns, ok := body["connections"].([]any)
	if !ok {
		t.Fatalf("connections missing or not an array: %v", body["connections"])
	}
	if len(conns) != 1 {
		t.Fatalf("connections len = %d, want the 1 opened socket", len(conns))
	}
	if got, _ := body["count"].(float64); int(got) != len(conns) {
		t.Errorf("count = %d, want len(connections) = %d", int(got), len(conns))
	}
	if _, hasClosed := body["closed"]; !hasClosed {
		t.Error("websocket_status omitted the `closed` key")
	}
}

func TestTelemetryEndpoint_WebSocketStatusIgnoresLimit(t *testing.T) {
	t.Parallel()

	srv, cap := lifecovTelemetryFixture(t)
	// websocket_status returns before the limit is applied. Documented here so
	// a future change that starts honouring it is a deliberate decision.
	_, body := lifecovTelemetryCall(t, handleTelemetry(srv, cap), "type=websocket_status&limit=0")
	if got, _ := body["count"].(float64); int(got) != 1 {
		t.Errorf("count = %d, want 1 (limit is not applied to websocket_status)", int(got))
	}
}
