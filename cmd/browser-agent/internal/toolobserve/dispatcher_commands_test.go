// dispatcher_commands_test.go — Tests observe command-state projections.

package toolobserve

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	observecore "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestDispatcherRegistersPageInventory(t *testing.T) {
	t.Parallel()
	if modes := NewDispatcher(Config{}).ValidModes(); !slices.Contains(modes, "page_inventory") {
		t.Fatalf("page_inventory missing from observe modes: %v", modes)
	}
}

func TestDispatcherDefaultsOptionalHooksAndReturnsStructuredRoutingErrors(t *testing.T) {
	t.Parallel()
	dispatcher := NewDispatcher(Config{})
	if !slices.IsSorted(dispatcher.ValidModes()) {
		t.Fatalf("observe modes are not sorted: %v", dispatcher.ValidModes())
	}
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	for name, args := range map[string]json.RawMessage{
		"invalid JSON": json.RawMessage(`{bad`),
		"missing mode": json.RawMessage(`{}`),
		"unknown mode": json.RawMessage(`{"what":"missing"}`),
	} {
		t.Run(name, func(t *testing.T) {
			response := dispatcher.Handle(req, args)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil || !result.IsError || len(result.Content) == 0 {
				t.Fatalf("routing response = %s, err=%v", response.Result, err)
			}
			if !strings.Contains(result.Content[0].Text, `"error_code"`) || !strings.Contains(result.Content[0].Text, `"recovery_playbook"`) {
				t.Fatalf("routing error is not structured: %s", result.Content[0].Text)
			}
		})
	}
}

func TestPilotModeDoesNotReceiveDisconnectWarning(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	defer captured.Close()
	dispatcher := NewDispatcher(Config{
		Observe:              observecore.Deps{Capture: captured},
		IsExtensionConnected: func() bool { return false },
		InjectSummary:        func(args json.RawMessage) json.RawMessage { return args },
		DrainAlerts:          func() []types.Alert { return nil },
	})
	response := dispatcher.Handle(mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, json.RawMessage(`{"what":"pilot"}`))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil || result.IsError || len(result.Content) == 0 {
		t.Fatalf("pilot response = %s, err=%v", response.Result, err)
	}
	if strings.Contains(result.Content[0].Text, "Extension is not connected") {
		t.Fatalf("pilot received disconnect warning: %s", result.Content[0].Text)
	}
}

type commandStoreStub struct {
	failed    []*queries.CommandResult
	completed []*queries.CommandResult
	pending   []*queries.CommandResult
	command   *queries.CommandResult
}

func (s commandStoreStub) WaitForCommand(string, time.Duration) (*queries.CommandResult, bool) {
	return s.command, s.command != nil
}
func (s commandStoreStub) GetCommandResult(string) (*queries.CommandResult, bool) {
	return s.command, s.command != nil
}
func (s commandStoreStub) GetPendingCommands() []*queries.CommandResult   { return s.pending }
func (s commandStoreStub) GetCompletedCommands() []*queries.CommandResult { return s.completed }
func (s commandStoreStub) GetFailedCommands() []*queries.CommandResult    { return s.failed }

func TestFailedCommandsProjectsEmptyAndFailedStates(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	for name, commands := range map[string][]*queries.CommandResult{
		"empty":  nil,
		"failed": {{CorrelationID: "failed-1", Status: "expired"}},
	} {
		t.Run(name, func(t *testing.T) {
			response := NewDispatcher(Config{Commands: commandStoreStub{failed: commands}}).FailedCommands(req, nil)
			var result mcp.MCPToolResult
			if err := json.Unmarshal(response.Result, &result); err != nil || result.IsError || len(result.Content) == 0 {
				t.Fatalf("failed commands response = %s, err=%v", response.Result, err)
			}
			var data map[string]any
			for index, character := range result.Content[0].Text {
				if character == '{' {
					if err := json.Unmarshal([]byte(result.Content[0].Text[index:]), &data); err != nil {
						t.Fatal(err)
					}
					break
				}
			}
			if data["count"] != float64(len(commands)) {
				t.Fatalf("count = %v, want %d", data["count"], len(commands))
			}
			if _, exists := data["status"]; exists {
				t.Fatal("failed command projection must not invent a status field")
			}
		})
	}
}

func TestServerSideCommandAndPilotResponseContracts(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	defer captured.Close()
	dispatcher := NewDispatcher(Config{
		Observe: observecore.Deps{Capture: captured}, Commands: commandStoreStub{},
		IsExtensionConnected: func() bool { return true },
		InjectSummary:        func(args json.RawMessage) json.RawMessage { return args },
		DrainAlerts:          func() []types.Alert { return nil },
	})
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 2}

	for name, args := range map[string]json.RawMessage{
		"pilot":            json.RawMessage(`{"what":"pilot"}`),
		"pending commands": json.RawMessage(`{"what":"pending_commands"}`),
	} {
		data := decodeResponseData(t, dispatcher.Handle(req, args))
		if name == "pilot" {
			for _, field := range []string{"enabled", "source", "extension_connected"} {
				if _, ok := data[field]; !ok {
					t.Fatalf("pilot missing %q: %#v", field, data)
				}
			}
			continue
		}
		for _, field := range []string{"pending", "completed", "failed", "extension_in_progress"} {
			if values, ok := data[field].([]any); !ok || len(values) != 0 {
				t.Fatalf("pending commands %q = %#v", field, data[field])
			}
		}
	}

	missingID := dispatcher.Handle(req, json.RawMessage(`{"what":"command_result"}`))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(missingID.Result, &result); err != nil || !result.IsError || !strings.Contains(string(missingID.Result), mcp.ErrMissingParam) {
		t.Fatalf("missing command state response = %s, err=%v", missingID.Result, err)
	}
}

