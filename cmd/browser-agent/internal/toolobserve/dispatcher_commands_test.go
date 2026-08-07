// dispatcher_commands_test.go — Tests observe command-state projections.

package toolobserve

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
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

type commandStoreStub struct {
	failed []*queries.CommandResult
}

func (s commandStoreStub) WaitForCommand(string, time.Duration) (*queries.CommandResult, bool) {
	return nil, false
}
func (s commandStoreStub) GetCommandResult(string) (*queries.CommandResult, bool) { return nil, false }
func (s commandStoreStub) GetPendingCommands() []*queries.CommandResult           { return nil }
func (s commandStoreStub) GetCompletedCommands() []*queries.CommandResult         { return nil }
func (s commandStoreStub) GetFailedCommands() []*queries.CommandResult            { return s.failed }

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
