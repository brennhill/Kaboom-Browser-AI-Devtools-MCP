// store_test.go — Durable evidence artifact storage tests.

package verification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreRoundTripsContentAddressedArtifact(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	artifact := mustEvidence(t, EvidenceInput{Kind: "dom", Tool: "observe", Action: "dom", CorrelationID: "qa-1", CapturedAt: time.Now().UTC(), Content: map[string]any{"visible": true}})
	if err := store.Save(artifact); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Save(artifact); err != nil {
		t.Fatalf("idempotent Save() error = %v", err)
	}
	loaded, err := store.Load(artifact.Ref.ID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Ref.ID != artifact.Ref.ID || string(loaded.Content) != string(artifact.Content) {
		t.Fatalf("round trip mismatch: %#v / %#v", loaded, artifact)
	}
	entries, err := os.ReadDir(store.Dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("stored entries = %d, err = %v", len(entries), err)
	}
	if filepath.Ext(entries[0].Name()) != ".json" {
		t.Fatalf("unexpected artifact filename %q", entries[0].Name())
	}
}

func TestStoreSurfacesLocalPersistenceFailures(t *testing.T) {
	artifact := mustEvidence(t, EvidenceInput{Kind: "dom", Tool: "observe", Action: "dom", CorrelationID: "qa-io", CapturedAt: time.Now().UTC(), Content: map[string]any{"visible": true}})
	tampered := artifact
	tampered.Content = []byte(`{"visible":false}`)
	if err := (Store{Dir: t.TempDir()}).Save(tampered); err == nil || !strings.Contains(err.Error(), "content address") {
		t.Fatalf("tampered Save error = %v", err)
	}
	if err := (Store{}).Save(artifact); err == nil || !strings.Contains(err.Error(), "directory is required") {
		t.Fatalf("missing-directory Save error = %v", err)
	}

	blockingFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (Store{Dir: blockingFile}).Save(artifact); err == nil || !strings.Contains(err.Error(), "inspect evidence artifact") {
		t.Fatalf("blocked-directory Save error = %v", err)
	}

	store := Store{Dir: t.TempDir()}
	path, err := store.path(artifact.Ref.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxStoredArtifactBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(artifact.Ref.ID); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized Load error = %v", err)
	}
}

func TestStoreRejectsTraversalMissingAndTamperedArtifacts(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if _, err := store.Load("../../private"); err == nil {
		t.Fatal("path traversal ID should fail")
	}
	if _, err := store.Load("sha256:" + string(make([]byte, 64))); err == nil {
		t.Fatal("invalid hash ID should fail")
	}

	artifact := mustEvidence(t, EvidenceInput{Kind: "dom", Tool: "observe", Action: "dom", CorrelationID: "qa-1", CapturedAt: time.Now().UTC(), Content: map[string]any{"visible": true}})
	if err := store.Save(artifact); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	path, err := store.path(artifact.Ref.ID)
	if err != nil {
		t.Fatalf("path() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":"1"}`), 0o600); err != nil {
		t.Fatalf("tamper artifact: %v", err)
	}
	if _, err := store.Load(artifact.Ref.ID); err == nil {
		t.Fatal("tampered artifact should fail")
	}
}
