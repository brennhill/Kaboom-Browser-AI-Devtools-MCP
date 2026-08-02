// registry_test.go — Tests bounded, generation-aware fixture transaction state.

package qafixture

import (
	"reflect"
	"testing"
	"time"
)

func TestRegistryRejectsNewTransactionAtCapacityWithoutEvictingRecovery(t *testing.T) {
	registry := NewRegistry(2)
	if err := registry.Add(recoveryRecord("old", time.Unix(1, 0))); err != nil {
		t.Fatal(err)
	}
	if err := registry.Add(recoveryRecord("new", time.Unix(2, 0))); err != nil {
		t.Fatal(err)
	}
	if err := registry.Add(recoveryRecord("newest", time.Unix(3, 0))); err == nil || err.Error() != "fixture_transaction_registry_full" {
		t.Fatalf("Add() error = %v", err)
	}

	if _, ok := registry.Get("old"); !ok {
		t.Fatal("active recovery obligation was evicted")
	}
	if registry.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", registry.Len())
	}
}

func TestRegistryRejectsIncompleteRecoveryRecord(t *testing.T) {
	registry := NewRegistry(2)
	for _, record := range []TransactionRecord{
		{SnapshotID: "snapshot", ExtensionGeneration: "generation", State: TransactionRestoreRequired},
		{TransactionID: "transaction", ExtensionGeneration: "generation", State: TransactionRestoreRequired},
		{TransactionID: "transaction", SnapshotID: "snapshot", State: TransactionRestoreRequired},
	} {
		if err := registry.Add(record); err == nil || err.Error() != "fixture_transaction_record_invalid" {
			t.Fatalf("Add(%+v) error = %v", record, err)
		}
	}
	if registry.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", registry.Len())
	}
}

func TestRegistryRecordsHaveStableRecoveryOrder(t *testing.T) {
	registry := NewRegistry(3)
	_ = registry.Add(recoveryRecord("b", time.Unix(2, 0)))
	_ = registry.Add(recoveryRecord("c", time.Unix(1, 0)))
	_ = registry.Add(recoveryRecord("a", time.Unix(1, 0)))
	records := registry.Records()
	got := []string{records[0].TransactionID, records[1].TransactionID, records[2].TransactionID}
	if !reflect.DeepEqual(got, []string{"a", "c", "b"}) {
		t.Fatalf("recovery order = %v", got)
	}
}

func TestRegistryRejectsRestoreFromStaleExtensionGeneration(t *testing.T) {
	registry := NewRegistry(2)
	if err := registry.Add(TransactionRecord{
		TransactionID:       "tx_1",
		SnapshotID:          "opaque_1",
		ExtensionGeneration: "generation_1",
		State:               TransactionRestoreRequired,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := registry.BeginRestore("tx_1", "generation_2"); err == nil || err.Error() != "fixture_transaction_generation_mismatch" {
		t.Fatalf("BeginRestore() error = %v", err)
	}
	record, _ := registry.Get("tx_1")
	if record.State != TransactionRestoreRequired {
		t.Fatalf("state = %q, want %q", record.State, TransactionRestoreRequired)
	}
}

func TestRegistryRestoreTransitionsAreDeterministic(t *testing.T) {
	registry := NewRegistry(2)
	if err := registry.Add(TransactionRecord{TransactionID: "tx_1", SnapshotID: "snapshot_1", ExtensionGeneration: "generation_1", State: TransactionRestoreRequired}); err != nil {
		t.Fatal(err)
	}

	record, err := registry.BeginRestore("tx_1", "generation_1")
	if err != nil || record.State != TransactionRestoring {
		t.Fatalf("BeginRestore() record=%+v error=%v", record, err)
	}
	if err := registry.RestoreFailed("tx_1"); err != nil {
		t.Fatal(err)
	}
	record, _ = registry.Get("tx_1")
	if record.State != TransactionRestoreRequired {
		t.Fatalf("state after failure = %q", record.State)
	}
	if err := registry.CompleteRestore("tx_1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Get("tx_1"); ok {
		t.Fatal("completed transaction remains in registry")
	}
}

func recoveryRecord(id string, createdAt time.Time) TransactionRecord {
	return TransactionRecord{
		TransactionID: id, SnapshotID: "snapshot_" + id, ExtensionGeneration: "generation_1",
		State: TransactionRestoreRequired, CreatedAt: createdAt,
	}
}
