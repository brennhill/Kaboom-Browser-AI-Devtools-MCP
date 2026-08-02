// handler_test.go — Tests configure QA fixture validation responses.

package qafixture

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestHandleValidateAcceptsVersionedFixtureWithoutEchoingState(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := mustHandler(t, nil).Handle(req, json.RawMessage(`{
		"what":"qa_fixture",
		"fixture_action":"validate",
		"fixture":{"version":1,"local_storage":{"token":"private-value"}}
	}`))
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-value") {
		t.Fatal("response leaked fixture state")
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"fixture_version":1`) || !strings.Contains(result.Content[0].Text, `"valid":true`) {
		t.Fatalf("response missing validation fields: %s", encoded)
	}
}

func TestHandleRejectsUnsupportedActionBeforeMutation(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := mustHandler(t, nil).Handle(req, json.RawMessage(`{"fixture_action":"unknown","fixture":{"version":1}}`))
	encoded, _ := json.Marshal(resp)
	if !strings.Contains(string(encoded), "invalid_param") || !strings.Contains(string(encoded), "apply") {
		t.Fatalf("response = %s, want actionable fixture action error", encoded)
	}
}

func TestHandleRejectsMalformedFixtureRedacted(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := mustHandler(t, nil).Handle(req, json.RawMessage(`{"fixture_action":"validate","fixture":{"version":1,"unknown":"private-value"}}`))
	encoded, _ := json.Marshal(resp)
	if !strings.Contains(string(encoded), "invalid_param") {
		t.Fatalf("response = %s, want invalid_param", encoded)
	}
	if strings.Contains(string(encoded), "private-value") {
		t.Fatal("response leaked invalid fixture value")
	}
}

func TestHandlerApplyRunsSnapshotThenApplyWithoutEchoingFixture(t *testing.T) {
	var commands []string
	handler := mustHandler(t, func(_ context.Context, command string, params json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		commands = append(commands, command)
		if !strings.Contains(string(params), "private-value") {
			t.Fatalf("%s params missing fixture", command)
		}
		if command == "qa_fixture_snapshot" {
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_1"}`), nil
		}
		return json.RawMessage(`{"success":true,"mutations":{"local_storage":1}}`), nil
	})
	resp := handler.Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{
		"fixture_action":"apply",
		"fixture":{"version":1,"local_storage":{"token":"private-value"}}
	}`))
	encoded, _ := json.Marshal(resp)
	if strings.Join(commands, ",") != "qa_fixture_snapshot,qa_fixture_apply" {
		t.Fatalf("commands = %v", commands)
	}
	if strings.Contains(string(encoded), "private-value") || strings.Contains(string(encoded), "opaque_1") {
		t.Fatalf("response leaked private fixture state: %s", encoded)
	}
	if !strings.Contains(string(encoded), `\"status\":\"applied\"`) {
		t.Fatalf("response = %s", encoded)
	}
}

func TestHandlerApplyFailureRestoresWithOpaqueSnapshotAndRedactsCause(t *testing.T) {
	var commands []string
	handler := mustHandler(t, func(_ context.Context, command string, params json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		commands = append(commands, command)
		switch command {
		case "qa_fixture_snapshot":
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_1"}`), nil
		case "qa_fixture_apply":
			return nil, errors.New("private-cookie-value")
		case "qa_fixture_restore":
			if !strings.Contains(string(params), `"snapshot_id":"opaque_1"`) {
				t.Fatalf("restore params = %s", params)
			}
			return json.RawMessage(`{"success":true,"restored":true}`), nil
		default:
			t.Fatalf("unexpected command %q", command)
			return nil, nil
		}
	})
	resp := handler.Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{
		"fixture_action":"apply","fixture":{"version":1,"cookies":[{"name":"session","value":"private-cookie"}]}
	}`))
	encoded, _ := json.Marshal(resp)
	if strings.Join(commands, ",") != "qa_fixture_snapshot,qa_fixture_apply,qa_fixture_restore" {
		t.Fatalf("commands = %v", commands)
	}
	if strings.Contains(string(encoded), "private") || strings.Contains(string(encoded), "opaque_1") {
		t.Fatalf("response leaked private state: %s", encoded)
	}
	if !strings.Contains(string(encoded), "apply_failed_rolled_back") {
		t.Fatalf("response = %s", encoded)
	}
}

func mustHandler(t *testing.T, execute CommandExecutor) *Handler {
	t.Helper()
	if execute == nil {
		execute = func(context.Context, string, json.RawMessage, time.Duration) (json.RawMessage, error) {
			return nil, errors.New("unexpected command")
		}
	}
	handler, err := New(Deps{
		Context:          context.Background(),
		Execute:          execute,
		NewCorrelationID: func() string { return "fixture_test_1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}
