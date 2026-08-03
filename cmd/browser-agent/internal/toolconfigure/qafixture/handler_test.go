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
	fixturecontract "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/qafixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
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
		if command == "environment_transaction_snapshot" {
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_1"}`), nil
		}
		return json.RawMessage(`{"success":true,"mutations":{"local_storage":1}}`), nil
	})
	resp := handler.Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{
		"fixture_action":"apply",
		"fixture":{"version":1,"local_storage":{"token":"private-value"}}
	}`))
	encoded, _ := json.Marshal(resp)
	if strings.Join(commands, ",") != "environment_transaction_snapshot,environment_transaction_apply" {
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
		case "environment_transaction_snapshot":
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_1"}`), nil
		case "environment_transaction_apply":
			return nil, errors.New("private-cookie-value")
		case "environment_transaction_restore":
			if !strings.Contains(string(params), `"snapshot_id":"opaque_1"`) {
				t.Fatalf("restore params = %s", params)
			}
			if strings.Contains(string(params), `"fixture"`) || strings.Contains(string(params), "private-cookie") {
				t.Fatalf("restore resent private fixture state: %s", params)
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
	if strings.Join(commands, ",") != "environment_transaction_snapshot,environment_transaction_apply,environment_transaction_restore" {
		t.Fatalf("commands = %v", commands)
	}
	if strings.Contains(string(encoded), "private") || strings.Contains(string(encoded), "opaque_1") {
		t.Fatalf("response leaked private state: %s", encoded)
	}
	if !strings.Contains(string(encoded), "apply_failed_rolled_back") {
		t.Fatalf("response = %s", encoded)
	}
}

func TestHandlerExposesRedactedStatusAndIdempotentRestoreLifecycle(t *testing.T) {
	restores := 0
	handler := mustHandler(t, func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		switch command {
		case "environment_transaction_snapshot":
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_private_snapshot"}`), nil
		case "environment_transaction_apply":
			return json.RawMessage(`{"success":true,"mutations":{"cookies":1}}`), nil
		case "environment_transaction_restore":
			restores++
			return json.RawMessage(`{"success":true,"restored":true}`), nil
		default:
			return nil, errors.New("unexpected command")
		}
	})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	apply := handler.Handle(req, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1,"cookies":[{"name":"session","value":"private-cookie"}]}}`))
	applyJSON, _ := json.Marshal(apply)
	if !strings.Contains(string(applyJSON), "transaction_test_1") || strings.Contains(string(applyJSON), "opaque_private_snapshot") {
		t.Fatalf("apply response = %s", applyJSON)
	}

	status := handler.Handle(req, json.RawMessage(`{"fixture_action":"status"}`))
	statusJSON, _ := json.Marshal(status)
	if !strings.Contains(string(statusJSON), "transaction_test_1") || strings.Contains(string(statusJSON), "opaque_private_snapshot") || strings.Contains(string(statusJSON), "generation_test_1") {
		t.Fatalf("status response = %s", statusJSON)
	}

	for attempt := 0; attempt < 2; attempt++ {
		response := handler.Handle(req, json.RawMessage(`{"fixture_action":"restore","transaction_id":"transaction_test_1"}`))
		encoded, _ := json.Marshal(response)
		if !strings.Contains(string(encoded), `\"restored\":true`) {
			t.Fatalf("restore response = %s", encoded)
		}
	}
	if restores != 1 {
		t.Fatalf("restore command count = %d, want 1", restores)
	}
}

func TestHandlerReportsCorrelatedActiveAndRecoveredLifecycle(t *testing.T) {
	diagnostics := statediag.NewCollector()
	handler := mustHandlerWithDiagnostics(t, func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		switch command {
		case "environment_transaction_snapshot":
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_1"}`), nil
		case "environment_transaction_apply":
			return json.RawMessage(`{"success":true,"mutations":{}}`), nil
		case "environment_transaction_restore":
			return json.RawMessage(`{"success":true,"restored":true}`), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}, diagnostics)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	handler.Handle(req, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))

	active := diagnostics.Snapshot()
	if len(active) != 1 || active[0].Lifecycle != statediag.LifecycleActive || active[0].CorrelationID != "fixture_test_1" {
		t.Fatalf("active diagnostics = %#v", active)
	}
	handler.Handle(req, json.RawMessage(`{"fixture_action":"restore","transaction_id":"transaction_test_1"}`))
	recovered := diagnostics.Snapshot()
	if len(recovered) != 1 || recovered[0].Lifecycle != statediag.LifecycleRecovered || len(recovered[0].History) != 2 {
		t.Fatalf("recovered diagnostics = %#v", recovered)
	}
}

