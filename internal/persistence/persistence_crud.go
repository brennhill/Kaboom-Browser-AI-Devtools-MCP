// Purpose: Implements Save, Load, Delete, and List operations for the session persistence store.
// Why: Separates CRUD logic from store initialization, validation, and maintenance.
package persistence

import (
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
	if sizeErr == nil && currentSize+int64(len(data)) > maxProjectSize {
		return fmt.Errorf("project size limit exceeded (10MB): current=%d, adding=%d", currentSize, len(data))
	}
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

	data, readErr := os.ReadFile(filePath) // #nosec G304 -- path validated above
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil, fmt.Errorf("key not found: %w: %s/%s", statediag.ErrAbsent, namespace, key)
		}
		return nil, fmt.Errorf("failed to read %s/%s: %w", namespace, key, readErr)
	}
	return data, nil
}

func (s *SessionStore) List(namespace string) ([]string, error) {
	nsDir, err := s.validatedNsDir(namespace)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return jsonKeysFromDir(nsDir)
}

func (s *SessionStore) Delete(namespace, key string) error {
	_, filePath, err := s.validatedPath(namespace, key)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete: %s/%s: %w", namespace, key, err)
	}
	return nil
}
