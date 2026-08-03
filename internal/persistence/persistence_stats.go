// Purpose: Counts files and bytes per namespace for persistence store statistics.
// Why: Isolates storage accounting from CRUD operations and maintenance.
package persistence

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

func countNamespaceFiles(files sessionFilesystem, nsDir string) (count int, bytes int64, readErr error) {
	nsEntries, err := files.ReadDir(nsDir)
	if err != nil {
		return 0, 0, err
	}
	for _, nsEntry := range nsEntries {
		if nsEntry.IsDir() {
			continue
		}
		info, err := nsEntry.Info()
		if err != nil {
			return 0, 0, err
		}
		bytes += info.Size()
		count++
	}
	return count, bytes, nil
}

func isSafeDirName(name string) bool {
	return name != ".." && !filepath.IsAbs(name) && !strings.Contains(name, "..")
}

func (s *SessionStore) Stats() (StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := StoreStats{
		Namespaces:   make(map[string]int),
		SessionCount: s.meta.SessionCount,
	}

	entries, err := s.filesystem().ReadDir(s.projectDir)
	if err != nil {
		s.reportRecovery("session_store_stats_state", "Session storage statistics could not be read; no incomplete totals were returned.", "Check permissions for the project .kaboom directory, then retry.")
		return stats, errors.New("session_state_stats_failed")
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				s.reportRecovery("session_store_stats_state", "Session storage statistics could not be read; no incomplete totals were returned.", "Check permissions for the project .kaboom directory, then retry.")
				return StoreStats{Namespaces: make(map[string]int), SessionCount: s.meta.SessionCount}, errors.New("session_state_stats_failed")
			}
			stats.TotalBytes += info.Size()
			continue
		}

		name := entry.Name()
		if !isSafeDirName(name) {
			continue
		}

		nsDir := filepath.Join(s.projectDir, name)
		count, bytes, readErr := countNamespaceFiles(s.filesystem(), nsDir)
		if readErr != nil {
			s.reportRecovery("session_store_stats_state", "Session storage statistics could not be read; no incomplete totals were returned.", "Check permissions for the project .kaboom directory, then retry.")
			return StoreStats{Namespaces: make(map[string]int), SessionCount: s.meta.SessionCount}, errors.New("session_state_stats_failed")
		}
		stats.TotalBytes += bytes
		stats.Namespaces[name] = count
	}

	statediag.Resolve(s.diagnostics, "session_store_stats_state")
	return stats, nil
}
