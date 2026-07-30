// Purpose: Tests for persistence branch path coverage.
// Docs: docs/features/feature/persistent-memory/index.md

package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

func TestSessionStoreNewAndGetMetaCopy(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "projects", "test")

	store, err := newSessionStoreInDir(projectPath, projectDir, defaultFlushInterval, nil)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}
	t.Cleanup(store.Shutdown)

	meta := store.GetMeta()
	if meta.ProjectPath == "" {
		t.Fatal("GetMeta().ProjectPath should be populated")
	}
	if meta.SessionCount < 1 {
		t.Fatalf("GetMeta().SessionCount = %d, want >= 1", meta.SessionCount)
	}

	meta.SessionCount = -1
	fresh := store.GetMeta()
	if fresh.SessionCount < 1 {
		t.Fatalf("GetMeta() should return a copy, got SessionCount=%d", fresh.SessionCount)
	}
}

func TestSessionStoreRecoversMalformedMetadataAndContext(t *testing.T) {
	t.Parallel()

	projectPath := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "project-state")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "meta.json"), []byte(`{"secret":`), 0o600); err != nil {
		t.Fatal(err)
	}

	diagnostics := statediag.NewCollector()
	store, err := newSessionStoreInDir(projectPath, projectDir, defaultFlushInterval, diagnostics)
	if err != nil {
		t.Fatalf("malformed metadata must not block startup: %v", err)
	}
	t.Cleanup(store.Shutdown)
	if store.GetMeta().SessionCount != 1 {
		t.Fatalf("metadata fallback = %#v, want fresh session", store.GetMeta())
	}

	errorDir := filepath.Join(projectDir, "errors")
	if err := os.MkdirAll(errorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(errorDir, "history.json"), []byte(`{"token":"secret"`), 0o600); err != nil {
		t.Fatal(err)
	}
	context := store.LoadSessionContext()
	if len(context.ErrorHistory) != 0 {
		t.Fatalf("error history fallback = %#v, want empty", context.ErrorHistory)
	}

	got := diagnostics.Snapshot()
	if len(got) != 2 || got[0].Fix == "" || got[1].Fix == "" {
		t.Fatalf("recovery diagnostics = %#v, want metadata and error-history warnings", got)
	}
	for _, diagnostic := range got {
		if diagnostic.Detail == `{"token":"secret"` || diagnostic.Detail == `{"secret":` {
			t.Fatalf("diagnostic leaked raw state: %#v", diagnostic)
		}
	}
}

