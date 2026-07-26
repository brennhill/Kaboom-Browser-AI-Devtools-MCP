// Purpose: Manages security snapshot creation, retention, and lookup.
// Why: Separates stateful snapshot lifecycle from diff computation logic.
// Docs: docs/features/feature/security-hardening/index.md

package security

import (
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/httpsec"
)

// TakeSnapshot captures and stores a named snapshot.
//
// Invariants:
// - Existing snapshot names are replaced atomically while preserving LRU order semantics.
//
// Failure semantics:
// - Name validation failure returns error and leaves store unchanged.
func (m *SecurityDiffManager) TakeSnapshot(name string, bodies []capture.NetworkBody) (*SecuritySnapshot, error) {
	if err := validateSnapshotName(name); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.snapshots[name]; exists {
		m.removeFromOrder(name)
	}
	m.evictOldest()

	snapshot := newEmptySnapshot(name)
	populateSnapshotFromBodies(snapshot, bodies)

	m.snapshots[name] = snapshot
	m.order = append(m.order, name)

	return snapshot, nil
}

// ListSnapshots returns a read-only summary view in insertion/LRU order.
//
// Failure semantics:
// - Expired entries are reported with Expired=true; they are not auto-deleted here.
func (m *SecurityDiffManager) ListSnapshots() []SecuritySnapshotListEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]SecuritySnapshotListEntry, 0, len(m.order))
	for _, name := range m.order {
		snapshot, ok := m.snapshots[name]
		if !ok {
			continue
		}
		entries = append(entries, SecuritySnapshotListEntry{
			Name:    snapshot.Name,
			TakenAt: snapshot.TakenAt.Format(time.RFC3339),
			Age:     formatDuration(time.Since(snapshot.TakenAt)),
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

func newEmptySnapshot(name string) *SecuritySnapshot {
	return &SecuritySnapshot{
		Name:      name,
		TakenAt:   time.Now(),
		Headers:   make(map[string]map[string]string),
		Cookies:   make(map[string][]SecurityCookie),
		Auth:      make(map[string]bool),
		Transport: make(map[string]string),
	}
}

func populateSnapshotFromBodies(snapshot *SecuritySnapshot, bodies []capture.NetworkBody) {
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

func populateHeaders(snapshot *SecuritySnapshot, origin string, body capture.NetworkBody) {
	if !httpsec.IsHTMLResponse(body) || body.ResponseHeaders == nil {
		return
	}
	if snapshot.Headers[origin] == nil {
		snapshot.Headers[origin] = make(map[string]string)
	}
	for _, header := range trackedSecurityHeaders {
		if value, ok := body.ResponseHeaders[header]; ok && value != "" {
			snapshot.Headers[origin][header] = value
		}
	}
}

func populateCookies(snapshot *SecuritySnapshot, origin string, body capture.NetworkBody) {
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

func (m *SecurityDiffManager) isExpired(snapshot *SecuritySnapshot) bool {
	return time.Since(snapshot.TakenAt) > m.ttl
}

func (m *SecurityDiffManager) removeFromOrder(name string) {
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

func (m *SecurityDiffManager) evictOldest() {
	for len(m.order) >= m.maxSnaps {
		oldest := m.order[0]
		newOrder := make([]string, len(m.order)-1)
		copy(newOrder, m.order[1:])
		m.order = newOrder
		delete(m.snapshots, oldest)
	}
}

func (m *SecurityDiffManager) resolveSnapshot(name string) (*SecuritySnapshot, error) {
	snapshot, ok := m.snapshots[name]
	if !ok {
		return nil, fmt.Errorf("snapshot %q not found", name)
	}
	if m.isExpired(snapshot) {
		return nil, fmt.Errorf("snapshot %q has expired (TTL: %v)", name, m.ttl)
	}
	return snapshot, nil
}

func (m *SecurityDiffManager) resolveToSnapshot(toName string, currentBodies []capture.NetworkBody) (*SecuritySnapshot, error) {
	if toName == "" || toName == "current" {
		snapshot := newEmptySnapshot("current")
		populateSnapshotFromBodies(snapshot, currentBodies)
		return snapshot, nil
	}
	return m.resolveSnapshot(toName)
}
