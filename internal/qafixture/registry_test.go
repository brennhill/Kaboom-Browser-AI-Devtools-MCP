// registry_test.go — Tests bounded, generation-aware fixture transaction state.

package qafixture

import (
	"reflect"
	"testing"
	"time"
)

func TestRegistryEvictsOldestTransactionAtCapacity(t *testing.T) {
	registry := NewRegistry(2)
	registry.Add(TransactionRecord{TransactionID: "old", CreatedAt: time.Unix(1, 0)})
	registry.Add(TransactionRecord{TransactionID: "new", CreatedAt: time.Unix(2, 0)})
	registry.Add(TransactionRecord{TransactionID: "newest", CreatedAt: time.Unix(3, 0)})

	if _, ok := registry.Get("old"); ok {
		t.Fatal("oldest transaction was not evicted")
	}
	if registry.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", registry.Len())
	}
}

func TestRegistryRecordsHaveStableRecoveryOrder(t *testing.T) {
	registry := NewRegistry(3)
	registry.Add(TransactionRecord{TransactionID: "b", CreatedAt: time.Unix(2, 0)})
	registry.Add(TransactionRecord{TransactionID: "c", CreatedAt: time.Unix(1, 0)})
	registry.Add(TransactionRecord{TransactionID: "a", CreatedAt: time.Unix(1, 0)})
	records := registry.Records()
	got := []string{records[0].TransactionID, records[1].TransactionID, records[2].TransactionID}
	if !reflect.DeepEqual(got, []string{"a", "c", "b"}) {
		t.Fatalf("recovery order = %v", got)
	}
}

func TestRegistryRejectsRestoreFromStaleExtensionGeneration(t *testing.T) {
	registry := NewRegistry(2)
	registry.Add(TransactionRecord{
		TransactionID:       "tx_1",
		SnapshotID:          "opaque_1",
		ExtensionGeneration: "generation_1",
		State:               TransactionRestoreRequired,
	})

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
	registry.Add(TransactionRecord{TransactionID: "tx_1", ExtensionGeneration: "generation_1", State: TransactionRestoreRequired})

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
