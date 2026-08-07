// startup_test.go — Tests deterministic startup recovery and Doctor reporting.

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

func TestRecoverAtStartupRestoresPersistedTransactionWhenExtensionIsReady(t *testing.T) {
	restores := 0
	handler := mustHandler(t, func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		switch command {
		case "environment_transaction_snapshot":
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_1"}`), nil
		case "environment_transaction_apply":
			return json.RawMessage(`{"success":true,"mutations":{}}`), nil
		case "environment_transaction_restore":
			restores++
			return json.RawMessage(`{"success":true,"restored":true}`), nil
		case "environment_transaction_reconcile":
			return json.RawMessage(`{"success":true,"pruned":0,"retained":1}`), nil
		default:
			return nil, context.Canceled
		}
	})
	handler.Handle(mcp.JSONRPCRequest{ID: 1}, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))

	startRecoveryAndWait(t, handler, func(context.Context, time.Duration) bool { return true })
	if restores != 1 || handler.registry.Len() != 0 {
		t.Fatalf("restores=%d registry_len=%d", restores, handler.registry.Len())
	}
	if got := handler.diagnostics.(interface{ Snapshot() []statediag.Diagnostic }).Snapshot(); len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
		t.Fatalf("diagnostics = %#v, want correlated recovered lifecycle", got)
	}
}

func TestRecoverAtStartupReconcilesEmptyDurableRegistryBeforeFixtureUse(t *testing.T) {
	var commands []string
	var reconcilePayload string
	handler := mustHandler(t, func(_ context.Context, command string, params json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		commands = append(commands, command)
		if command != "environment_transaction_reconcile" {
			return nil, errors.New("unexpected_command")
		}
		reconcilePayload = string(params)
		return json.RawMessage(`{"success":true,"pruned":2,"retained":0}`), nil
	})

	startRecoveryAndWait(t, handler, func(context.Context, time.Duration) bool { return true })
	if strings.Join(commands, ",") != "environment_transaction_reconcile" {
		t.Fatalf("startup commands = %v", commands)
	}
	if reconcilePayload != `{"snapshot_ids":[]}` {
		t.Fatalf("reconcile payload = %s, want opaque empty ownership set", reconcilePayload)
	}
	encoded, _ := json.Marshal(handler.diagnostics.(interface{ Snapshot() []statediag.Diagnostic }).Snapshot())
	if strings.Contains(string(encoded), "opaque_") || strings.Contains(string(encoded), "private") {
		t.Fatalf("reconciliation diagnostics leaked identifiers: %s", encoded)
	}
	if !strings.Contains(string(encoded), `"name":"environment_snapshot_reconciliation"`) || !strings.Contains(string(encoded), `"lifecycle":"recovered"`) {
		t.Fatalf("reconciliation recovery was not retained by Doctor: %s", encoded)
	}
}

func TestRecoverAtStartupReportsReconciliationFailureWithoutLeakingIdentifiers(t *testing.T) {
	handler := mustHandler(t, func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		if command == "environment_transaction_reconcile" {
			return nil, errors.New("private snapshot opaque_secret")
		}
		return nil, errors.New("unexpected_command")
	})

	startRecoveryAndWait(t, handler, func(context.Context, time.Duration) bool { return true })
	encoded, _ := json.Marshal(handler.diagnostics.(interface{ Snapshot() []statediag.Diagnostic }).Snapshot())
	if !strings.Contains(string(encoded), `"name":"environment_snapshot_reconciliation"`) || !strings.Contains(string(encoded), `"lifecycle":"active"`) {
		t.Fatalf("reconciliation failure missing from Doctor: %s", encoded)
	}
	if strings.Contains(string(encoded), "opaque_secret") || strings.Contains(string(encoded), "private snapshot") {
		t.Fatalf("reconciliation failure leaked extension detail: %s", encoded)
	}
}

