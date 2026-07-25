// manager.go — PTY session manager: create, get, destroy sessions with auth tokens.
// Why: Centralizes session lifecycle and token-based access control for the terminal WebSocket endpoint.
// Docs: docs/features/feature/terminal/index.md

package pty

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

// SessionKey identifies a session by repository path and agent type,
// allowing multiple providers (e.g., Claude and Codex) to coexist for the same repo.
type SessionKey struct {
	RepoPath  string
	AgentType string
}

// maxSessions is the maximum number of concurrent PTY sessions.
// Each session spawns a real OS process, so this must be bounded.
const maxSessions = 10

// ErrMaxSessions is returned when the session limit is reached.
var ErrMaxSessions = errors.New("pty: maximum concurrent sessions reached")

// Manager manages PTY sessions with token-based authentication.
type Manager struct {
	mu        sync.RWMutex
	sessions  map[string]*Session   // keyed by session ID
	tokens    map[string]string     // token → session ID
	repoIndex map[SessionKey]string // (repo, agent) → session ID

	// spawn creates a session from a config. Defaults to the real Spawn; tests
	// inject a fake so the Manager's token/index/limit bookkeeping can be
	// exercised deterministically without launching real OS processes.
	spawn func(SpawnConfig) (*Session, error)
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{
		sessions:  make(map[string]*Session),
		tokens:    make(map[string]string),
		repoIndex: make(map[SessionKey]string),
		spawn:     Spawn,
	}
}

// StartConfig is the configuration for starting a new terminal session.
type StartConfig struct {
	ID        string   // Session ID (default: "default").
	Cmd       string   // CLI binary.
	Args      []string // CLI arguments.
	Dir       string   // Working directory.
	Env       []string // Extra environment variables.
	Cols      uint16   // Terminal columns.
	Rows      uint16   // Terminal rows.
	RepoPath  string   // Repository path (for repo+agent indexing).
	AgentType string   // Agent type (e.g., "claude", "codex").
}

// StartResult is returned after successfully starting a session.
type StartResult struct {
	SessionID string
	Token     string
	Pid       int
}

// ErrSessionExists is returned when a session ID is already in use.
var ErrSessionExists = errors.New("pty: session already exists")

// ErrSessionNotFound is returned when a session ID is not found.
var ErrSessionNotFound = errors.New("pty: session not found")

// ErrInvalidToken is returned when an authentication token is invalid.
var ErrInvalidToken = errors.New("pty: invalid token")

// Start creates and starts a new PTY session. Returns the session token for WebSocket auth.
func (m *Manager) Start(cfg StartConfig) (*StartResult, error) {
	if cfg.ID == "" {
		cfg.ID = "default"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[cfg.ID]; exists {
		return nil, fmt.Errorf("%w: %s", ErrSessionExists, cfg.ID)
	}

	if len(m.sessions) >= maxSessions {
		return nil, fmt.Errorf("%w: limit %d", ErrMaxSessions, maxSessions)
	}

	sess, err := m.spawn(SpawnConfig{
		ID:   cfg.ID,
		Cmd:  cfg.Cmd,
		Args: cfg.Args,
		Dir:  cfg.Dir,
		Env:  cfg.Env,
		Cols: cfg.Cols,
		Rows: cfg.Rows,
	})
	if err != nil {
		return nil, err
	}

	token, err := generateToken()
	if err != nil {
		sess.Close()
		return nil, fmt.Errorf("generate token: %w", err)
	}

	m.sessions[cfg.ID] = sess
	m.tokens[token] = cfg.ID
	if cfg.RepoPath != "" {
		key := SessionKey{RepoPath: cfg.RepoPath, AgentType: cfg.AgentType}
		m.repoIndex[key] = cfg.ID
	}

	return &StartResult{
		SessionID: cfg.ID,
		Token:     token,
		Pid:       sess.Pid(),
	}, nil
}

// GetByToken returns the session associated with the given auth token.
func (m *Manager) GetByToken(token string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionID, ok := m.tokens[token]
	if !ok {
		return nil, ErrInvalidToken
	}
	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return sess, nil
}

// GetTokenForSession returns the auth token for a session ID, or empty string if not found.
func (m *Manager) GetTokenForSession(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for token, sid := range m.tokens {
		if sid == id {
			return token
		}
	}
	return ""
}

// Get returns the session by ID.
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}
	return sess, nil
}

// Stop destroys a session by ID, cleaning up PTY and child process.
// Removes map entries under lock, then closes the session outside the lock
// so that slow Close() calls (up to 2s) don't block concurrent reads.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}

	// Remove token mapping.
	for token, sid := range m.tokens {
		if sid == id {
			delete(m.tokens, token)
			break
		}
	}
	// Remove repo-agent mapping.
	for key, sid := range m.repoIndex {
		if sid == id {
			delete(m.repoIndex, key)
			break
		}
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	return sess.Close()
}

// StopAll destroys all active sessions. Called during daemon shutdown.
// Collects sessions under lock, clears maps, then closes sessions outside the
// lock so that slow Close() calls (up to 2s each) don't block concurrent reads.
func (m *Manager) StopAll() {
	m.mu.Lock()
	toClose := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		toClose = append(toClose, sess)
	}
	m.sessions = make(map[string]*Session)
	m.tokens = make(map[string]string)
	m.repoIndex = make(map[SessionKey]string)
	m.mu.Unlock()

	for _, sess := range toClose {
		_ = sess.Close()
	}
}

// List returns the IDs of all active sessions.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// GetByRepoAgent returns the session for a given repo path and agent type.
func (m *Manager) GetByRepoAgent(repoPath, agentType string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := SessionKey{RepoPath: repoPath, AgentType: agentType}
	sid, ok := m.repoIndex[key]
	if !ok {
		return nil, fmt.Errorf("%w: repo=%s agent=%s", ErrSessionNotFound, repoPath, agentType)
	}
	sess, ok := m.sessions[sid]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sid)
	}
	return sess, nil
}

// generateToken creates a cryptographically random 32-byte hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
