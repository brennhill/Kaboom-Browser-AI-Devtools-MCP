// statefile_test.go — Proves durable atomic state-file write and move semantics.
package statefile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var errInjected = errors.New("injected state-file failure")

type fakeFile struct {
	name  string
	steps *[]string
	fail  Stage
}

func (file *fakeFile) Name() string { return file.name }
func (file *fakeFile) Chmod(fs.FileMode) error {
	*file.steps = append(*file.steps, "chmod")
	return file.errorAt(StageChmod)
}
func (file *fakeFile) Write([]byte) (int, error) {
	*file.steps = append(*file.steps, "write")
	if err := file.errorAt(StageWrite); err != nil {
		return 2, err
	}
	return 4, nil
}
func (file *fakeFile) Sync() error {
	*file.steps = append(*file.steps, "file_sync")
	return file.errorAt(StageFileSync)
}
func (file *fakeFile) Close() error {
	*file.steps = append(*file.steps, "close")
	return file.errorAt(StageClose)
}
func (file *fakeFile) errorAt(stage Stage) error {
	if file.fail == stage {
		return errInjected
	}
	return nil
}

func TestWriteUsesDurableAtomicOrder(t *testing.T) {
	steps := []string{}
	ops := defaultOperations()
	ops.mkdirAll = func(string, fs.FileMode) error { steps = append(steps, "mkdir"); return nil }
	ops.createTemp = func(string, string) (temporaryFile, error) {
		steps = append(steps, "create")
		return &fakeFile{name: "/state/.tmp", steps: &steps}, nil
	}
	ops.rename = func(string, string) error { steps = append(steps, "rename"); return nil }
	ops.syncDirectory = func(string) error { steps = append(steps, "directory_sync"); return nil }
	ops.remove = func(string) error { steps = append(steps, "remove"); return nil }

	if err := writeWith("/state/value.json", []byte("data"), 0o600, ops); err != nil {
		t.Fatal(err)
	}
	want := []string{"mkdir", "create", "chmod", "write", "file_sync", "close", "rename", "directory_sync"}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
}

func TestWriteClassifiesFailuresAndCleansUncommittedTemporary(t *testing.T) {
	for _, stage := range []Stage{StageMkdir, StageCreate, StageChmod, StageWrite, StageFileSync, StageClose, StageRename, StageDirectorySync} {
		t.Run(string(stage), func(t *testing.T) {
			steps := []string{}
			removed := false
			ops := defaultOperations()
			ops.mkdirAll = func(string, fs.FileMode) error {
				if stage == StageMkdir {
					return errInjected
				}
				return nil
			}
			ops.createTemp = func(string, string) (temporaryFile, error) {
				if stage == StageCreate {
					return nil, errInjected
				}
				return &fakeFile{name: "/state/.tmp", steps: &steps, fail: stage}, nil
			}
			ops.rename = func(string, string) error {
				if stage == StageRename {
					return errInjected
				}
				return nil
			}
			ops.syncDirectory = func(string) error {
				if stage == StageDirectorySync {
					return errInjected
				}
				return nil
			}
			ops.remove = func(string) error { removed = true; return nil }

			err := writeWith("/state/value.json", []byte("data"), 0o600, ops)
			if FailureStage(err) != stage || !errors.Is(err, errInjected) {
				t.Fatalf("error = %v stage=%q, want %q", err, FailureStage(err), stage)
			}
			if stage != StageMkdir && stage != StageCreate && stage != StageDirectorySync && !removed {
				t.Fatal("uncommitted temporary file was not cleaned")
			}
			if stage == StageDirectorySync && removed {
				t.Fatal("committed destination must not be removed after directory-sync failure")
			}
		})
	}
}

func TestWritePreservesExistingDestinationBeforeRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	ops := defaultOperations()
	ops.rename = func(string, string) error { return errInjected }
	if err := writeWith(path, []byte("new"), 0o600, ops); FailureStage(err) != StageRename {
		t.Fatalf("error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "old" {
		t.Fatalf("destination = %q err=%v, want old", data, err)
	}
}

func TestMoveRenamesThenSyncsDestinationDirectory(t *testing.T) {
	steps := []string{}
	ops := defaultOperations()
	ops.rename = func(string, string) error { steps = append(steps, "rename"); return nil }
	ops.syncDirectory = func(path string) error { steps = append(steps, "sync:"+path); return nil }
	if err := moveWith("/state/value", "/state/value.corrupt", ops); err != nil {
		t.Fatal(err)
	}
	if want := []string{"rename", "sync:/state"}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
}

func TestMoveClassifiesRenameAndDirectorySyncFailures(t *testing.T) {
	for _, stage := range []Stage{StageRename, StageDirectorySync} {
		t.Run(string(stage), func(t *testing.T) {
			ops := defaultOperations()
			ops.rename = func(string, string) error {
				if stage == StageRename {
					return errInjected
				}
				return nil
			}
			ops.syncDirectory = func(string) error {
				if stage == StageDirectorySync {
					return errInjected
				}
				return nil
			}

			err := moveWith("/state/value", "/state/value.corrupt", ops)
			if FailureStage(err) != stage || !errors.Is(err, errInjected) {
				t.Fatalf("error = %v stage=%q, want %q", err, FailureStage(err), stage)
			}
		})
	}
}

func TestWriteReportsCleanupFailureWithoutLeakingPath(t *testing.T) {
	ops := defaultOperations()
	ops.mkdirAll = func(string, fs.FileMode) error { return nil }
	ops.createTemp = func(string, string) (temporaryFile, error) {
		steps := []string{}
		return &fakeFile{name: "/private/state/secret.tmp", steps: &steps, fail: StageWrite}, nil
	}
	ops.remove = func(string) error { return os.ErrPermission }

	err := writeWith("/private/state/value.json", []byte("data"), 0o600, ops)
	if FailureStage(err) != StageWrite || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("error = %v stage=%q", err, FailureStage(err))
	}
	if !strings.Contains(err.Error(), "state_file_cleanup_failed") {
		t.Fatalf("cleanup failure was omitted: %v", err)
	}
	if !HasFailureStage(err, StageWrite) || !HasFailureStage(err, StageCleanup) {
		t.Fatalf("joined failure stages were not discoverable: %v", err)
	}
	if strings.Contains(err.Error(), "/private/") {
		t.Fatalf("error leaked a state-file path: %v", err)
	}
}
