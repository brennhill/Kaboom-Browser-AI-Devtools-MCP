// Purpose: Edge-case tests for persistence boundary conditions.
// Docs: docs/features/feature/persistent-memory/index.md

// ai_persistence_edge_test.go — Edge case tests for persistence, validation, and error paths.
package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================
// validateStoreInput: comprehensive
// ============================================

func TestValidateStoreInput_Comprehensive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		label   string
		wantErr bool
		errMsg  string
	}{
		{"empty_value", "", "field", false, ""},
		{"normal_value", "noise", "namespace", false, ""},
		{"path_traversal", "../escape", "namespace", true, "path traversal"},
		{"embedded_traversal", "foo/../bar", "key", true, "path traversal"},
		{"forward_slash", "foo/bar", "namespace", true, "path separator"},
		{"valid_with_dot", "config.v2", "key", false, ""},
		{"valid_with_dash", "my-namespace", "namespace", false, ""},
		{"valid_with_underscore", "my_key", "key", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStoreInput(tt.value, tt.label)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.value)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for %q: %v", tt.value, err)
				}
			}
		})
	}
}

// ============================================
// validatePathInDir: comprehensive
// ============================================

func TestValidatePathInDir_Comprehensive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		base    string
		target  string
		wantErr bool
	}{
		{"valid_within", "/base/dir", "/base/dir/sub/file.json", false},
		{"exact_base", "/base/dir", "/base/dir/file.json", false},
		{"escapes_dir", "/base/dir", "/base/other/file.json", true},
		{"root_escape", "/base/dir", "/etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathInDir(tt.base, tt.target)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for base=%q target=%q", tt.base, tt.target)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for base=%q target=%q: %v", tt.base, tt.target, err)
			}
		})
	}
}

// ============================================
// isSafeDirName: comprehensive
// ============================================

func TestIsSafeDirName_Comprehensive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		safe bool
	}{
		{"noise", true},
		{"baselines", true},
		{"my-dir", true},
		{"..", false},
		{"../escape", false},
		{"foo/../bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeDirName(tt.name); got != tt.safe {
				t.Errorf("isSafeDirName(%q) = %v, want %v", tt.name, got, tt.safe)
			}
		})
	}
}

// ============================================
// requireFields: comprehensive
// ============================================

func TestRequireFields_Comprehensive(t *testing.T) {
	t.Parallel()

	// All present
	err := requireFields("save", map[string]string{
		"namespace": "ns",
		"key":       "k",
	})
	if err != nil {
		t.Errorf("all fields present should not error, got %v", err)
	}

	// Missing one field
	err = requireFields("save", map[string]string{
		"namespace": "ns",
		"key":       "",
	})
	if err == nil {
		t.Fatal("missing field should error")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Errorf("error should mention missing field, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "save") {
		t.Errorf("error should mention action, got %q", err.Error())
	}
}

// ============================================
// SessionStore: double shutdown is safe
// ============================================

func TestSessionStore_DoubleShutdown(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	store.Shutdown()
	// Second shutdown should not panic
	store.Shutdown()
}

// ============================================
// SessionStore: GetMeta with nil meta
// ============================================

func TestSessionStore_GetMetaNil(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "projects", "nil-meta-test")

	store, err := newSessionStoreInDir(projectPath, projectDir, time.Hour, nil)
	if err != nil {
		t.Fatalf("newSessionStoreInDir error = %v", err)
	}
	// Shutdown first to stop background goroutine cleanly before forcing nil meta
	store.Shutdown()

	// Force nil meta (shouldn't happen in practice)
	store.mu.Lock()
	store.meta = nil
	store.mu.Unlock()

	meta := store.GetMeta()
	if meta.ProjectPath != "" {
		t.Errorf("nil meta should return empty ProjectMeta, got %+v", meta)
	}
}

// ============================================
// SessionStore: Load non-existent namespace
// ============================================

func TestSessionStore_LoadNonExistent(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	_, err := store.Load("nonexistent", "key")
	if err == nil {
		t.Fatal("Load for non-existent key should return error")
	}
	if !strings.Contains(err.Error(), "key not found") {
		t.Errorf("error = %q, want to contain 'key not found'", err.Error())
	}
}

// ============================================
// SessionStore: List on empty namespace returns empty slice
// ============================================

func TestSessionStore_ListEmptyNamespace(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	keys, err := store.List("empty-ns")
	if err != nil {
		t.Fatalf("List(empty-ns) error = %v", err)
	}
	if keys == nil {
		t.Fatal("List should return non-nil slice even for empty namespace")
	}
	if len(keys) != 0 {
		t.Errorf("List(empty-ns) = %v, want empty", keys)
	}
}

// ============================================
// SessionStore: Delete non-existent key
// ============================================

