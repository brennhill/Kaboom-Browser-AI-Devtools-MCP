// registry_store_test.go — Tests atomic and corruption-tolerant transaction persistence.

package qafixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRegistryStoreRoundTripContainsNoFixtureValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture-transactions.json")
	store := NewRegistryStore(path, 4)
	registry := NewRegistry(4)
	if err := registry.Add(TransactionRecord{
		TransactionID: "tx_1", CorrelationID: "corr_1", SnapshotID: "opaque_1",
		ExtensionGeneration: "generation_1", State: TransactionRestoreRequired,
		CreatedAt: time.Unix(10, 0), Mutations: MutationCounts{Cookies: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(registry); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"cookie_value", "storage_value", "private-secret"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("persisted registry contains %q", forbidden)
		}
	}
	loaded, notice := store.Load()
	if notice != "" || loaded.Len() != 1 {
		t.Fatalf("Load() len=%d notice=%q", loaded.Len(), notice)
	}
}

func TestRegistryStoreResumesInterruptedRestoreAsRequired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture-transactions.json")
	store := NewRegistryStore(path, 2)
	registry := NewRegistry(2)
	if err := registry.Add(TransactionRecord{
		TransactionID: "tx_1", CorrelationID: "corr_1", SnapshotID: "snapshot_1", ExtensionGeneration: "generation_1",
		State: TransactionRestoring, CreatedAt: time.Unix(1, 0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(registry); err != nil {
		t.Fatal(err)
	}

	loaded, notice := store.Load()
	record, ok := loaded.Get("tx_1")
	if notice != "" || !ok || record.State != TransactionRestoreRequired {
		t.Fatalf("Load() record=%+v present=%v notice=%q", record, ok, notice)
	}
}

func TestRegistryStoreQuarantinesCorruptStateAndStartsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture-transactions.json")
	if err := os.WriteFile(path, []byte(`{"records":[{"snapshot_id":"private-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, notice := NewRegistryStore(path, 4).Load()
	if registry.Len() != 0 || notice != "fixture_transaction_registry_corrupt" {
		t.Fatalf("Load() len=%d notice=%q", registry.Len(), notice)
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatalf("corrupt state was not quarantined: %v", err)
	}
	if strings.Contains(notice, "private-secret") {
		t.Fatalf("notice leaked persisted content: %q", notice)
	}
}

func TestRegistryStoreReportsFailedCorruptStateQuarantine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture-transactions.json")
	if err := os.WriteFile(path, []byte(`not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewRegistryStore(path, 4)
	store.rename = func(_, _ string) error { return os.ErrPermission }

	registry, notice := store.Load()
	if registry.Len() != 0 || notice != "fixture_transaction_registry_corrupt_quarantine_failed" {
		t.Fatalf("Load() len=%d notice=%q", registry.Len(), notice)
	}
}

func TestRegistryStoreQuarantinesDuplicateTransactionIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture-transactions.json")
	data := `{"version":1,"records":[` +
		`{"transaction_id":"tx_1","snapshot_id":"snapshot_1","extension_generation":"generation_1","state":"restore_required","created_at":"1970-01-01T00:00:01Z","mutations":{}},` +
		`{"transaction_id":"tx_1","snapshot_id":"snapshot_2","extension_generation":"generation_1","state":"restore_required","created_at":"1970-01-01T00:00:02Z","mutations":{}}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, notice := NewRegistryStore(path, 4).Load()
	if registry.Len() != 0 || notice != "fixture_transaction_registry_corrupt" {
		t.Fatalf("Load() len=%d notice=%q", registry.Len(), notice)
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatalf("duplicate state was not quarantined: %v", err)
	}
}

func TestRegistryStoreQuarantinesUnsafeOpaqueIdentifiers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture-transactions.json")
	data := `{"version":1,"records":[{"transaction_id":"private value","correlation_id":"corr_1","snapshot_id":"snapshot_1","extension_generation":"generation_1","state":"restore_required","created_at":"1970-01-01T00:00:01Z","mutations":{}}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	registry, notice := NewRegistryStore(path, 4).Load()
	if registry.Len() != 0 || notice != "fixture_transaction_registry_corrupt" {
		t.Fatalf("Load() len=%d notice=%q", registry.Len(), notice)
	}
}

func TestRegistryStoreFailedSavePreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture-transactions.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"records":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewRegistryStore(path, 4)
	store.rename = func(_, _ string) error { return os.ErrPermission }
	if err := store.Save(NewRegistry(4)); err == nil {
		t.Fatal("Save() succeeded despite rename failure")
	}
	data, _ := os.ReadFile(path)
	if string(data) != `{"version":1,"records":[]}` {
		t.Fatalf("existing file changed after failed save: %s", data)
	}
}
