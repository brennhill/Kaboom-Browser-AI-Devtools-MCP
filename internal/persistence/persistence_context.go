// Purpose: Loads JSON files from persistence directories into generic map structures.
// Why: Isolates raw file I/O from CRUD operations and store lifecycle.
package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func (s *SessionStore) loadJSONFileAs(path, diagnosticName string) map[string]any {
	data, err := os.ReadFile(path) // #nosec G304 -- callers construct path from internal projectDir field // nosemgrep: go_filesystem_rule-fileread -- local persistence store I/O
	if err != nil {
		if !os.IsNotExist(err) {
			s.reportRecovery(diagnosticName, "Saved project context could not be read; that context is unavailable.", "Check permissions for the project .kaboom directory.")
		}
		return nil
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil {
		s.reportRecovery(diagnosticName, "Saved project context was malformed; that context was ignored.", "Recreate the affected project context with the corresponding configure action.")
		return nil
	}
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
	data, err := os.ReadFile(path) // #nosec G304 -- callers construct path from internal projectDir field
	if err != nil {
		if !os.IsNotExist(err) {
			s.reportRecovery("error_history_state", "Saved error history could not be read; an empty history is active.", "Check permissions for the project .kaboom directory.")
		}
		return nil
	}

	var entries []ErrorHistoryEntry
	if json.Unmarshal(data, &entries) == nil {
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
	if keys, err := jsonKeysFromDir(baselineDir); err == nil && len(keys) > 0 {
		ctx.Baselines = keys
	}

	ctx.NoiseConfig = s.loadJSONFileAs(filepath.Join(s.projectDir, "noise", "config.json"), "noise_context_state")
	if entries := s.loadErrorHistory(filepath.Join(s.projectDir, "errors", "history.json")); entries != nil {
		ctx.ErrorHistory = entries
	}
	ctx.APISchema = s.loadJSONFileAs(filepath.Join(s.projectDir, "api_schema", "schema.json"), "api_schema_state")
	ctx.Performance = s.loadJSONFileAs(filepath.Join(s.projectDir, "performance", "endpoints.json"), "performance_context_state")

	return ctx
}
