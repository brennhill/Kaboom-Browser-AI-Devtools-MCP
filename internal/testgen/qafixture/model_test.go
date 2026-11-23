// model_test.go — Deterministic model-based fixture recovery lifecycle coverage.

package qafixture

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestRegistryModelPreservesRecoveryObligations(t *testing.T) {
	for seed := int64(1); seed <= 100; seed++ {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			registry := NewRegistry(16)
			model := make(map[string]string)

			for step := 0; step < 200; step++ {
				id := fmt.Sprintf("tx-%d", rng.Intn(16))
				switch rng.Intn(4) {
				case 0:
					record := TransactionRecord{
						TransactionID: id, SnapshotID: "snapshot-" + id,
						ExtensionGeneration: "generation-1", State: TransactionRestoreRequired,
						CreatedAt: time.Unix(int64(step), 0),
					}
					if err := registry.Add(record); err == nil {
						model[id] = TransactionRestoreRequired
					}
				case 1:
					if _, err := registry.BeginRestore(id, "generation-1"); err == nil {
						model[id] = TransactionRestoring
					}
				case 2:
					if err := registry.RestoreFailed(id); err == nil {
						model[id] = TransactionRestoreRequired
					}
				case 3:
					prior, exists := model[id]
					err := registry.CompleteRestore(id)
					if exists && prior != TransactionRestoring && err == nil {
						t.Fatalf("seed=%d step=%d: recovery obligation %s disappeared without restoration proof", seed, step, id)
					}
					if err == nil {
						delete(model, id)
					}
				}

				if registry.Len() != len(model) {
					t.Fatalf("seed=%d step=%d: registry=%d model=%d", seed, step, registry.Len(), len(model))
				}
				for transactionID, expectedState := range model {
					record, ok := registry.Get(transactionID)
					if !ok || record.State != expectedState {
						t.Fatalf("seed=%d step=%d: %s state=%q want=%q", seed, step, transactionID, record.State, expectedState)
					}
				}
			}
		})
	}
}
