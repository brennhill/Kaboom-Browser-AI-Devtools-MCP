// Purpose: Loads JSON files from persistence directories into generic map structures.
// Why: Isolates raw file I/O from CRUD operations and store lifecycle.
package persistence

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

func (s *SessionStore) loadJSONFileAs(path, diagnosticName string) map[string]any {
	data, err := s.filesystem().ReadFile(path) // #nosec G304 -- callers construct path from internal projectDir field // nosemgrep: go_filesystem_rule-fileread -- local persistence store I/O
	if err != nil {
		if os.IsNotExist(err) {
			// EXPECTED_ABSENCE: optional project context has not been authored yet;
			// resolving clears any earlier transient read incident.
			statediag.Resolve(s.diagnostics, diagnosticName)
			return nil
		}
		s.reportRecovery(diagnosticName, "Saved project context could not be read; that context is unavailable.", "Check permissions for the project .kaboom directory.")
		return nil
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil {
		s.reportRecovery(diagnosticName, "Saved project context was malformed; that context was ignored.", "Recreate the affected project context with the corresponding configure action.")
		return nil
	}
	statediag.Resolve(s.diagnostics, diagnosticName)
	return result
}

func parseRawErrorEntry(raw map[string]any) ErrorHistoryEntry {
	entry := ErrorHistoryEntry{}
	if fp, ok := raw["fingerprint"].(string); ok {
		entry.Fingerprint = fp
	}
	if c, ok := raw["count"].(float64); ok {
		entry.Count = int(c)
	}
	if r, ok := raw["resolved"].(bool); ok {
		entry.Resolved = r
	}
	return entry
}

func (s *SessionStore) loadErrorHistory(path string) []ErrorHistoryEntry {
	data, err := s.filesystem().ReadFile(path) // #nosec G304 -- callers construct path from internal projectDir field
	if err != nil {
		if os.IsNotExist(err) {
			// EXPECTED_ABSENCE: error history is optional until the first recorded
			// error; resolving clears any earlier transient read incident.
			statediag.Resolve(s.diagnostics, "error_history_state")
			return nil
		}
		s.reportRecovery("error_history_state", "Saved error history could not be read; an empty history is active.", "Check permissions for the project .kaboom directory.")
		return nil
	}

	var entries []ErrorHistoryEntry
	if json.Unmarshal(data, &entries) == nil {
		statediag.Resolve(s.diagnostics, "error_history_state")
		return entries
	}

	var rawEntries []map[string]any
	if json.Unmarshal(data, &rawEntries) != nil {
		s.reportRecovery("error_history_state", "Saved error history was malformed; an empty history is active.", "Clear or recreate the saved error history.")
		return nil
	}

	result := make([]ErrorHistoryEntry, 0, len(rawEntries))
	for _, raw := range rawEntries {
		result = append(result, parseRawErrorEntry(raw))
	}
	statediag.Resolve(s.diagnostics, "error_history_state")
	return result
}

func (s *SessionStore) LoadSessionContext() SessionContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ctx := SessionContext{
		ProjectID:    s.projectPath,
		SessionCount: s.meta.SessionCount,
		Baselines:    []string{},
		ErrorHistory: []ErrorHistoryEntry{},
	}

	baselineDir := filepath.Join(s.projectDir, "baselines")
	if keys, err := jsonKeysFromDir(s.filesystem(), baselineDir); err == nil && len(keys) > 0 {
		ctx.Baselines = keys
	} else if err != nil {
		s.reportRecovery("baseline_context_state", "Saved baselines could not be listed; an empty baseline list is active.", "Check permissions for the project .kaboom directory.")
	} else {
		statediag.Resolve(s.diagnostics, "baseline_context_state")
	}

	ctx.NoiseConfig = s.loadJSONFileAs(filepath.Join(s.projectDir, "noise", "config.json"), "noise_context_state")
	if entries := s.loadErrorHistory(filepath.Join(s.projectDir, "errors", "history.json")); entries != nil {
		ctx.ErrorHistory = entries
	}
	ctx.APISchema = s.loadJSONFileAs(filepath.Join(s.projectDir, "api_schema", "schema.json"), "api_schema_state")
	ctx.Performance = s.loadJSONFileAs(filepath.Join(s.projectDir, "performance", "endpoints.json"), "performance_context_state")

	return ctx
}

func (s *SessionStore) projectSize() (int64, error) {
	var total int64
	err := s.filesystem().Walk(s.projectDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func countNamespaceFiles(files sessionFilesystem, nsDir string) (count int, bytes int64, readErr error) {
	entries, err := files.ReadDir(nsDir)
	if err != nil {
		return 0, 0, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
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

// Stats returns a complete filesystem-backed storage snapshot or a diagnosed error.
func (s *SessionStore) Stats() (StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := StoreStats{Namespaces: make(map[string]int), SessionCount: s.meta.SessionCount}
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
		if !isSafeDirName(entry.Name()) {
			continue
		}
		count, bytes, readErr := countNamespaceFiles(s.filesystem(), filepath.Join(s.projectDir, entry.Name()))
		if readErr != nil {
			s.reportRecovery("session_store_stats_state", "Session storage statistics could not be read; no incomplete totals were returned.", "Check permissions for the project .kaboom directory, then retry.")
			return StoreStats{Namespaces: make(map[string]int), SessionCount: s.meta.SessionCount}, errors.New("session_state_stats_failed")
		}
		stats.TotalBytes += bytes
		stats.Namespaces[entry.Name()] = count
	}
	statediag.Resolve(s.diagnostics, "session_store_stats_state")
	return stats, nil
}
