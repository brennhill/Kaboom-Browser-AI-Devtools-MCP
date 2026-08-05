// Purpose: Defines session storage state and its public persistence contracts.
// Why: Preserves investigative context across daemon restarts and multi-session workflows.
// Docs: docs/features/feature/persistent-memory/index.md

// Package persistence provides validated namespace/key storage, metadata
// handling, and background dirty flushing for session state.
package persistence

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

const (
	maxFileSize          = 1 * 1024 * 1024
	maxProjectSize       = 10 * 1024 * 1024
	maxErrorHistory      = 500
	staleErrorThreshold  = 30 * 24 * time.Hour
	defaultFlushInterval = 30 * time.Second
	dirPermissions       = 0o755
	filePermissions      = 0o644
)

type SessionStore struct {
	mu          sync.RWMutex
	projectPath string
	projectDir  string
	meta        *ProjectMeta

	dirty   map[string][]byte
	dirtyMu sync.Mutex

	flushInterval time.Duration
	stopCh        chan struct{}
	stopped       bool
	diagnostics   statediag.Reporter
	files         sessionFilesystem
}

type ProjectMeta struct {
	ProjectID    string    `json:"project_id"`
	ProjectPath  string    `json:"project_path"`
	FirstCreated time.Time `json:"first_created"`
	LastSession  time.Time `json:"last_session"`
	SessionCount int       `json:"session_count"`
}

type SessionContext struct {
	ProjectID    string              `json:"project_id"`
	SessionCount int                 `json:"session_count"`
	Baselines    []string            `json:"baselines"`
	NoiseConfig  map[string]any      `json:"noise_config,omitempty"`
	ErrorHistory []ErrorHistoryEntry `json:"error_history"`
	APISchema    map[string]any      `json:"api_schema,omitempty"`
	Performance  map[string]any      `json:"performance,omitempty"`
}

type ErrorHistoryEntry struct {
	Fingerprint string    `json:"fingerprint"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Count       int       `json:"count"`
	Resolved    bool      `json:"resolved"`
	ResolvedAt  time.Time `json:"resolved_at,omitempty"`
}

type StoreStats struct {
	TotalBytes   int64          `json:"total_bytes"`
	SessionCount int            `json:"session_count"`
	Namespaces   map[string]int `json:"namespaces"`
}

type SessionStoreArgs struct {
	Action    string          `json:"action"`
	Namespace string          `json:"namespace,omitempty"`
	Key       string          `json:"key,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}