func TestCommandResultMissingAndAnnotationMissingAreTerminal(t *testing.T) {
	t.Parallel()
	dispatcher := NewDispatcher(Config{
		Commands: commandStoreStub{}, DiagnosticHint: mcp.WithHint("doctor"),
	})
	for _, correlationID := range []string{"missing-command", "ann_missing-command"} {
		response := dispatcher.CommandResult(
			mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1},
			json.RawMessage(`{"correlation_id":"`+correlationID+`"}`),
		)
		var result mcp.MCPToolResult
		if err := json.Unmarshal(response.Result, &result); err != nil || !result.IsError ||
			!strings.Contains(result.Content[0].Text, mcp.ErrNoData) || !strings.Contains(result.Content[0].Text, `"final":true`) {
			t.Fatalf("missing %s response = %s, err=%v", correlationID, response.Result, err)
		}
	}
}

func TestCommandResultDelegatesStoredCommandToFormatter(t *testing.T) {
	t.Parallel()
	command := &queries.CommandResult{CorrelationID: "complete", Status: "complete"}
	formatted := false
	dispatcher := NewDispatcher(Config{
		Commands: commandStoreStub{command: command},
		FormatCommand: func(req mcp.JSONRPCRequest, got queries.CommandResult, correlationID string) mcp.JSONRPCResponse {
			formatted = got.CorrelationID == command.CorrelationID && correlationID == command.CorrelationID
			return mcp.SucceedText(req, "formatted")
		},
	})
	response := dispatcher.CommandResult(
		mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, json.RawMessage(`{"correlation_id":"complete"}`),
	)
	if !formatted || strings.Contains(string(response.Result), `"isError":true`) {
		t.Fatalf("formatted=%t response=%s", formatted, response.Result)
	}
}

func decodeResponseData(t *testing.T, response mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil || result.IsError || len(result.Content) == 0 {
		t.Fatalf("response = %s, err=%v", response.Result, err)
	}
	for index, character := range result.Content[0].Text {
		if character != '{' {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(result.Content[0].Text[index:]), &data); err != nil {
			t.Fatalf("decode response data: %v", err)
		}
		return data
	}
	t.Fatal("response contains no JSON object")
	return nil
}

// pending_commands returned every pending, completed and failed command the
// daemon had ever seen. The completed list grows for the whole life of the
// process, so an agent asking "did my command finish?" was handed the entire
// command history — 48,949 bytes on a daemon that had been up for a working
// day, against 162 on a fresh one.
func TestPendingCommandsBoundsEachCollection(t *testing.T) {
	t.Parallel()
	completed := make([]*queries.CommandResult, commandListDefaultLimit+30)
	for i := range completed {
		completed[i] = &queries.CommandResult{CorrelationID: "done-" + strconv.Itoa(i), Status: "complete"}
	}
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	response := NewDispatcher(Config{Commands: commandStoreStub{completed: completed}}).
		pendingCommands(observecore.Deps{}, req, nil)

	data := commandsPayload(t, response)
	list, _ := data["completed"].([]any)
	if len(list) != commandListDefaultLimit {
		t.Fatalf("returned %d completed commands, want the default page of %d", len(list), commandListDefaultLimit)
	}
	if total, _ := data["completed_total"].(float64); int(total) != commandListDefaultLimit+30 {
		t.Errorf("completed_total = %v, want every command counted", data["completed_total"])
	}
	if truncated, _ := data["truncated"].(bool); !truncated {
		t.Error("truncated must be true when commands were withheld")
	}
}

// A daemon with little history must not claim its listing was cut.
func TestPendingCommandsDoesNotClaimTruncationWhenComplete(t *testing.T) {
	t.Parallel()
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}
	response := NewDispatcher(Config{Commands: commandStoreStub{
		completed: []*queries.CommandResult{{CorrelationID: "done-1", Status: "complete"}},
	}}).pendingCommands(observecore.Deps{}, req, nil)

	data := commandsPayload(t, response)
	if truncated, ok := data["truncated"].(bool); ok && truncated {
		t.Error("a complete listing must not report truncated")
	}
	if total, _ := data["completed_total"].(float64); int(total) != 1 {
		t.Errorf("completed_total = %v, want 1", data["completed_total"])
	}
}

func commandsPayload(t *testing.T, response mcp.JSONRPCResponse) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil || len(result.Content) == 0 {
		t.Fatalf("response = %s, err=%v", response.Result, err)
	}
	text := result.Content[0].Text
	start := strings.Index(text, "{")
	if start < 0 {
		t.Fatalf("no JSON body in %.120q", text)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[start:]), &data); err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	return data
}
