// Purpose: Tests for persistence branch path coverage.
// Docs: docs/features/feature/persistent-memory/index.md

package persistence

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

type faultSessionFilesystem struct {
	sessionFilesystem
	readErr    error
	readDirErr error
	mkdirErr   error
	removeErr  error
	walkErr    error
	write      func(string, []byte, os.FileMode) error
}

func (files *faultSessionFilesystem) MkdirAll(path string, permissions fs.FileMode) error {
	if files.mkdirErr != nil {
		return files.mkdirErr
	}
	return files.sessionFilesystem.MkdirAll(path, permissions)
}

func (files *faultSessionFilesystem) ReadFile(path string) ([]byte, error) {
	if files.readErr != nil {
		return nil, files.readErr
	}
	return files.sessionFilesystem.ReadFile(path)
}

func (files *faultSessionFilesystem) ReadDir(path string) ([]os.DirEntry, error) {
	if files.readDirErr != nil {
		return nil, files.readDirErr
	}
	return files.sessionFilesystem.ReadDir(path)
}

func (files *faultSessionFilesystem) Remove(path string) error {
	if files.removeErr != nil {
		return files.removeErr
	}
	return files.sessionFilesystem.Remove(path)
}

func (files *faultSessionFilesystem) Walk(root string, walkFn filepath.WalkFunc) error {
	if files.walkErr != nil {
		return files.walkErr
	}
	return files.sessionFilesystem.Walk(root, walkFn)
}

func (files *faultSessionFilesystem) WriteFile(path string, data []byte, permissions fs.FileMode) error {
	if files.write != nil {
		return files.write(path, data, permissions)
	}
	return files.sessionFilesystem.WriteFile(path, data, permissions)
}

func replaceSessionWriter(store *SessionStore, write func(string, []byte, os.FileMode) error) sessionFilesystem {
	previous := store.files
	store.files = &faultSessionFilesystem{sessionFilesystem: previous, write: write}
	return previous
}

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

func TestSessionStoreImmediateWriteFaultsPreserveStateAndReportRecovery(t *testing.T) {
	const privateValue = `{"private":"must-not-leak"}`
	for _, kind := range []statefault.Kind{
		statefault.Write,
		statefault.Sync,
		statefault.Rename,
		statefault.DirectorySync,
		statefault.PartialWrite,
		statefault.Quota,
	} {
		t.Run(string(kind), func(t *testing.T) {
			diagnostics := statediag.NewCollector()
			store, err := newSessionStoreInDir(t.TempDir(), t.TempDir(), defaultFlushInterval, diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(store.Shutdown)
			if err := store.Save("session", "state", []byte(`{"version":"old"}`)); err != nil {
				t.Fatal(err)
			}

			previousFiles := replaceSessionWriter(store, func(string, []byte, os.FileMode) error {
				return statefault.New(kind, privateValue).Error()
			})
			err = store.Save("session", "state", []byte(privateValue))
			if err == nil || strings.Contains(err.Error(), privateValue) {
				t.Fatalf("Save() error = %v, want redacted failure", err)
			}
			persisted, loadErr := store.Load("session", "state")
			if loadErr != nil || string(persisted) != `{"version":"old"}` {
				t.Fatalf("persisted state = %q err=%v, want prior value", persisted, loadErr)
			}

			got := diagnostics.Snapshot()
			if len(got) != 1 || got[0].Name != "session_store_write_state" || got[0].Lifecycle != statediag.LifecycleActive {
				t.Fatalf("write diagnostics = %#v", got)
			}
			if strings.Contains(got[0].Detail, privateValue) || strings.Contains(got[0].Detail, "session/state") {
				t.Fatalf("write diagnostic leaked state identity or value: %#v", got[0])
			}

			store.files = previousFiles
			if err := store.Save("session", "state", []byte(`{"version":"recovered"}`)); err != nil {
				t.Fatal(err)
			}
			got = diagnostics.Snapshot()
			if len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
				t.Fatalf("resolved write diagnostics = %#v", got)
			}
		})
	}
}

