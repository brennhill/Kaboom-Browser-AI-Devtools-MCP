// registry_store.go — Atomically persists redacted fixture recovery records.

package qafixture

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const registryVersion = 1

type registryDocument struct {
	Version int                 `json:"version"`
	Records []TransactionRecord `json:"records"`
}

type RegistryStore struct {
	path   string
	limit  int
	rename func(string, string) error
}

func NewRegistryStore(path string, limit int) *RegistryStore {
	return &RegistryStore{path: path, limit: limit, rename: os.Rename}
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
	if json.Unmarshal(data, &document) != nil || document.Version != registryVersion || !validRecords(document.Records) {
		if store.rename(store.path, store.path+".corrupt") != nil {
			return registry, "fixture_transaction_registry_corrupt_quarantine_failed"
		}
		return registry, "fixture_transaction_registry_corrupt"
	}
	for _, record := range document.Records {
		registry.Add(record)
	}
	return registry, ""
}

func (store *RegistryStore) Save(registry *Registry) error {
	document := registryDocument{Version: registryVersion, Records: registry.Records()}
	data, err := json.Marshal(document)
	if err != nil {
		return errors.New("fixture_transaction_registry_encode_failed")
	}
	dir := filepath.Dir(store.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.New("fixture_transaction_registry_directory_failed")
	}
	temporary, err := os.CreateTemp(dir, ".fixture-transactions-*")
	if err != nil {
		return errors.New("fixture_transaction_registry_write_failed")
	}
	temporaryPath := temporary.Name()
	if err := temporary.Chmod(0o600); err != nil {
		if cleanupErr := discardOpenTemporary(temporary, temporaryPath); cleanupErr != nil {
			return cleanupErr
		}
		return errors.New("fixture_transaction_registry_write_failed")
	}
	if _, err := temporary.Write(data); err != nil {
		if cleanupErr := discardOpenTemporary(temporary, temporaryPath); cleanupErr != nil {
			return cleanupErr
		}
		return errors.New("fixture_transaction_registry_write_failed")
	}
	if err := temporary.Sync(); err != nil {
		if cleanupErr := discardOpenTemporary(temporary, temporaryPath); cleanupErr != nil {
			return cleanupErr
		}
		return errors.New("fixture_transaction_registry_write_failed")
	}
	if err := temporary.Close(); err != nil {
		if cleanupErr := discardClosedTemporary(temporaryPath); cleanupErr != nil {
			return cleanupErr
		}
		return errors.New("fixture_transaction_registry_write_failed")
	}
	if err := store.rename(temporaryPath, store.path); err != nil {
		if cleanupErr := discardClosedTemporary(temporaryPath); cleanupErr != nil {
			return cleanupErr
		}
		return errors.New("fixture_transaction_registry_replace_failed")
	}
	return nil
}

func discardOpenTemporary(temporary *os.File, path string) error {
	if temporary.Close() != nil {
		return errors.New("fixture_transaction_registry_cleanup_failed")
	}
	return discardClosedTemporary(path)
}

func discardClosedTemporary(path string) error {
	if err := os.Remove(path); err != nil {
		// EXPECTED_ABSENCE: a failed atomic rename may already have consumed the temporary path.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return errors.New("fixture_transaction_registry_cleanup_failed")
	}
	return nil
}

func validRecords(records []TransactionRecord) bool {
	for _, record := range records {
		if record.TransactionID == "" || record.SnapshotID == "" || record.ExtensionGeneration == "" {
			return false
		}
		if record.State != TransactionRestoreRequired && record.State != TransactionRestoring {
			return false
		}
	}
	return true
}
