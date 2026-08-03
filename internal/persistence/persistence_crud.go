// Purpose: Implements Save, Load, Delete, and List operations for the session persistence store.
// Why: Separates CRUD logic from store initialization, validation, and maintenance.
package persistence

import (
	"errors"
	"fmt"
	"os"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

func (s *SessionStore) Save(namespace, key string, data []byte) error {
	_, filePath, err := s.validatedPath(namespace, key)
	if err != nil {
		return err
	}
	if len(data) > maxFileSize {
		return fmt.Errorf("data exceeds maximum file size (1MB): %d bytes", len(data))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentSize, sizeErr := s.projectSize()
	if sizeErr != nil {
		s.reportRecovery("session_store_quota_state", "Session storage usage could not be measured, so the write was blocked to protect the configured quota.", "Check permissions for the project .kaboom directory, then retry.")
		return errors.New("session_storage_quota_check_failed")
	}
	existingSize := int64(0)
	if existing, readErr := s.filesystem().ReadFile(filePath); readErr == nil {
		existingSize = int64(len(existing))
	} else if !os.IsNotExist(readErr) {
		s.reportRecovery("session_store_quota_state", "Existing session state could not be measured, so the write was blocked to protect the configured quota.", "Check permissions for the project .kaboom directory, then retry.")
		return errors.New("session_storage_quota_check_failed")
	}
	projectedSize := currentSize - existingSize + int64(len(data))
	if projectedSize > maxProjectSize {
		s.reportRecovery("session_store_quota_state", "The session storage quota is full, so the previous durable value remains active.", "Delete unused stored session entries, then retry.")
		return fmt.Errorf("project size limit exceeded (10MB): projected=%d", projectedSize)
	}
	statediag.Resolve(s.diagnostics, "session_store_quota_state")
	return s.writeStateFile(
		filePath,
		data,
		"session_store_write_state",
		"Session state could not be persisted; the previous durable value remains active.",
		"Check permissions, available disk space, and the project .kaboom directory, then retry.",
	)
}

func (s *SessionStore) Load(namespace, key string) ([]byte, error) {
	_, filePath, err := s.validatedPath(namespace, key)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	data, readErr := s.filesystem().ReadFile(filePath) // #nosec G304 -- path validated above
	if readErr != nil {
		if os.IsNotExist(readErr) {
			// EXPECTED_ABSENCE: the requested key has not been stored or was deleted.
			return nil, fmt.Errorf("key not found: %w", statediag.ErrAbsent)
		}
		s.reportRecovery("session_store_read_state", "Saved session state could not be read; the requested value is unavailable.", "Check permissions for the project .kaboom directory, then retry.")
		return nil, errors.New("session_state_read_failed")
	}
	statediag.Resolve(s.diagnostics, "session_store_read_state")
	return data, nil
}

func (s *SessionStore) List(namespace string) ([]string, error) {
	nsDir, err := s.validatedNsDir(namespace)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	keys, readErr := jsonKeysFromDir(s.filesystem(), nsDir)
	if readErr != nil {
		s.reportRecovery("session_store_list_state", "Saved session keys could not be listed; no incomplete result was returned.", "Check permissions for the project .kaboom directory, then retry.")
		return nil, errors.New("session_state_list_failed")
	}
	statediag.Resolve(s.diagnostics, "session_store_list_state")
	return keys, nil
}

func (s *SessionStore) Delete(namespace, key string) error {
	_, filePath, err := s.validatedPath(namespace, key)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.filesystem().Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			// EXPECTED_ABSENCE: deleting an unknown key is a caller-visible miss,
			// not a persisted-state health incident.
			return fmt.Errorf("key not found: %w", statediag.ErrAbsent)
		}
		s.reportRecovery("session_store_delete_state", "Saved session state could not be deleted; the previous durable value remains active.", "Check permissions for the project .kaboom directory, then retry.")
		return errors.New("session_state_delete_failed")
	}
	statediag.Resolve(s.diagnostics, "session_store_delete_state")
	return nil
}
