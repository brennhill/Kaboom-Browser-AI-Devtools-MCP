// Purpose: Tests for data persistence read, write, and file management.
// Docs: docs/features/feature/persistent-memory/index.md

package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPackageFileBoundary(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read persistence package: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			count++
		}
	}
	if count > 10 {
		t.Fatalf("persistence package has %d Go files; maximum is 10", count)
	}
}

func newTestSessionStore(t *testing.T) *SessionStore {
	t.Helper()

	projectPath := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "projects", "test")

	store, err := newSessionStoreInDir(projectPath, projectDir, time.Hour, nil)
	if err != nil {
		t.Fatalf("newSessionStoreInDir() error = %v", err)
	}
	t.Cleanup(store.Shutdown)
	return store
}

func TestSessionStore_CRUDAndStats(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	payload := []byte(`{"enabled":true}`)
	if err := store.Save("noise", "config", payload); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load("noise", "config")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("Load() mismatch: got %s want %s", string(got), string(payload))
	}

	keys, err := store.List("noise")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(keys) != 1 || keys[0] != "config" {
		t.Fatalf("List() = %v, want [config]", keys)
	}

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.Namespaces["noise"] != 1 {
		t.Fatalf("Stats().Namespaces[noise] = %d, want 1", stats.Namespaces["noise"])
	}
	if stats.TotalBytes == 0 {
		t.Fatal("Stats().TotalBytes should be > 0")
	}

	if err := store.Delete("noise", "config"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Load("noise", "config"); err == nil {
		t.Fatal("Load() after Delete() should fail")
	}
}

func TestSessionStore_SaveRejectsUnsafeNamespaceOrKey(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	cases := []struct {
		namespace string
		key       string
	}{
		{namespace: "../noise", key: "ok"},
		{namespace: "noise/inner", key: "ok"},
		{namespace: "noise", key: "../config"},
		{namespace: "noise", key: "dir/config"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%s", tc.namespace, tc.key), func(t *testing.T) {
			t.Parallel()
			if err := store.Save(tc.namespace, tc.key, []byte(`{}`)); err == nil {
				t.Fatalf("Save(%q, %q) should fail for unsafe path input", tc.namespace, tc.key)
			}
		})
	}
}

func TestSessionStore_LoadSessionContextParsesStoredData(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	if err := store.Save("baselines", "home", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("save baseline error: %v", err)
	}
	if err := store.Save("noise", "config", []byte(`{"mode":"strict"}`)); err != nil {
		t.Fatalf("save noise error: %v", err)
	}
	if err := store.Save("api_schema", "schema", []byte(`{"paths":{"\/users":{}}}`)); err != nil {
		t.Fatalf("save api schema error: %v", err)
	}
	if err := store.Save("performance", "endpoints", []byte(`{"p95_ms":123}`)); err != nil {
		t.Fatalf("save performance error: %v", err)
	}

	// Force fallback parse path: invalid time string fails []ErrorHistoryEntry unmarshal,
	// but generic map parsing still extracts fingerprint/count/resolved.
	rawHistory := []byte(`[{"fingerprint":"err-1","count":2,"resolved":true,"first_seen":"not-a-time"}]`)
	if err := store.Save("errors", "history", rawHistory); err != nil {
		t.Fatalf("save history error: %v", err)
	}

	ctx := store.LoadSessionContext()

	if len(ctx.Baselines) != 1 || ctx.Baselines[0] != "home" {
		t.Fatalf("ctx.Baselines = %v, want [home]", ctx.Baselines)
	}
	if mode, _ := ctx.NoiseConfig["mode"].(string); mode != "strict" {
		t.Fatalf("ctx.NoiseConfig[mode] = %v, want strict", ctx.NoiseConfig["mode"])
	}
	if len(ctx.ErrorHistory) != 1 {
		t.Fatalf("ctx.ErrorHistory len = %d, want 1", len(ctx.ErrorHistory))
	}
	if ctx.ErrorHistory[0].Fingerprint != "err-1" || ctx.ErrorHistory[0].Count != 2 || !ctx.ErrorHistory[0].Resolved {
		t.Fatalf("unexpected error history entry: %+v", ctx.ErrorHistory[0])
	}
	if ctx.APISchema == nil || ctx.Performance == nil {
		t.Fatalf("expected APISchema and Performance data, got APISchema=%v Performance=%v", ctx.APISchema, ctx.Performance)
	}
}

func TestSessionStore_MarkDirtyFlushAndHandleSessionStore(t *testing.T) {
	t.Parallel()
	store := newTestSessionStore(t)

	store.MarkDirty("noise", "config", []byte(`{"enabled":false}`))
	store.flushDirty()

	dirtyLoaded, err := store.Load("noise", "config")
	if err != nil {
		t.Fatalf("Load() after flushDirty() error = %v", err)
	}
	if string(dirtyLoaded) != `{"enabled":false}` {
		t.Fatalf("dirty flush mismatch: got %s", string(dirtyLoaded))
	}

	// Invalid dirty writes should be dropped.
	store.MarkDirty("../noise", "config", []byte(`{}`))
	store.MarkDirty("noise", "../config", []byte(`{}`))
	store.dirtyMu.Lock()
	if len(store.dirty) != 0 {
		store.dirtyMu.Unlock()
		t.Fatalf("invalid MarkDirty inputs should not be buffered, dirty=%v", store.dirty)
	}
	store.dirtyMu.Unlock()

	saveResp, err := store.HandleSessionStore(SessionStoreArgs{
		Action:    "save",
		Namespace: "session",
		Key:       "state",
		Data:      json.RawMessage(`{"step":1}`),
	})
	if err != nil {
		t.Fatalf("HandleSessionStore(save) error = %v", err)
	}
	var savePayload map[string]any
	if err := json.Unmarshal(saveResp, &savePayload); err != nil {
		t.Fatalf("unmarshal save response error = %v", err)
	}
	if savePayload["status"] != "saved" {
		t.Fatalf("save status = %v, want saved", savePayload["status"])
	}

	loadResp, err := store.HandleSessionStore(SessionStoreArgs{
		Action:    "load",
		Namespace: "session",
		Key:       "state",
	})
	if err != nil {
		t.Fatalf("HandleSessionStore(load) error = %v", err)
	}
	var loadPayload map[string]any
	if err := json.Unmarshal(loadResp, &loadPayload); err != nil {
		t.Fatalf("unmarshal load response error = %v", err)
	}
	loadedData, ok := loadPayload["data"].(map[string]any)
	if !ok || loadedData["step"] != float64(1) {
		t.Fatalf("load payload data = %v, want step=1", loadPayload["data"])
	}

	if _, err := store.HandleSessionStore(SessionStoreArgs{Action: "unknown"}); err == nil {
		t.Fatal("HandleSessionStore(unknown) should fail")
	}
}
