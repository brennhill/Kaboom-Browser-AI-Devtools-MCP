// store.go — Atomic local persistence for content-addressed QA evidence.

package verification

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxStoredArtifactBytes = MaxEvidenceBytes + 4096

var ErrInvalidEvidenceID = errors.New("invalid evidence_id")

type Store struct {
	Dir string
}

func (s Store) Save(artifact EvidenceArtifact) error {
	if !evidenceIsAuthentic(artifact) {
		return fmt.Errorf("evidence artifact does not match its content address")
	}
	path, err := s.path(artifact.Ref.ID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		_, loadErr := s.Load(artifact.Ref.ID)
		return loadErr
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect evidence artifact: %w", err)
	}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return fmt.Errorf("encode evidence artifact: %w", err)
	}
	if len(encoded) > maxStoredArtifactBytes {
		return fmt.Errorf("encoded evidence artifact exceeds storage limit")
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}
	temporary, err := os.CreateTemp(s.Dir, ".evidence-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary evidence artifact: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure temporary evidence artifact: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return fmt.Errorf("write evidence artifact: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync evidence artifact: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close evidence artifact: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish evidence artifact: %w", err)
	}
	return nil
}

func (s Store) Load(id string) (EvidenceArtifact, error) {
	path, err := s.path(id)
	if err != nil {
		return EvidenceArtifact{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return EvidenceArtifact{}, fmt.Errorf("open evidence artifact: %w", err)
	}
	defer file.Close()
	encoded, err := io.ReadAll(io.LimitReader(file, maxStoredArtifactBytes+1))
	if err != nil {
		return EvidenceArtifact{}, fmt.Errorf("read evidence artifact: %w", err)
	}
	if len(encoded) > maxStoredArtifactBytes {
		return EvidenceArtifact{}, fmt.Errorf("stored evidence artifact exceeds size limit")
	}
	var artifact EvidenceArtifact
	if err := json.Unmarshal(encoded, &artifact); err != nil {
		return EvidenceArtifact{}, fmt.Errorf("decode evidence artifact: %w", err)
	}
	if artifact.Ref.ID != id || !evidenceIsAuthentic(artifact) {
		return EvidenceArtifact{}, fmt.Errorf("stored evidence artifact failed content-address validation")
	}
	return artifact, nil
}

func (s Store) path(id string) (string, error) {
	if strings.TrimSpace(s.Dir) == "" {
		return "", fmt.Errorf("evidence directory is required")
	}
	if !strings.HasPrefix(id, "sha256:") || len(id) != len("sha256:")+64 {
		return "", ErrInvalidEvidenceID
	}
	hexDigest := strings.TrimPrefix(id, "sha256:")
	if _, err := hex.DecodeString(hexDigest); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidEvidenceID, err)
	}
	return filepath.Join(s.Dir, hexDigest+".json"), nil
}
