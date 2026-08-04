// snapshot.go — Snapshot capture, bounded retention and lookup.
// Purpose: Manages security snapshot creation, retention, and lookup.
// Why: Separates stateful snapshot lifecycle from diff computation logic.
// Docs: docs/features/feature/security-hardening/index.md

package diff

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/httpsec"
)

// TakeSnapshot captures and stores a named snapshot.
//
// Invariants:
// - Existing snapshot names are replaced atomically while preserving LRU order semantics.
//
// Failure semantics:
// - Name validation failure returns error and leaves store unchanged.
func (m *Manager) TakeSnapshot(name string, bodies []types.NetworkBody) (*Snapshot, error) {
	if err := validateSnapshotName(name); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.snapshots[name]; exists {
		m.removeFromOrder(name)
	}
	m.evictOldest()

	snapshot := m.newEmptySnapshot(name)
	populateSnapshotFromBodies(snapshot, bodies)

	m.snapshots[name] = snapshot
	m.order = append(m.order, name)

	return snapshot, nil
}

// ListSnapshots returns a read-only summary view in insertion/LRU order.
//
// Failure semantics:
// - Expired entries are reported with Expired=true; they are not auto-deleted here.
func (m *Manager) ListSnapshots() []SnapshotListEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]SnapshotListEntry, 0, len(m.order))
	for _, name := range m.order {
		snapshot, ok := m.snapshots[name]
		if !ok {
			continue
		}
		entries = append(entries, SnapshotListEntry{
			Name:    snapshot.Name,
			TakenAt: snapshot.TakenAt.Format(time.RFC3339),
			Age:     formatDuration(m.now().Sub(snapshot.TakenAt)),
			Expired: m.isExpired(snapshot),
		})
	}
	return entries
}

func validateSnapshotName(name string) error {
	if name == "" {
		return fmt.Errorf("snapshot name cannot be empty")
	}
	if name == "current" {
		return fmt.Errorf("snapshot name 'current' is reserved")
	}
	if len(name) > 50 {
		return fmt.Errorf("snapshot name exceeds 50 characters")
	}
	return nil
}

func (m *Manager) newEmptySnapshot(name string) *Snapshot {
	return &Snapshot{
		Name:      name,
		TakenAt:   m.now(),
		Headers:   make(map[string]map[string]string),
		Cookies:   make(map[string][]Cookie),
		Auth:      make(map[string]bool),
		Transport: make(map[string]string),
	}
}

func populateSnapshotFromBodies(snapshot *Snapshot, bodies []types.NetworkBody) {
	for _, body := range bodies {
		origin := extractSnapshotOrigin(body.URL)
		populateHeaders(snapshot, origin, body)
		populateCookies(snapshot, origin, body)
		snapshot.Auth[body.Method+" "+body.URL] = body.HasAuthHeader
		if scheme := extractScheme(body.URL); scheme != "" {
			snapshot.Transport[origin] = scheme
		}
	}
}

func populateHeaders(snapshot *Snapshot, origin string, body types.NetworkBody) {
	if !httpsec.IsHTMLResponse(body) || body.ResponseHeaders == nil {
		return
	}
	if snapshot.Headers[origin] == nil {
		snapshot.Headers[origin] = make(map[string]string)
	}
	for _, header := range trackedHeaders {
		if value, ok := body.ResponseHeaders[header]; ok && value != "" {
			snapshot.Headers[origin][header] = value
		}
	}
}

func populateCookies(snapshot *Snapshot, origin string, body types.NetworkBody) {
	if body.ResponseHeaders == nil {
		return
	}
	setCookie, ok := body.ResponseHeaders["Set-Cookie"]
	if !ok || setCookie == "" {
		return
	}
	cookies := parseSnapshotCookies(setCookie)
	if len(cookies) > 0 {
		snapshot.Cookies[origin] = append(snapshot.Cookies[origin], cookies...)
	}
}

func (m *Manager) isExpired(snapshot *Snapshot) bool {
	return m.now().Sub(snapshot.TakenAt) > m.ttl
}

func (m *Manager) removeFromOrder(name string) {
	for i, current := range m.order {
		if current == name {
			newOrder := make([]string, len(m.order)-1)
			copy(newOrder, m.order[:i])
			copy(newOrder[i:], m.order[i+1:])
			m.order = newOrder
			return
		}
	}
}

func (m *Manager) evictOldest() {
	for len(m.order) >= m.maxSnaps {
		oldest := m.order[0]
		newOrder := make([]string, len(m.order)-1)
		copy(newOrder, m.order[1:])
		m.order = newOrder
		delete(m.snapshots, oldest)
	}
}

func (m *Manager) resolveSnapshot(name string) (*Snapshot, error) {
	snapshot, ok := m.snapshots[name]
	if !ok {
		return nil, fmt.Errorf("snapshot %q not found", name)
	}
	if m.isExpired(snapshot) {
		return nil, fmt.Errorf("snapshot %q has expired (TTL: %v)", name, m.ttl)
	}
	return snapshot, nil
}

func (m *Manager) resolveToSnapshot(toName string, currentBodies []types.NetworkBody) (*Snapshot, error) {
	if toName == "" || toName == "current" {
		snapshot := m.newEmptySnapshot("current")
		populateSnapshotFromBodies(snapshot, currentBodies)
		return snapshot, nil
	}
	return m.resolveSnapshot(toName)
}
