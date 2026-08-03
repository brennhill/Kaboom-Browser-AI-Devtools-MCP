// Purpose: Creates and initializes the session persistence store with project directory resolution.
// Why: Separates store construction and directory setup from CRUD and maintenance logic.
package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

type sessionFilesystem interface {
	MkdirAll(string, fs.FileMode) error
	ReadFile(string) ([]byte, error)
	ReadDir(string) ([]os.DirEntry, error)
	Remove(string) error
	Walk(string, filepath.WalkFunc) error
	WriteFile(string, []byte, fs.FileMode) error
}

type localSessionFilesystem struct{}

func (localSessionFilesystem) MkdirAll(path string, permissions fs.FileMode) error {
	return os.MkdirAll(path, permissions)
}
func (localSessionFilesystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (localSessionFilesystem) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}
func (localSessionFilesystem) Remove(path string) error { return os.Remove(path) }
func (localSessionFilesystem) Walk(root string, walkFn filepath.WalkFunc) error {
	return filepath.Walk(root, walkFn)
}
func (localSessionFilesystem) WriteFile(path string, data []byte, permissions fs.FileMode) error {
	return statefile.Write(path, data, permissions)
}

func (s *SessionStore) filesystem() sessionFilesystem {
	if s != nil && s.files != nil {
		return s.files
	}
	// EXPECTED_ABSENCE: read-only helper tests use a partial zero-value store;
	// production constructors always install the local filesystem boundary.
	return localSessionFilesystem{}
}

func NewSessionStore(projectPath string, diagnostics statediag.Reporter) (*SessionStore, error) {
	return NewSessionStoreWithInterval(projectPath, defaultFlushInterval, diagnostics)
}

func NewSessionStoreWithInterval(
	projectPath string,
	flushInterval time.Duration,
	diagnostics statediag.Reporter,
) (*SessionStore, error) {
	absPath, projectDir, err := resolveProjectDir(projectPath)
	if err != nil {
		return nil, err
	}
	return newSessionStoreInDir(absPath, projectDir, flushInterval, diagnostics)
}

func resolveProjectDir(projectPath string) (absPath, projectDir string, err error) {
	absPath, err = filepath.Abs(projectPath)
	if err != nil {
		return "", "", fmt.Errorf("invalid project path: %w", err)
	}
	if strings.Contains(absPath, "..") {
		return "", "", fmt.Errorf("project path contains '..': %s", absPath)
	}
	projectDir, err = state.ProjectDir(absPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve project directory: %w", err)
	}
	return absPath, projectDir, nil
}

func newSessionStoreInDir(
	projectPath, projectDir string,
	flushInterval time.Duration,
	diagnostics statediag.Reporter,
) (*SessionStore, error) {
	return newSessionStoreWithFilesystem(projectPath, projectDir, flushInterval, diagnostics, localSessionFilesystem{})
}

func newSessionStoreWithFilesystem(
	projectPath, projectDir string,
	flushInterval time.Duration,
	diagnostics statediag.Reporter,
	files sessionFilesystem,
) (*SessionStore, error) {
	s := &SessionStore{
		projectPath:   projectPath,
		projectDir:    projectDir,
		dirty:         make(map[string][]byte),
		flushInterval: flushInterval,
		stopCh:        make(chan struct{}),
		diagnostics:   diagnostics,
		files:         files,
	}

	if err := s.filesystem().MkdirAll(projectDir, dirPermissions); err != nil {
		s.reportRecovery("session_directory_state", "The project session directory could not be prepared, so persistence is unavailable.", "Check parent-directory permissions and available disk space, then restart Kaboom.")
		return nil, errors.New("session_directory_initialization_failed")
	}
	if err := s.loadOrCreateMeta(); err != nil {
		return nil, fmt.Errorf("failed to load meta: %w", err)
	}

	util.SafeGo(s.backgroundFlush)
	return s, nil
}

func (s *SessionStore) loadOrCreateMeta() error {
	metaPath := filepath.Join(s.projectDir, "meta.json")
	data, err := s.filesystem().ReadFile(metaPath) // #nosec G304 -- path is constructed from internal projectDir field // nosemgrep: go_filesystem_rule-fileread -- local persistence store I/O
	if err != nil {
		now := time.Now()
		s.meta = &ProjectMeta{
			ProjectID:    s.projectPath,
			ProjectPath:  s.projectPath,
			FirstCreated: now,
			LastSession:  now,
			SessionCount: 1,
		}
		if !os.IsNotExist(err) {
			s.reportRecovery(
				"session_metadata_state",
				"Project session metadata could not be read; a fresh in-memory session is active.",
				"Check permissions for the project .kaboom directory, then restart Kaboom.",
			)
			return nil
		}
		// EXPECTED_ABSENCE: this project has no session metadata on its first run.
		return s.saveMeta()
	}

	var meta ProjectMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		now := time.Now()
		s.meta = &ProjectMeta{
			ProjectID:    s.projectPath,
			ProjectPath:  s.projectPath,
			FirstCreated: now,
			LastSession:  now,
			SessionCount: 1,
		}
		s.reportRecovery(
			"session_metadata_state",
			"Project session metadata was malformed; a fresh session replaced it.",
			"Restart Kaboom after confirming the project .kaboom directory is writable.",
		)
		if saveErr := s.saveMeta(); saveErr != nil {
			return nil
		}
		return nil
	}

	meta.SessionCount++
	meta.LastSession = time.Now()
	s.meta = &meta
	if err := s.saveMeta(); err != nil {
		return err
	}
	statediag.Resolve(s.diagnostics, "session_metadata_state")
	return nil
}

func (s *SessionStore) reportRecovery(name, detail, fix string) {
	if s == nil || s.diagnostics == nil {
		return
	}
	s.diagnostics.Report(statediag.Diagnostic{Name: name, Detail: detail, Fix: fix})
}

func (s *SessionStore) saveMeta() error {
	metaPath := filepath.Join(s.projectDir, "meta.json")
	data, err := json.Marshal(s.meta)
	if err != nil {
		return err
	}
	return s.writeStateFile(
		metaPath,
		data,
		"session_metadata_write_state",
		"Project session metadata could not be persisted; the previous durable metadata remains active.",
		"Check permissions and available disk space for the project .kaboom directory, then retry.",
	)
}

func (s *SessionStore) writeStateFile(path string, data []byte, diagnosticName, detail, fix string) error {
	if err := s.filesystem().WriteFile(path, data, filePermissions); err != nil {
		s.reportRecovery(diagnosticName, detail, fix)
		return fmt.Errorf("session state persistence failed: %w", err)
	}
	statediag.Resolve(s.diagnostics, diagnosticName)
	return nil
}

func (s *SessionStore) GetMeta() ProjectMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.meta == nil {
		return ProjectMeta{}
	}
	return *s.meta
}
