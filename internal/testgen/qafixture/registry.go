// registry.go — Owns bounded fixture transaction recovery state.

package qafixture

import (
	"errors"
	"sort"
	"sync"
	"time"
)

const (
	TransactionRestoreRequired = "restore_required"
	TransactionRestoring       = "restoring"
)

// TransactionRecord contains only opaque recovery references and non-sensitive
// mutation metadata. Raw fixtures and captured browser values never belong here.
type TransactionRecord struct {
	TransactionID       string         `json:"transaction_id"`
	CorrelationID       string         `json:"correlation_id"`
	SnapshotID          string         `json:"snapshot_id"`
	ExtensionGeneration string         `json:"extension_generation"`
	State               string         `json:"state"`
	CreatedAt           time.Time      `json:"created_at"`
	Mutations           MutationCounts `json:"mutations"`
}

// Registry is the single in-memory owner of active fixture transactions.
type Registry struct {
	mu      sync.RWMutex
	limit   int
	records map[string]TransactionRecord
}

func NewRegistry(limit int) *Registry {
	if limit < 1 {
		limit = 1
	}
	return &Registry{limit: limit, records: make(map[string]TransactionRecord, limit)}
}

func (registry *Registry) Add(record TransactionRecord) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if record.TransactionID == "" || record.SnapshotID == "" || record.ExtensionGeneration == "" ||
		(record.State != TransactionRestoreRequired && record.State != TransactionRestoring) {
		return errors.New("fixture_transaction_record_invalid")
	}
	if _, exists := registry.records[record.TransactionID]; !exists && len(registry.records) >= registry.limit {
		return errors.New("fixture_transaction_registry_full")
	}
	registry.records[record.TransactionID] = record
	return nil
}

func (registry *Registry) Get(transactionID string) (TransactionRecord, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	record, ok := registry.records[transactionID]
	return record, ok
}

func (registry *Registry) Len() int {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return len(registry.records)
}

func (registry *Registry) Records() []TransactionRecord {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	records := make([]TransactionRecord, 0, len(registry.records))
	for _, record := range registry.records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].TransactionID < records[j].TransactionID
		}
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	return records
}

func (registry *Registry) BeginRestore(transactionID, generation string) (TransactionRecord, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	record, ok := registry.records[transactionID]
	if !ok {
		return TransactionRecord{}, errors.New("fixture_transaction_not_found")
	}
	if record.ExtensionGeneration != generation {
		return TransactionRecord{}, errors.New("fixture_transaction_generation_mismatch")
	}
	if record.State != TransactionRestoreRequired {
		return TransactionRecord{}, errors.New("fixture_transaction_restore_in_progress")
	}
	record.State = TransactionRestoring
	registry.records[transactionID] = record
	return record, nil
}

func (registry *Registry) RestoreFailed(transactionID string) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	record, ok := registry.records[transactionID]
	if !ok {
		return errors.New("fixture_transaction_not_found")
	}
	record.State = TransactionRestoreRequired
	registry.records[transactionID] = record
	return nil
}

func (registry *Registry) SetMutations(transactionID string, mutations MutationCounts) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	record, ok := registry.records[transactionID]
	if !ok {
		return errors.New("fixture_transaction_not_found")
	}
	record.Mutations = mutations
	registry.records[transactionID] = record
	return nil
}

func (registry *Registry) CompleteRestore(transactionID string) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	record, ok := registry.records[transactionID]
	if !ok {
		return errors.New("fixture_transaction_not_found")
	}
	if record.State != TransactionRestoring {
		return errors.New("fixture_transaction_restore_not_started")
	}
	delete(registry.records, transactionID)
	return nil
}

// Discard removes an uncommitted record during same-process compensation.
// Unlike CompleteRestore, absence is harmless because persistence never succeeded.
func (registry *Registry) Discard(transactionID string) {
	registry.mu.Lock()
	delete(registry.records, transactionID)
	registry.mu.Unlock()
}
