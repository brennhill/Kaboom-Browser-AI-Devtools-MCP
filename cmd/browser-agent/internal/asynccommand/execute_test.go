// execute_test.go — Tests for create-and-wait probes and terminal timeout/polling responses.

package asynccommand

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// answerFirstQuery plays the extension: it waits for the first queued query of
// the given type and answers it, returning the params the daemon sent.
func answerFirstQuery(t *testing.T, captured *capture.Capture, queryType string, result json.RawMessage) <-chan json.RawMessage {
	t.Helper()
	sent := make(chan json.RawMessage, 1)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			for _, pending := range captured.Queries().GetPendingQueries() {
				if pending.Type == queryType {
					sent <- pending.Params
					captured.Queries().SetQueryResult(pending.ID, result)
					return
				}
			}
			time.Sleep(time.Millisecond)
		}
		close(sent)
	}()
	return sent
}

func TestExecuteA11yQueryRoundTripsThroughQueue(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	defer captured.Close()
	sent := answerFirstQuery(t, captured, "a11y", json.RawMessage(`{"violations":[]}`))

	got, err := New(Deps{Capture: captured}).ExecuteA11yQuery("#main", []string{"wcag2a"}, nil, true)
	if err != nil || string(got) != `{"violations":[]}` {
		t.Fatalf("ExecuteA11yQuery = %s, %v", got, err)
	}
	var params map[string]any
	if err := json.Unmarshal(<-sent, &params); err != nil || params["scope"] != "#main" || params["force_refresh"] != true {
		t.Fatalf("a11y params = %#v (%v)", params, err)
	}
}

func TestExecuteStyleProbeRoundTripsThroughQueue(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	defer captured.Close()
	sent := answerFirstQuery(t, captured, "computed_styles", json.RawMessage(`{"elements":[]}`))

	got, err := New(Deps{Capture: captured}).ExecuteStyleProbe("button", 7, true)
	if err != nil || string(got) != `{"elements":[]}` {
		t.Fatalf("ExecuteStyleProbe = %s, %v", got, err)
	}
	var params map[string]any
	if err := json.Unmarshal(<-sent, &params); err != nil ||
		params["selector"] != "button" || params["max_elements"] != float64(7) || params["include_custom_properties"] != true {
		t.Fatalf("style params = %#v (%v)", params, err)
	}
}

func TestExecuteProbesFailLoudWhenQueueIsFull(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	defer captured.Close()
	saturateQueue(t, captured)
	handler := New(Deps{Capture: captured})

	if _, err := handler.ExecuteA11yQuery("", nil, nil, false); !errors.Is(err, queries.ErrQueueFull) {
		t.Fatalf("ExecuteA11yQuery err = %v, want ErrQueueFull", err)
	}
	if _, err := handler.ExecuteStyleProbe("p", 0, false); !errors.Is(err, queries.ErrQueueFull) {
		t.Fatalf("ExecuteStyleProbe err = %v, want ErrQueueFull", err)
	}
}

func TestFormatCommandResultTimeoutExplainsDisconnectedExtension(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	defer captured.Close()
	result := decodeToolResult(t, New(Deps{Capture: captured, DiagnosticHintString: func() string { return "hint" }}).FormatCommandResult(
		mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1},
		queries.CommandResult{CorrelationID: "slow", Status: "timeout", Error: "no response", CreatedAt: time.Unix(100, 0)},
		"slow",
	))
	data := decodeResponseData(t, result)
	if !result.IsError || data["error"] != mcp.ErrExtTimeout || data["final"] != true {
		t.Fatalf("timeout result = %#v data=%#v", result, data)
	}
	if retry, _ := data["retry"].(string); !strings.Contains(retry, "disconnected") {
		t.Fatalf("timeout retry guidance = %q, want disconnected-extension guidance", retry)
	}
}

func TestStillProcessingResponseIsNonFinalPollingHandle(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	defer captured.Close()
	cmd := &queries.CommandResult{CorrelationID: "long", Status: "pending", CreatedAt: time.Now()}
	result := decodeToolResult(t, New(Deps{Capture: captured}).stillProcessingResponse(
		mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, cmd, "long", 2, 1500,
	))
	data := decodeResponseData(t, result)
	retryContext, _ := data["retry_context"].(map[string]any)
	if result.IsError || data["status"] != "still_processing" || data["final"] != false ||
		retryContext["attempts"] != float64(2) || retryContext["total_wait_ms"] != float64(1500) {
		t.Fatalf("still-processing response = %#v data=%#v", result, data)
	}
}