func TestSessionStore_DeleteNonExistent(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	err := store.Delete("ns", "nonexistent")
	if err == nil {
		t.Fatal("Delete for non-existent key should return error")
	}
}

// ============================================
// SessionStore: save and load cycle preserves data
// ============================================

func TestSessionStore_SaveLoadCycle(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	data := map[string]any{
		"name":    "test",
		"count":   42,
		"enabled": true,
		"tags":    []string{"a", "b"},
	}
	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	if err := store.Save("test-ns", "test-key", payload); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	loaded, err := store.Load("test-ns", "test-key")
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(loaded, &result); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("name = %v, want 'test'", result["name"])
	}
	if result["count"] != float64(42) {
		t.Errorf("count = %v, want 42", result["count"])
	}
	if result["enabled"] != true {
		t.Errorf("enabled = %v, want true", result["enabled"])
	}
}

// ============================================
// loadJSONFileAs: various inputs
// ============================================

func TestLoadJSONFileAs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := &SessionStore{}

	// Valid JSON file
	validPath := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(validPath, []byte(`{"key":"value"}`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	result := store.loadJSONFileAs(validPath, "test_state")
	if result == nil {
		t.Fatal("valid JSON should return non-nil map")
	}
	if result["key"] != "value" {
		t.Errorf("key = %v, want 'value'", result["key"])
	}

	// Non-existent file
	result = store.loadJSONFileAs(filepath.Join(dir, "missing.json"), "test_state")
	if result != nil {
		t.Error("missing file should return nil")
	}

	// Invalid JSON
	invalidPath := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	result = store.loadJSONFileAs(invalidPath, "test_state")
	if result != nil {
		t.Error("invalid JSON should return nil")
	}

	// JSON array (not object)
	arrayPath := filepath.Join(dir, "array.json")
	if err := os.WriteFile(arrayPath, []byte(`[1,2,3]`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	result = store.loadJSONFileAs(arrayPath, "test_state")
	if result != nil {
		t.Error("JSON array should return nil (expects object)")
	}
}

// ============================================
// parseRawErrorEntry: comprehensive
// ============================================

func TestParseRawErrorEntry_Comprehensive(t *testing.T) {
	t.Parallel()

	// Full entry
	raw := map[string]any{
		"fingerprint": "err-123",
		"count":       float64(5),
		"resolved":    true,
	}
	entry := parseRawErrorEntry(raw)
	if entry.Fingerprint != "err-123" {
		t.Errorf("Fingerprint = %q, want 'err-123'", entry.Fingerprint)
	}
	if entry.Count != 5 {
		t.Errorf("Count = %d, want 5", entry.Count)
	}
	if !entry.Resolved {
		t.Error("Resolved should be true")
	}

	// Missing fields
	empty := parseRawErrorEntry(map[string]any{})
	if empty.Fingerprint != "" || empty.Count != 0 || empty.Resolved {
		t.Errorf("empty map should return zero-value entry, got %+v", empty)
	}

	// Wrong types
	wrongTypes := parseRawErrorEntry(map[string]any{
		"fingerprint": 123,
		"count":       "five",
		"resolved":    "yes",
	})
	if wrongTypes.Fingerprint != "" || wrongTypes.Count != 0 || wrongTypes.Resolved {
		t.Errorf("wrong types should be handled gracefully, got %+v", wrongTypes)
	}
}

// ============================================
// loadErrorHistory: various file contents
// ============================================

func TestLoadErrorHistory_Various(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := &SessionStore{}

	// Non-existent file
	result := store.loadErrorHistory(filepath.Join(dir, "missing.json"))
	if result != nil {
		t.Error("missing file should return nil")
	}

	// Valid typed entries
	validEntries := []ErrorHistoryEntry{
		{Fingerprint: "err-1", Count: 3, Resolved: false, FirstSeen: time.Now(), LastSeen: time.Now()},
	}
	data, _ := json.Marshal(validEntries)
	validPath := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(validPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	result = store.loadErrorHistory(validPath)
	if len(result) != 1 || result[0].Fingerprint != "err-1" || result[0].Count != 3 {
		t.Errorf("valid typed entries: got %+v", result)
	}

	// Invalid JSON
	invalidPath := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("{bad"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	result = store.loadErrorHistory(invalidPath)
	if result != nil {
		t.Error("invalid JSON should return nil")
	}

	// Generic map entries (fallback path)
	genericPath := filepath.Join(dir, "generic.json")
	genericData := `[{"fingerprint":"gen-1","count":7,"resolved":true,"first_seen":"invalid-time"}]`
	if err := os.WriteFile(genericPath, []byte(genericData), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	result = store.loadErrorHistory(genericPath)
	if len(result) != 1 {
		t.Fatalf("generic fallback should return 1 entry, got %d", len(result))
	}
	if result[0].Fingerprint != "gen-1" || result[0].Count != 7 || !result[0].Resolved {
		t.Errorf("generic fallback entry = %+v", result[0])
	}
}

// ============================================
// SessionStore: HandleSessionStore list action
// ============================================

func TestHandleSessionStore_ListAction(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	// Save some data
	if err := store.Save("myns", "key1", []byte(`{"a":1}`)); err != nil {
		t.Fatalf("Save error = %v", err)
	}
	if err := store.Save("myns", "key2", []byte(`{"b":2}`)); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	result, err := store.HandleSessionStore(SessionStoreArgs{
		Action:    "list",
		Namespace: "myns",
	})
	if err != nil {
		t.Fatalf("HandleSessionStore(list) error = %v", err)
	}

	var resp struct {
		Namespace string   `json:"namespace"`
		Keys      []string `json:"keys"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if resp.Namespace != "myns" {
		t.Errorf("namespace = %q, want 'myns'", resp.Namespace)
	}
	if len(resp.Keys) != 2 {
		t.Errorf("keys len = %d, want 2", len(resp.Keys))
	}
}

// ============================================
// SessionStore: HandleSessionStore stats action
// ============================================

func TestHandleSessionStore_StatsAction(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	if err := store.Save("ns1", "k1", []byte(`{"x":1}`)); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	result, err := store.HandleSessionStore(SessionStoreArgs{Action: "stats"})
	if err != nil {
		t.Fatalf("HandleSessionStore(stats) error = %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if _, ok := resp["total_bytes"]; !ok {
		t.Error("stats response missing total_bytes")
	}
	if _, ok := resp["session_count"]; !ok {
		t.Error("stats response missing session_count")
	}
	if _, ok := resp["namespaces"]; !ok {
		t.Error("stats response missing namespaces")
	}
}

// ============================================
// SessionStore: MarkDirty and flush
// ============================================

func TestSessionStore_MarkDirtyAndFlush(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	store.MarkDirty("dirty-ns", "dirty-key", []byte(`{"dirty":true}`))

	// Verify it's in dirty buffer
	store.dirtyMu.Lock()
	if len(store.dirty) != 1 {
		store.dirtyMu.Unlock()
		t.Fatalf("dirty buffer len = %d, want 1", len(store.dirty))
	}
	store.dirtyMu.Unlock()

	// Flush
	store.flushDirty()

	// Should be written to disk
	loaded, err := store.Load("dirty-ns", "dirty-key")
	if err != nil {
		t.Fatalf("Load after flush error = %v", err)
	}
	if string(loaded) != `{"dirty":true}` {
		t.Errorf("loaded = %q, want '{\"dirty\":true}'", string(loaded))
	}

	// Dirty buffer should be empty
	store.dirtyMu.Lock()
	if len(store.dirty) != 0 {
		store.dirtyMu.Unlock()
		t.Fatal("dirty buffer should be empty after flush")
	}
	store.dirtyMu.Unlock()
}

// ============================================
// SessionStore: flushDirty with empty buffer is no-op
// ============================================

func TestSessionStore_FlushDirtyEmpty(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	// Should not panic or error
	store.flushDirty()
}

// ============================================
// SessionStore: LoadSessionContext empty store
// ============================================

func TestSessionStore_LoadSessionContextEmpty(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	ctx := store.LoadSessionContext()

	if ctx.SessionCount < 1 {
		t.Errorf("SessionCount = %d, want >= 1", ctx.SessionCount)
	}
	if ctx.Baselines == nil || len(ctx.Baselines) != 0 {
		t.Errorf("Baselines = %v, want empty non-nil", ctx.Baselines)
	}
	if ctx.ErrorHistory == nil || len(ctx.ErrorHistory) != 0 {
		t.Errorf("ErrorHistory = %v, want empty non-nil", ctx.ErrorHistory)
	}
}

// ============================================
// SessionStore: loadOrCreateMeta with existing meta
// ============================================

func TestSessionStore_LoadOrCreateMetaExisting(t *testing.T) {
	t.Parallel()

	// Create store (first session)
	projectPath := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "projects", "existing-meta-test")
	store1, err := newSessionStoreInDir(projectPath, projectDir, time.Hour, nil)
	if err != nil {
		t.Fatalf("first store error = %v", err)
	}
	store1.Shutdown()

	// Create second store (second session - should increment session count)
	store2, err := newSessionStoreInDir(projectPath, projectDir, time.Hour, nil)
	if err != nil {
		t.Fatalf("second store error = %v", err)
	}
	defer store2.Shutdown()

	meta := store2.GetMeta()
	if meta.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2 (second session)", meta.SessionCount)
	}
}