func TestSessionStoreMetadataWriteFailureIsRedactedAndActionable(t *testing.T) {
	diagnostics := statediag.NewCollector()
	store, err := newSessionStoreInDir(t.TempDir(), t.TempDir(), defaultFlushInterval, diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Shutdown)
	previousFiles := replaceSessionWriter(store, func(string, []byte, os.FileMode) error {
		return statefault.New(statefault.PartialWrite, "private-project-path").Error()
	})
	defer func() { store.files = previousFiles }()

	err = store.saveMeta()
	if err == nil || strings.Contains(err.Error(), "private-project-path") {
		t.Fatalf("saveMeta() error = %v, want redacted failure", err)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "session_metadata_write_state" || got[0].Fix == "" {
		t.Fatalf("metadata write diagnostics = %#v", got)
	}
}

func TestSessionStoreDeferredWriteFailuresRetainObligationUntilRecovery(t *testing.T) {
	const privateValue = `{"private":"deferred-secret"}`
	for _, kind := range []statefault.Kind{statefault.Write, statefault.Cancellation, statefault.PartialWrite} {
		t.Run(string(kind), func(t *testing.T) {
			diagnostics := statediag.NewCollector()
			store, err := newSessionStoreInDir(t.TempDir(), t.TempDir(), defaultFlushInterval, diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(store.Shutdown)
			store.MarkDirty("session", "deferred", []byte(privateValue))
			previousFiles := replaceSessionWriter(store, func(string, []byte, os.FileMode) error {
				return statefault.New(kind, privateValue).Error()
			})

			store.flushDirty()
			store.dirtyMu.Lock()
			retained := string(store.dirty["session/deferred"])
			store.dirtyMu.Unlock()
			if retained != privateValue {
				t.Fatalf("retained obligation = %q, want original value", retained)
			}
			got := diagnostics.Snapshot()
			if len(got) != 1 || got[0].Name != "session_deferred_write_state" || got[0].Lifecycle != statediag.LifecycleActive {
				t.Fatalf("deferred diagnostics = %#v", got)
			}
			if strings.Contains(got[0].Detail, privateValue) || strings.Contains(got[0].Detail, "session/deferred") {
				t.Fatalf("deferred diagnostic leaked state: %#v", got[0])
			}

			store.files = previousFiles
			store.flushDirty()
			store.dirtyMu.Lock()
			remaining := len(store.dirty)
			store.dirtyMu.Unlock()
			if remaining != 0 {
				t.Fatalf("dirty obligations after recovery = %d", remaining)
			}
			persisted, loadErr := store.Load("session", "deferred")
			if loadErr != nil || string(persisted) != privateValue {
				t.Fatalf("persisted deferred value = %q err=%v", persisted, loadErr)
			}
			got = diagnostics.Snapshot()
			if len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
				t.Fatalf("resolved deferred diagnostics = %#v", got)
			}
		})
	}
}

func TestSessionStoreDeferredRetryNeverOverwritesNewerQueuedValue(t *testing.T) {
	store, err := newSessionStoreInDir(t.TempDir(), t.TempDir(), defaultFlushInterval, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Shutdown)
	store.MarkDirty("session", "state", []byte("old"))
	previousFiles := replaceSessionWriter(store, func(string, []byte, os.FileMode) error {
		store.MarkDirty("session", "state", []byte("new"))
		return statefault.New(statefault.Write, "private").Error()
	})

	store.flushDirty()
	store.dirtyMu.Lock()
	retained := string(store.dirty["session/state"])
	store.dirtyMu.Unlock()
	if retained != "new" {
		t.Fatalf("queued value = %q, want newer value", retained)
	}
	store.files = previousFiles
}

func TestSessionStoreMarkDirtyOwnsQueuedBytes(t *testing.T) {
	store, err := newSessionStoreInDir(t.TempDir(), t.TempDir(), defaultFlushInterval, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Shutdown)
	value := []byte("original")
	store.MarkDirty("session", "state", value)
	copy(value, "mutated!")

	store.flushDirty()
	persisted, err := store.Load("session", "state")
	if err != nil || string(persisted) != "original" {
		t.Fatalf("persisted value = %q err=%v, want owned original", persisted, err)
	}
}

func TestSessionStoreShutdownRetainsFailedDeferredWrite(t *testing.T) {
	diagnostics := statediag.NewCollector()
	store, err := newSessionStoreInDir(t.TempDir(), t.TempDir(), defaultFlushInterval, diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	store.MarkDirty("session", "shutdown", []byte("latest"))
	replaceSessionWriter(store, func(string, []byte, os.FileMode) error {
		return statefault.New(statefault.Write, "private").Error()
	})

	store.Shutdown()
	store.dirtyMu.Lock()
	retained := string(store.dirty["session/shutdown"])
	store.dirtyMu.Unlock()
	if retained != "latest" {
		t.Fatalf("shutdown obligation = %q, want latest", retained)
	}
	names := make(map[string]bool)
	for _, diagnostic := range diagnostics.Snapshot() {
		names[diagnostic.Name] = diagnostic.Lifecycle == statediag.LifecycleActive
	}
	if !names[deferredWriteDiagnostic] || !names["session_metadata_write_state"] {
		t.Fatalf("shutdown diagnostics = %#v", diagnostics.Snapshot())
	}
}

func TestSessionStoreStartupReadFailureFallsBackWithRedactedDoctorEvidence(t *testing.T) {
	const private = "private-startup-state"
	diagnostics := statediag.NewCollector()
	files := &faultSessionFilesystem{
		sessionFilesystem: localSessionFilesystem{},
		readErr:           statefault.New(statefault.Read, private).Error(),
	}
	store, err := newSessionStoreWithFilesystem(t.TempDir(), t.TempDir(), defaultFlushInterval, diagnostics, files)
	if err != nil {
		t.Fatalf("startup read fallback failed: %v", err)
	}
	t.Cleanup(store.Shutdown)
	if store.GetMeta().SessionCount != 1 {
		t.Fatalf("metadata fallback = %#v", store.GetMeta())
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "session_metadata_state" || strings.Contains(got[0].Detail, private) {
		t.Fatalf("startup diagnostics = %#v", got)
	}
}

func TestSessionStoreStartupDirectoryFailureIsRedactedAndDiagnosable(t *testing.T) {
	const private = "private-project-directory"
	diagnostics := statediag.NewCollector()
	files := &faultSessionFilesystem{
		sessionFilesystem: localSessionFilesystem{},
		mkdirErr:          statefault.New(statefault.Write, private).Error(),
	}
	store, err := newSessionStoreWithFilesystem(t.TempDir(), t.TempDir(), defaultFlushInterval, diagnostics, files)
	if store != nil || err == nil || strings.Contains(err.Error(), private) {
		t.Fatalf("startup directory result store=%v err=%v", store, err)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "session_directory_state" || got[0].Fix == "" || strings.Contains(got[0].Detail, private) {
		t.Fatalf("startup directory diagnostics = %#v", got)
	}
}

func TestSessionStoreEmptyMetadataIsCorruptionNotExpectedAbsence(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "meta.json"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := statediag.NewCollector()
	store, err := newSessionStoreInDir(t.TempDir(), projectDir, defaultFlushInterval, diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Shutdown)
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "session_metadata_state" || got[0].Lifecycle != statediag.LifecycleActive {
		t.Fatalf("empty metadata diagnostics = %#v", got)
	}
}

func TestSessionStoreReadListDeleteAndQuotaFaultsAreDiagnosable(t *testing.T) {
	const private = "private-filesystem-state"
	for _, testCase := range []struct {
		name           string
		diagnosticName string
		configure      func(*faultSessionFilesystem)
		invoke         func(*SessionStore) error
	}{
		{
			name: "read", diagnosticName: "session_store_read_state",
			configure: func(files *faultSessionFilesystem) { files.readErr = statefault.New(statefault.Read, private).Error() },
			invoke:    func(store *SessionStore) error { _, err := store.Load("session", "state"); return err },
		},
		{
			name: "list", diagnosticName: "session_store_list_state",
			configure: func(files *faultSessionFilesystem) {
				files.readDirErr = statefault.New(statefault.Read, private).Error()
			},
			invoke: func(store *SessionStore) error { _, err := store.List("session"); return err },
		},
		{
			name: "stats", diagnosticName: "session_store_stats_state",
			configure: func(files *faultSessionFilesystem) {
				files.readDirErr = statefault.New(statefault.Read, private).Error()
			},
			invoke: func(store *SessionStore) error { _, err := store.Stats(); return err },
		},
		{
			name: "delete", diagnosticName: "session_store_delete_state",
			configure: func(files *faultSessionFilesystem) {
				files.removeErr = statefault.New(statefault.Write, private).Error()
			},
			invoke: func(store *SessionStore) error { return store.Delete("session", "state") },
		},
		{
			name: "quota", diagnosticName: "session_store_quota_state",
			configure: func(files *faultSessionFilesystem) { files.walkErr = statefault.New(statefault.Quota, private).Error() },
			invoke:    func(store *SessionStore) error { return store.Save("session", "new", []byte(private)) },
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			diagnostics := statediag.NewCollector()
			store, err := newSessionStoreInDir(t.TempDir(), t.TempDir(), defaultFlushInterval, diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(store.Shutdown)
			if err := store.Save("session", "state", []byte(`{"old":true}`)); err != nil {
				t.Fatal(err)
			}
			files := &faultSessionFilesystem{sessionFilesystem: store.files}
			testCase.configure(files)
			store.files = files

			err = testCase.invoke(store)
			if err == nil || strings.Contains(err.Error(), private) {
				t.Fatalf("operation error = %v, want redacted failure", err)
			}
			got := diagnostics.Snapshot()
			if len(got) != 1 || got[0].Name != testCase.diagnosticName || got[0].Fix == "" || strings.Contains(got[0].Detail, private) {
				t.Fatalf("operation diagnostics = %#v", got)
			}
			store.files = localSessionFilesystem{}
			if err := testCase.invoke(store); err != nil {
				t.Fatalf("operation recovery failed: %v", err)
			}
			got = diagnostics.Snapshot()
			if len(got) != 1 || got[0].Lifecycle != statediag.LifecycleRecovered {
				t.Fatalf("resolved operation diagnostics = %#v", got)
			}
		})
	}
}

func TestSessionStoreContextReadFaultsFallBackAndResolveWithoutLeakingState(t *testing.T) {
	const private = "private-context-state"
	diagnostics := statediag.NewCollector()
	store, err := newSessionStoreInDir(t.TempDir(), t.TempDir(), defaultFlushInterval, diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Shutdown)
	store.files = &faultSessionFilesystem{
		sessionFilesystem: localSessionFilesystem{},
		readErr:           statefault.New(statefault.Read, private).Error(),
		readDirErr:        statefault.New(statefault.Read, private).Error(),
	}

	context := store.LoadSessionContext()
	if len(context.Baselines) != 0 || len(context.ErrorHistory) != 0 || context.NoiseConfig != nil || context.APISchema != nil || context.Performance != nil {
		t.Fatalf("context fault fallback = %#v", context)
	}
	wantNames := map[string]bool{
		"baseline_context_state":    false,
		"noise_context_state":       false,
		"error_history_state":       false,
		"api_schema_state":          false,
		"performance_context_state": false,
	}
	for _, diagnostic := range diagnostics.Snapshot() {
		if strings.Contains(diagnostic.Detail, private) {
			t.Fatalf("context diagnostic leaked private state: %#v", diagnostic)
		}
		if _, exists := wantNames[diagnostic.Name]; exists {
			wantNames[diagnostic.Name] = diagnostic.Lifecycle == statediag.LifecycleActive
		}
	}
	for name, active := range wantNames {
		if !active {
			t.Fatalf("missing active context diagnostic %q: %#v", name, diagnostics.Snapshot())
		}
	}

	store.files = localSessionFilesystem{}
	store.LoadSessionContext()
	for _, diagnostic := range diagnostics.Snapshot() {
		if _, tracked := wantNames[diagnostic.Name]; tracked && diagnostic.Lifecycle != statediag.LifecycleRecovered {
			t.Fatalf("context diagnostic did not recover after expected absence: %#v", diagnostic)
		}
	}
}

func TestSessionStoreRestartPreservesDurableStateAndAdvancesSession(t *testing.T) {
	projectPath := t.TempDir()
	projectDir := t.TempDir()
	first, err := newSessionStoreInDir(projectPath, projectDir, defaultFlushInterval, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Save("session", "restart", []byte(`{"durable":true}`)); err != nil {
		t.Fatal(err)
	}
	firstCount := first.GetMeta().SessionCount
	first.Shutdown()
	if got := statefault.New(statefault.Restart, "private").NextGeneration(uint64(firstCount)); got != uint64(firstCount+1) {
		t.Fatalf("restart fixture generation = %d", got)
	}

	second, err := newSessionStoreInDir(projectPath, projectDir, defaultFlushInterval, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.Shutdown)
	persisted, err := second.Load("session", "restart")
	if err != nil || string(persisted) != `{"durable":true}` {
		t.Fatalf("restart state = %q err=%v", persisted, err)
	}
	if second.GetMeta().SessionCount != firstCount+1 {
		t.Fatalf("restart session count = %d, want %d", second.GetMeta().SessionCount, firstCount+1)
	}
}
