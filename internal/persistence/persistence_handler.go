// Purpose: Implements persistence command handlers for save/load/list AI context operations.
// Why: Exposes persistent-memory workflows through stable tool-facing handler entrypoints.
// Docs: docs/features/feature/persistent-memory/index.md

package persistence

import (
	"encoding/json"
	"fmt"
)

// requireFields validates that required string fields are non-empty for a
// given action. Returns an error naming the first missing field.
func requireFields(action string, fields map[string]string) error {
	for name, value := range fields {
		if value == "" {
			return fmt.Errorf("%s is required for %s action", name, action)
		}
	}
	return nil
}

func (s *SessionStore) handleSave(args SessionStoreArgs) (json.RawMessage, error) {
	if err := requireFields("save", map[string]string{
		"namespace": args.Namespace,
		"key":       args.Key,
	}); err != nil {
		return nil, err
	}
	if len(args.Data) == 0 {
		return nil, fmt.Errorf("data is required for save action")
	}
	if err := s.Save(args.Namespace, args.Key, []byte(args.Data)); err != nil {
		return nil, err
	}
	// Error impossible: map contains only string values
	result, _ := json.Marshal(map[string]any{
		"status":    "saved",
		"namespace": args.Namespace,
		"key":       args.Key,
	})
	return result, nil
}

func (s *SessionStore) handleLoad(args SessionStoreArgs) (json.RawMessage, error) {
	if err := requireFields("load", map[string]string{
		"namespace": args.Namespace,
		"key":       args.Key,
	}); err != nil {
		return nil, err
	}
	data, err := s.Load(args.Namespace, args.Key)
	if err != nil {
		return nil, err
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		s.reportRecovery(
			"stored_session_state",
			"Saved session data was malformed and was returned as an empty value.",
			"Overwrite or delete the affected configure store entry.",
		)
	}
	// Error impossible: map contains only primitive types and pre-parsed JSON data
	result, _ := json.Marshal(map[string]any{
		"namespace": args.Namespace,
		"key":       args.Key,
		"data":      parsed,
	})
	return result, nil
}

func (s *SessionStore) handleList(args SessionStoreArgs) (json.RawMessage, error) {
	if args.Namespace == "" {
		return nil, fmt.Errorf("namespace is required for list action")
	}
	keys, err := s.List(args.Namespace)
	if err != nil {
		return nil, err
	}
	// Error impossible: map contains only string values and string slices
	result, _ := json.Marshal(map[string]any{
		"namespace": args.Namespace,
		"keys":      keys,
	})
	return result, nil
}

func (s *SessionStore) handleDelete(args SessionStoreArgs) (json.RawMessage, error) {
	if err := requireFields("delete", map[string]string{
		"namespace": args.Namespace,
		"key":       args.Key,
	}); err != nil {
		return nil, err
	}
	if err := s.Delete(args.Namespace, args.Key); err != nil {
		return nil, err
	}
	// Error impossible: map contains only string values
	result, _ := json.Marshal(map[string]any{
		"status":    "deleted",
		"namespace": args.Namespace,
		"key":       args.Key,
	})
	return result, nil
}

func (s *SessionStore) handleStats() (json.RawMessage, error) {
	stats, err := s.Stats()
	if err != nil {
		return nil, err
	}
	// Error impossible: map contains only primitive types and string slices
	result, _ := json.Marshal(map[string]any{
		"total_bytes":   stats.TotalBytes,
		"session_count": stats.SessionCount,
		"namespaces":    stats.Namespaces,
	})
	return result, nil
}

// HandleSessionStore handles the session_store MCP tool actions.
func (s *SessionStore) HandleSessionStore(args SessionStoreArgs) (json.RawMessage, error) {
	switch args.Action {
	case "save":
		return s.handleSave(args)
	case "load":
		return s.handleLoad(args)
	case "list":
		return s.handleList(args)
	case "delete":
		return s.handleDelete(args)
	case "stats":
		return s.handleStats()
	default:
		return nil, fmt.Errorf("unknown action: %s (valid: save, load, list, delete, stats)", args.Action)
	}
}