func TestRecoverAtStartupLeavesRedactedDoctorNoticeWhenExtensionIsUnavailable(t *testing.T) {
	handler := mustHandler(t, func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		if command == "environment_transaction_snapshot" {
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_private_snapshot"}`), nil
		}
		return json.RawMessage(`{"success":true,"mutations":{}}`), nil
	})
	handler.Handle(mcp.JSONRPCRequest{ID: 1}, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))

	startRecoveryAndWait(t, handler, func(context.Context, time.Duration) bool { return false })
	got := handler.diagnostics.(interface{ Snapshot() []statediag.Diagnostic }).Snapshot()
	if len(got) != 1 || got[0].CorrelationID != "fixture_test_1" || got[0].Lifecycle != statediag.LifecycleActive {
		t.Fatalf("diagnostics = %#v", got)
	}
	encoded, _ := json.Marshal(got)
	if string(encoded) == "" || containsSensitive(string(encoded)) {
		t.Fatalf("diagnostic leaked private recovery state: %s", encoded)
	}
}

func TestRecoverAtStartupRehydratesDaemonRegistryAfterRestart(t *testing.T) {
	path := t.TempDir() + "/fixture-transactions.json"
	store := fixturecontract.NewRegistryStore(path, 4)
	firstRegistry := fixturecontract.NewRegistry(4)
	execute := func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		switch command {
		case "environment_transaction_snapshot":
			return json.RawMessage(`{"success":true,"snapshot_id":"opaque_1"}`), nil
		case "environment_transaction_apply":
			return json.RawMessage(`{"success":true,"mutations":{}}`), nil
		case "environment_transaction_restore":
			return json.RawMessage(`{"success":true,"restored":true}`), nil
		case "environment_transaction_reconcile":
			return json.RawMessage(`{"success":true,"pruned":0,"retained":1}`), nil
		default:
			return nil, context.Canceled
		}
	}
	first := mustRecoveryHandler(t, firstRegistry, store.Save, execute)
	first.Handle(mcp.JSONRPCRequest{ID: 1}, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))

	rehydrated, notice := store.Load()
	if notice != "" || rehydrated.Len() != 1 {
		t.Fatalf("Load() len=%d notice=%q", rehydrated.Len(), notice)
	}
	second := mustRecoveryHandler(t, rehydrated, store.Save, execute)
	startRecoveryAndWait(t, second, func(context.Context, time.Duration) bool { return true })
	finalRegistry, finalNotice := store.Load()
	if finalNotice != "" || finalRegistry.Len() != 0 {
		t.Fatalf("final Load() len=%d notice=%q", finalRegistry.Len(), finalNotice)
	}
}

func TestStartStartupRecoveryRegistersBarrierBeforeLaunchingWork(t *testing.T) {
	handler := mustHandler(t, func(_ context.Context, command string, _ json.RawMessage, _ time.Duration) (json.RawMessage, error) {
		if command != "environment_transaction_reconcile" {
			t.Fatalf("unexpected command %q", command)
		}
		return json.RawMessage(`{"success":true,"pruned":0,"retained":0}`), nil
	})
	entered := make(chan struct{})
	release := make(chan struct{})
	done := handler.StartStartupRecovery(context.Background(), func(context.Context, time.Duration) bool {
		close(entered)
		<-release
		return true
	})
	if repeated := handler.StartStartupRecovery(context.Background(), func(context.Context, time.Duration) bool {
		t.Fatal("repeated start launched a second recovery")
		return false
	}); repeated != done {
		t.Fatal("repeated start returned a different lifecycle barrier")
	}
	<-entered
	select {
	case <-done:
		t.Fatal("startup barrier closed before recovery completed")
	default:
	}
	close(release)
	<-done
}

func startRecoveryAndWait(t *testing.T, handler *Handler, waiter ExtensionReadinessWaiter) {
	t.Helper()
	<-handler.StartStartupRecovery(context.Background(), waiter)
}

func mustRecoveryHandler(
	t *testing.T,
	registry *fixturecontract.Registry,
	persist func(*fixturecontract.Registry) error,
	execute CommandExecutor,
) *Handler {
	t.Helper()
	handler, err := New(Deps{
		Context: context.Background(), Execute: execute,
		NewCorrelationID:    func() string { return "correlation_1" },
		NewTransactionID:    func() string { return "transaction_1" },
		ExtensionGeneration: func() string { return "generation_1" },
		Now:                 func() time.Time { return time.Unix(1, 0) },
		Registry:            registry, Persist: persist, OnNotice: func(string) {},
		Diagnostics: statediag.NewCollector(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func containsSensitive(value string) bool {
	for _, sensitive := range []string{"opaque_private_snapshot", "cookie", "storage"} {
		if strings.Contains(value, sensitive) {
			return true
		}
	}
	return false
}