func TestHandlerReportsRedactedCorrelatedRestoreFailure(t *testing.T) {
	diagnostics := statediag.NewCollector()
	handler := mustHandlerWithDiagnostics(t, func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		switch command {
		case "environment_transaction_snapshot":
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_1"}`), nil
		case "environment_transaction_apply":
			return json.RawMessage(`{"success":true,"mutations":{}}`), nil
		case "environment_transaction_restore":
			return nil, errors.New("private-cookie-value")
		default:
			return nil, errors.New("unexpected command")
		}
	}, diagnostics)
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	handler.Handle(req, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))
	handler.Handle(req, json.RawMessage(`{"fixture_action":"restore","transaction_id":"transaction_test_1"}`))

	got := diagnostics.Snapshot()
	encoded, _ := json.Marshal(got)
	if len(got) != 1 || got[0].Lifecycle != statediag.LifecycleActive || got[0].CorrelationID != "fixture_test_1" {
		t.Fatalf("restore failure diagnostics = %s", encoded)
	}
	if strings.Contains(string(encoded), "private-cookie-value") {
		t.Fatalf("restore failure leaked private cause: %s", encoded)
	}
}

func TestHandlerMarksSnapshotFailureRecoveredBecauseNoStateWasMutated(t *testing.T) {
	diagnostics := statediag.NewCollector()
	handler := mustHandlerWithDiagnostics(t, func(_ context.Context, _ string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		return nil, errors.New("private-snapshot-cause")
	}, diagnostics)
	handler.Handle(mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))

	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
		t.Fatalf("snapshot failure diagnostics = %#v, want recovered incident", got)
	}
}

func TestHandlerRejectsIncompleteDependenciesAndMalformedExtensionResults(t *testing.T) {
	t.Parallel()
	if handler, err := New(Deps{}); err == nil || handler != nil {
		t.Fatalf("New() = %#v, %v", handler, err)
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	if resp := mustHandler(t, nil).Handle(req, json.RawMessage(`{"fixture_action":"restore"}`)); !strings.Contains(string(resp.Result), "missing_param") {
		t.Fatalf("missing transaction response = %s", resp.Result)
	}
	if resp := mustHandler(t, nil).Handle(req, json.RawMessage(`{"fixture_action":"validate"}`)); !strings.Contains(string(resp.Result), "missing_param") {
		t.Fatalf("missing fixture response = %s", resp.Result)
	}
	for name, commandResult := range map[string]json.RawMessage{
		"snapshot": json.RawMessage(`{"success":true}`),
		"apply":    json.RawMessage(`{"success":false}`),
	} {
		t.Run(name, func(t *testing.T) {
			handler := mustHandler(t, func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
				if (name == "snapshot" && command == "environment_transaction_snapshot") ||
					(name == "apply" && command == "environment_transaction_apply") {
					return commandResult, nil
				}
				if command == "environment_transaction_snapshot" {
					return json.RawMessage(`{"success":true,"snapshot_id":"opaque"}`), nil
				}
				if command == "environment_transaction_restore" {
					return json.RawMessage(`{"success":true,"restored":true}`), nil
				}
				return nil, errors.New("unexpected")
			})
			resp := handler.Handle(req, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))
			if !strings.Contains(string(resp.Result), "failed") {
				t.Fatalf("%s malformed result response = %s", name, resp.Result)
			}
		})
	}
}

func TestHandlerRejectsInvalidRestoreAcknowledgement(t *testing.T) {
	handler := mustHandler(t, func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		switch command {
		case "environment_transaction_snapshot":
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque"}`), nil
		case "environment_transaction_apply":
			return json.RawMessage(`{"success":true,"mutations":{}}`), nil
		case "environment_transaction_restore":
			return json.RawMessage(`{"success":true,"restored":false}`), nil
		default:
			return nil, errors.New("unexpected")
		}
	})
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 3}
	_ = handler.Handle(req, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))
	resp := handler.Handle(req, json.RawMessage(`{"fixture_action":"restore","transaction_id":"transaction_test_1"}`))
	if !strings.Contains(string(resp.Result), "restore failed") {
		t.Fatalf("invalid restore acknowledgement = %s", resp.Result)
	}
}

func mustHandler(t *testing.T, execute CommandExecutor) *Handler {
	return mustHandlerWithDiagnostics(t, execute, statediag.NewCollector())
}

func mustHandlerWithDiagnostics(t *testing.T, execute CommandExecutor, diagnostics *statediag.Collector) *Handler {
	t.Helper()
	if execute == nil {
		execute = func(context.Context, string, json.RawMessage, time.Duration) (json.RawMessage, error) {
			return nil, errors.New("unexpected command")
		}
	}
	handler, err := New(Deps{
		Context:             context.Background(),
		Execute:             execute,
		NewCorrelationID:    func() string { return "fixture_test_1" },
		NewTransactionID:    func() string { return "transaction_test_1" },
		ExtensionGeneration: func() string { return "generation_test_1" },
		Now:                 func() time.Time { return time.Unix(1, 0) },
		Registry:            fixturecontract.NewRegistry(32),
		Persist:             func(*fixturecontract.Registry) error { return nil },
		OnNotice:            func(string) {},
		Diagnostics:         diagnostics,
	})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}