func TestSessionStoreReportsMalformedExplicitLoad(t *testing.T) {
	t.Parallel()

	diagnostics := statediag.NewCollector()
	store, err := newSessionStoreInDir(t.TempDir(), t.TempDir(), defaultFlushInterval, diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Shutdown)
	if err := store.Save("session", "broken", []byte(`{"token":"secret"`)); err != nil {
		t.Fatal(err)
	}
	result, err := store.HandleSessionStore(SessionStoreArgs{
		Action: "load", Namespace: "session", Key: "broken",
	})
	if err != nil {
		t.Fatalf("malformed explicit load should retain a safe response: %v", err)
	}
	if string(result) == "" {
		t.Fatal("malformed explicit load returned no response")
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "stored_session_state" || got[0].Fix == "" {
		t.Fatalf("diagnostics = %#v, want actionable stored-session warning", got)
	}
}

func TestSessionStoreHandleSessionStoreBranches(t *testing.T) {
	t.Parallel()

	store := newTestSessionStore(t)

	// Required-field validation paths.
	missingArgs := []SessionStoreArgs{
		{Action: "save", Key: "k", Data: json.RawMessage(`{"x":1}`)},
		{Action: "save", Namespace: "ns", Data: json.RawMessage(`{"x":1}`)},
		{Action: "save", Namespace: "ns", Key: "k"},
		{Action: "load", Key: "k"},
		{Action: "load", Namespace: "ns"},
		{Action: "list"},
		{Action: "delete", Key: "k"},
		{Action: "delete", Namespace: "ns"},
	}
	for _, args := range missingArgs {
		if _, err := store.HandleSessionStore(args); err == nil {
			t.Fatalf("HandleSessionStore(%q) with missing fields should fail", args.Action)
		}
	}

	if err := store.Save("ns", "a", []byte(`{"v":1}`)); err != nil {
		t.Fatalf("Save(ns/a) error = %v", err)
	}
	if err := store.Save("ns", "b", []byte(`{"v":2}`)); err != nil {
		t.Fatalf("Save(ns/b) error = %v", err)
	}

	listRaw, err := store.HandleSessionStore(SessionStoreArgs{
		Action:    "list",
		Namespace: "ns",
	})
	if err != nil {
		t.Fatalf("HandleSessionStore(list) error = %v", err)
	}
	var listResp struct {
		Namespace string   `json:"namespace"`
		Keys      []string `json:"keys"`
	}
	if err := json.Unmarshal(listRaw, &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if listResp.Namespace != "ns" || len(listResp.Keys) != 2 {
		t.Fatalf("list response unexpected: %+v", listResp)
	}
	if !slices.Contains(listResp.Keys, "a") || !slices.Contains(listResp.Keys, "b") {
		t.Fatalf("list response keys = %v, want [a b]", listResp.Keys)
	}

	if err := store.Save("raw", "blob", []byte("not-json")); err != nil {
		t.Fatalf("Save(raw/blob) error = %v", err)
	}
	loadRaw, err := store.HandleSessionStore(SessionStoreArgs{
		Action:    "load",
		Namespace: "raw",
		Key:       "blob",
	})
	if err != nil {
		t.Fatalf("HandleSessionStore(load raw/blob) error = %v", err)
	}
	var loadResp map[string]any
	if err := json.Unmarshal(loadRaw, &loadResp); err != nil {
		t.Fatalf("unmarshal load response error = %v", err)
	}
	if val, ok := loadResp["data"]; !ok || val != nil {
		t.Fatalf("load response data for invalid JSON should be null, got %v", loadResp["data"])
	}

	deleteRaw, err := store.HandleSessionStore(SessionStoreArgs{
		Action:    "delete",
		Namespace: "ns",
		Key:       "a",
	})
	if err != nil {
		t.Fatalf("HandleSessionStore(delete) error = %v", err)
	}
	var deleteResp map[string]any
	if err := json.Unmarshal(deleteRaw, &deleteResp); err != nil {
		t.Fatalf("unmarshal delete response error = %v", err)
	}
	if deleteResp["status"] != "deleted" {
		t.Fatalf("delete status = %v, want deleted", deleteResp["status"])
	}

	statsRaw, err := store.HandleSessionStore(SessionStoreArgs{Action: "stats"})
	if err != nil {
		t.Fatalf("HandleSessionStore(stats) error = %v", err)
	}
	var statsResp map[string]any
	if err := json.Unmarshal(statsRaw, &statsResp); err != nil {
		t.Fatalf("unmarshal stats response error = %v", err)
	}
	if _, ok := statsResp["total_bytes"]; !ok {
		t.Fatalf("stats response missing total_bytes: %v", statsResp)
	}
}

func TestSessionStoreSaveListDeleteErrorBranches(t *testing.T) {
	t.Parallel()

	store := newTestSessionStore(t)

	if err := store.Save("limits", "too-big", make([]byte, maxFileSize+1)); err == nil {
		t.Fatal("Save() should fail when payload exceeds max file size")
	}

	// Force project-size limit path.
	filler := filepath.Join(store.projectDir, "filler.bin")
	if err := os.WriteFile(filler, make([]byte, maxProjectSize), 0o600); err != nil {
		t.Fatalf("WriteFile(filler) error = %v", err)
	}
	if err := store.Save("limits", "after-filler", []byte(`{}`)); err == nil {
		t.Fatal("Save() should fail when project size limit is exceeded")
	}

	if _, err := store.List("does-not-exist"); err != nil {
		t.Fatalf("List(non-existent) error = %v", err)
	}

	nsDir := filepath.Join(store.projectDir, "mixed")
	if err := os.MkdirAll(filepath.Join(nsDir, "subdir"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(nsDir, "good.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile(good.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(nsDir, "notes.txt"), []byte("ignore"), 0o600); err != nil {
		t.Fatalf("WriteFile(notes.txt) error = %v", err)
	}

	keys, err := store.List("mixed")
	if err != nil {
		t.Fatalf("List(mixed) error = %v", err)
	}
	if len(keys) != 1 || keys[0] != "good" {
		t.Fatalf("List(mixed) = %v, want [good]", keys)
	}

	if err := store.Delete("mixed", "missing"); err == nil {
		t.Fatal("Delete() should fail for missing key")
	}

	if _, err := store.Load("../unsafe", "k"); err == nil {
		t.Fatal("Load() should reject unsafe namespace")
	}
	if err := store.Delete("safe", "../unsafe"); err == nil {
		t.Fatal("Delete() should reject unsafe key")
	}
}
