// Purpose: Tests for JSON persistence serialization and deserialization.
// Docs: docs/features/feature/persistent-memory/index.md

// ai_persistence_json_test.go — Tests for persistence JSON serialization, file utilities, and namespace helpers.
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
// SessionStore JSON fields: snake_case
// ============================================

func TestSessionStore_JSONSnakeCase(t *testing.T) {
	t.Parallel()

	// ProjectMeta
	meta := ProjectMeta{
		ProjectID:    "test",
		ProjectPath:  "/path",
		FirstCreated: time.Now(),
		LastSession:  time.Now(),
		SessionCount: 5,
	}
	data, _ := json.Marshal(meta)
	jsonStr := string(data)

	expectedFields := []string{`"project_id"`, `"project_path"`, `"first_created"`, `"last_session"`, `"session_count"`}
	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("ProjectMeta JSON missing %s: %s", field, jsonStr)
		}
	}

	// SessionContext
	ctx := SessionContext{
		ProjectID:    "test",
		SessionCount: 1,
		Baselines:    []string{},
		NoiseConfig:  map[string]any{"k": "v"},
		ErrorHistory: []ErrorHistoryEntry{},
		APISchema:    map[string]any{"paths": "/users"},
		Performance:  map[string]any{"p95": 100},
	}
	data, _ = json.Marshal(ctx)
	jsonStr = string(data)

	ctxFields := []string{`"project_id"`, `"session_count"`, `"baselines"`, `"noise_config"`, `"error_history"`, `"api_schema"`, `"performance"`}
	for _, field := range ctxFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("SessionContext JSON missing %s: %s", field, jsonStr)
		}
	}

	// ErrorHistoryEntry
	entry := ErrorHistoryEntry{
		Fingerprint: "fp",
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Count:       3,
		Resolved:    true,
		ResolvedAt:  time.Now(),
	}
	data, _ = json.Marshal(entry)
	jsonStr = string(data)

	entryFields := []string{`"fingerprint"`, `"first_seen"`, `"last_seen"`, `"count"`, `"resolved"`, `"resolved_at"`}
	for _, field := range entryFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("ErrorHistoryEntry JSON missing %s: %s", field, jsonStr)
		}
	}

	// StoreStats
	stats := StoreStats{
		TotalBytes:   1024,
		SessionCount: 3,
		Namespaces:   map[string]int{"ns": 2},
	}
	data, _ = json.Marshal(stats)
	jsonStr = string(data)

	statsFields := []string{`"total_bytes"`, `"session_count"`, `"namespaces"`}
	for _, field := range statsFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("StoreStats JSON missing %s: %s", field, jsonStr)
		}
	}
}

// ============================================
// SessionStore: countNamespaceFiles
// ============================================

func TestCountNamespaceFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Empty dir
	files := localSessionFilesystem{}
	count, bytes, err := countNamespaceFiles(files, dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 || bytes != 0 {
		t.Errorf("empty dir: count=%d bytes=%d, want 0/0", count, bytes)
	}

	// Non-existent dir
	count, bytes, err = countNamespaceFiles(files, filepath.Join(dir, "nonexistent"))
	if err == nil || count != 0 || bytes != 0 {
		t.Errorf("non-existent dir: count=%d bytes=%d err=%v, want 0/0/error", count, bytes, err)
	}

	// Dir with files
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.json"), []byte(`{"b":2}`), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	// Subdirectory should be skipped
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	count, bytes, err = countNamespaceFiles(files, dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if bytes == 0 {
		t.Error("bytes should be > 0")
	}
}

// ============================================
// jsonKeysFromDir: various states
// ============================================

func TestJsonKeysFromDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Non-existent directory
	files := localSessionFilesystem{}
	keys, err := jsonKeysFromDir(files, filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatalf("non-existent dir should not error, got %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("non-existent dir keys = %v, want empty", keys)
	}

	// Empty directory
	emptyDir := filepath.Join(dir, "empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	keys, err = jsonKeysFromDir(files, emptyDir)
	if err != nil {
		t.Fatalf("empty dir error = %v", err)
	}
	if keys == nil {
		t.Fatal("empty dir should return non-nil empty slice")
	}
	if len(keys) != 0 {
		t.Errorf("empty dir keys = %v, want empty", keys)
	}

	// Directory with mixed files
	mixedDir := filepath.Join(dir, "mixed")
	if err := os.MkdirAll(mixedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	os.WriteFile(filepath.Join(mixedDir, "config.json"), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(mixedDir, "notes.txt"), []byte("text"), 0o644)
	os.MkdirAll(filepath.Join(mixedDir, "subdir"), 0o755)

	keys, err = jsonKeysFromDir(files, mixedDir)
	if err != nil {
		t.Fatalf("mixed dir error = %v", err)
	}
	if len(keys) != 1 || keys[0] != "config" {
		t.Errorf("mixed dir keys = %v, want [config]", keys)
	}
}
