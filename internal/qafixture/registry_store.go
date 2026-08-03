// registry_store.go — Atomically persists redacted fixture recovery records.

package qafixture

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
)

const registryVersion = 1

type registryDocument struct {
	Version int                 `json:"version"`
	Records []TransactionRecord `json:"records"`
}

type RegistryStore struct {
	path      string
	limit     int
	writeFile func(string, []byte, os.FileMode) error
	moveFile  func(string, string) error
}

func NewRegistryStore(path string, limit int) *RegistryStore {
	return &RegistryStore{path: path, limit: limit, writeFile: statefile.Write, moveFile: statefile.Move}
}

func (store *RegistryStore) Load() (*Registry, string) {
	registry := NewRegistry(store.limit)
	data, err := os.ReadFile(store.path) // #nosec G304 -- path is provided by the runtime state owner.
	if errors.Is(err, os.ErrNotExist) {
		return registry, ""
	}
	if err != nil {
		return registry, "fixture_transaction_registry_unreadable"
	}
	var document registryDocument
	if json.Unmarshal(data, &document) != nil || document.Version != registryVersion || len(document.Records) > registry.limit || !validRecords(document.Records) {
		if err := store.moveFile(store.path, store.path+".corrupt"); err != nil {
			if statefile.FailureStage(err) == statefile.StageDirectorySync {
				return registry, "fixture_transaction_registry_corrupt_quarantine_sync_failed"
			}
			return registry, "fixture_transaction_registry_corrupt_quarantine_failed"
		}
		return registry, "fixture_transaction_registry_corrupt"
	}
	for _, record := range document.Records {
		if record.State == TransactionRestoring {
			record.State = TransactionRestoreRequired
		}
		if registry.Add(record) != nil {
			return NewRegistry(store.limit), "fixture_transaction_registry_corrupt"
		}
	}
	return registry, ""
}

func (store *RegistryStore) Save(registry *Registry) error {
	document := registryDocument{Version: registryVersion, Records: registry.Records()}
	data, err := json.Marshal(document)
	if err != nil {
		return errors.New("fixture_transaction_registry_encode_failed")
	}
	if err := store.writeFile(store.path, data, 0o600); err != nil {
		if statefile.HasFailureStage(err, statefile.StageCleanup) {
			return errors.New("fixture_transaction_registry_cleanup_failed")
		}
		switch statefile.FailureStage(err) {
		case statefile.StageMkdir:
			return errors.New("fixture_transaction_registry_directory_failed")
		case statefile.StageRename:
			return errors.New("fixture_transaction_registry_replace_failed")
		case statefile.StageDirectorySync:
			return errors.New("fixture_transaction_registry_directory_sync_failed")
		default:
			return errors.New("fixture_transaction_registry_write_failed")
		}
	}
	return nil
}

func validRecords(records []TransactionRecord) bool {
	transactionIDs := make(map[string]struct{}, len(records))
	for _, record := range records {
		if !validOpaqueID(record.TransactionID) || !validOpaqueID(record.CorrelationID) || !validOpaqueID(record.SnapshotID) || !validOpaqueID(record.ExtensionGeneration) {
			return false
		}
		if _, duplicate := transactionIDs[record.TransactionID]; duplicate {
			return false
		}
		transactionIDs[record.TransactionID] = struct{}{}
		if record.State != TransactionRestoreRequired && record.State != TransactionRestoring {
			return false
		}
	}
	return true
}

func validOpaqueID(value string) bool {
	if value == "" || len(value) > 160 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}
