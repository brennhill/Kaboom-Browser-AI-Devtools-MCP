// Purpose: Encapsulates performance snapshot map operations behind focused store methods.
// Why: Reduces Capture method complexity during god-object decomposition while preserving behavior.
// Docs: docs/features/feature/observe/index.md

package capture

const (
	maxPerformanceSnapshots = 100
	maxBeforeSnapshots      = 50
)

// appendSnapshots stores snapshots by URL with oldest-entry eviction.
func (s *PerformanceStore) appendSnapshots(snapshots []PerformanceSnapshot) {
	for _, snapshot := range snapshots {
		key := snapshot.URL
		if key == "" {
			continue
		}

		if _, exists := s.snapshots[key]; !exists {
			s.snapshotOrder = append(s.snapshotOrder, key)
		}
		s.snapshots[key] = snapshot

		for len(s.snapshots) > maxPerformanceSnapshots && len(s.snapshotOrder) > 0 {
			oldestKey := s.snapshotOrder[0]
			s.snapshotOrder = s.snapshotOrder[1:]
			delete(s.snapshots, oldestKey)
		}
	}
}

// snapshotsList returns a detached list copy.
func (s *PerformanceStore) snapshotsList() []PerformanceSnapshot {
	if len(s.snapshots) == 0 {
		return []PerformanceSnapshot{}
	}
	out := make([]PerformanceSnapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		out = append(out, snapshot)
	}
	return out
}

// snapshotByURL returns one snapshot by URL key.
func (s *PerformanceStore) snapshotByURL(url string) (PerformanceSnapshot, bool) {
	snap, ok := s.snapshots[url]
	return snap, ok
}

// storeBeforeSnapshot keeps a pre-action snapshot for perf diff correlation.
func (s *PerformanceStore) storeBeforeSnapshot(correlationID string, snapshot PerformanceSnapshot) {
	s.beforeSnapshots[correlationID] = snapshot
	if len(s.beforeSnapshots) <= maxBeforeSnapshots {
		return
	}

	// Preserve current semantics: remove an arbitrary key when over cap.
	for key := range s.beforeSnapshots {
		delete(s.beforeSnapshots, key)
		break
	}
}

// takeBeforeSnapshot retrieves and deletes a before-snapshot (consume-on-read).
func (s *PerformanceStore) takeBeforeSnapshot(correlationID string) (PerformanceSnapshot, bool) {
	snap, ok := s.beforeSnapshots[correlationID]
	if ok {
		delete(s.beforeSnapshots, correlationID)
	}
	return snap, ok
}

// clear resets performance snapshot/baseline/before-snapshot state.
func (s *PerformanceStore) clear() {
	s.snapshots = make(map[string]PerformanceSnapshot)
	s.snapshotOrder = make([]string, 0)
	s.baselines = make(map[string]PerformanceBaseline)
	s.baselineOrder = make([]string, 0)
	s.beforeSnapshots = make(map[string]PerformanceSnapshot)
}
