// Purpose: Defines per-client cursor/state model and client-ID derivation utilities.
// Why: Keeps per-client mutable state behavior separate from registry orchestration.
// Docs: docs/features/feature/request-session-correlation/index.md

package clientreg

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// ============================================
// BufferCursor
// ============================================

// BufferCursor tracks a client's read position in a ring buffer.
// The timestamp allows detecting when the buffer has wrapped and
// the position is no longer valid (all data at that position has been evicted).
type BufferCursor struct {
	Position  int64     // Monotonic position in the buffer (total items ever added)
	Timestamp time.Time // When this position was last valid
}

// ============================================
// ClientState
// ============================================

// ClientState holds per-client isolated state.
// Each client gets its own cursors, checkpoint namespace, and noise rules.
type ClientState struct {
	mu sync.RWMutex

	// Client identification
	ID         string    // SHA256(CWD)[:12]
	CWD        string    // Original working directory
	CreatedAt  time.Time // When client first connected
	LastSeenAt time.Time // Last activity (for LRU eviction)

	// Buffer cursors - track where each client has read up to
	WSEventCursor        BufferCursor
	NetworkBodyCursor    BufferCursor
	EnhancedActionCursor BufferCursor

	// Per-client checkpoint namespace prefix (clientId + ":")
	// Checkpoints are stored as "clientId:checkpointName" in the global store
	CheckpointPrefix string
	now              func() time.Time
}

// NewClientState creates a new client state for the given CWD.
func NewClientState(cwd string) *ClientState {
	now := time.Now()
	id := DeriveClientID(cwd)
	return &ClientState{
		ID:               id,
		CWD:              cwd,
		CreatedAt:        now,
		LastSeenAt:       now,
		CheckpointPrefix: id + ":",
		now:              time.Now,
	}
}

// Touch updates LastSeenAt to current time.
func (cs *ClientState) Touch() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.LastSeenAt = cs.now()
}

// GetLastSeen returns when this client was last active.
func (cs *ClientState) GetLastSeen() time.Time {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.LastSeenAt
}

// UpdateWSCursor updates the WebSocket event cursor.
func (cs *ClientState) UpdateWSCursor(cursor BufferCursor) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.WSEventCursor = cursor
}

// GetWSCursor returns the current WebSocket event cursor.
func (cs *ClientState) GetWSCursor() BufferCursor {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.WSEventCursor
}

// UpdateNetworkCursor updates the network body cursor.
func (cs *ClientState) UpdateNetworkCursor(cursor BufferCursor) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.NetworkBodyCursor = cursor
}

// GetNetworkCursor returns the current network body cursor.
func (cs *ClientState) GetNetworkCursor() BufferCursor {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.NetworkBodyCursor
}

// UpdateActionCursor updates the enhanced action cursor.
func (cs *ClientState) UpdateActionCursor(cursor BufferCursor) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.EnhancedActionCursor = cursor
}

// GetActionCursor returns the current enhanced action cursor.
func (cs *ClientState) GetActionCursor() BufferCursor {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.EnhancedActionCursor
}

// ============================================
// Helper Functions
// ============================================

// DeriveClientID generates a stable client ID from the working directory.
// Uses SHA256 hash truncated to 12 hex characters for uniqueness while
// remaining human-readable.
func DeriveClientID(cwd string) string {
	if cwd == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(cwd))
	return hex.EncodeToString(hash[:])[:clientIDLength]
}
