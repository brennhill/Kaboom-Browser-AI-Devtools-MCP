// startup_test.go — Tests deterministic startup recovery and Doctor reporting.

package qafixture

import (
	"context"
	"encoding/json"
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
		default:
			return nil, context.Canceled
		}
	})
	handler.Handle(mcp.JSONRPCRequest{ID: 1}, json.RawMessage(`{"fixture_action":"apply","fixture":{"version":1}}`))

	handler.RecoverAtStartup(context.Background(), func(context.Context, time.Duration) bool { return true })
	if restores != 1 || handler.registry.Len() != 0 {
		t.Fatalf("restores=%d registry_len=%d", restores, handler.registry.Len())
	}
	if got := handler.diagnostics.(interface{ Snapshot() []statediag.Diagnostic }).Snapshot(); len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
		t.Fatalf("diagnostics = %#v, want correlated recovered lifecycle", got)
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

	handler.RecoverAtStartup(context.Background(), func(context.Context, time.Duration) bool { return false })
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
	second.RecoverAtStartup(context.Background(), func(context.Context, time.Duration) bool { return true })
	finalRegistry, finalNotice := store.Load()
	if finalNotice != "" || finalRegistry.Len() != 0 {
		t.Fatalf("final Load() len=%d notice=%q", finalRegistry.Len(), finalNotice)
	}
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
