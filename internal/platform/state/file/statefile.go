// statefile.go — Canonical durable atomic writes and moves for persisted state.
package statefile

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Stage identifies the stable operation at which a state-file transaction failed.
type Stage string

const (
	StageMkdir         Stage = "mkdir"
	StageCreate        Stage = "create"
	StageChmod         Stage = "chmod"
	StageWrite         Stage = "write"
	StageFileSync      Stage = "file_sync"
	StageClose         Stage = "close"
	StageRename        Stage = "rename"
	StageDirectorySync Stage = "directory_sync"
	StageCleanup       Stage = "cleanup"
)

type failure struct {
	stage Stage
	cause error
}

func (err *failure) Error() string { return "state_file_" + string(err.stage) + "_failed" }
func (err *failure) Unwrap() error { return err.cause }

// FailureStage returns the stable failure stage without exposing paths or values.
func FailureStage(err error) Stage {
	var target *failure
	if errors.As(err, &target) {
		return target.stage
	}
	return ""
}

// HasFailureStage reports whether any error in a wrapped or joined failure tree
// belongs to stage. It preserves secondary cleanup failures for diagnostics.
func HasFailureStage(err error, stage Stage) bool {
	if err == nil {
		return false
	}
	if typed, ok := err.(*failure); ok && typed.stage == stage {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if HasFailureStage(child, stage) {
				return true
			}
		}
		return false
	}
	return HasFailureStage(errors.Unwrap(err), stage)
}

type temporaryFile interface {
	Name() string
	Chmod(fs.FileMode) error
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type operations struct {
	mkdirAll      func(string, fs.FileMode) error
	createTemp    func(string, string) (temporaryFile, error)
	rename        func(string, string) error
	remove        func(string) error
	syncDirectory func(string) error
}

func defaultOperations() operations {
	return operations{
		mkdirAll: os.MkdirAll, createTemp: func(dir, pattern string) (temporaryFile, error) { return os.CreateTemp(dir, pattern) },
		rename: os.Rename, remove: os.Remove, syncDirectory: syncDirectory,
	}
}

// Write durably replaces path with data through a same-directory temporary file.
func Write(path string, data []byte, permissions fs.FileMode) error {
	return writeWith(path, data, permissions, defaultOperations())
}

func writeWith(path string, data []byte, permissions fs.FileMode, ops operations) error {
	directory := filepath.Dir(path)
	if err := ops.mkdirAll(directory, 0o700); err != nil {
		return &failure{stage: StageMkdir, cause: err}
	}
	temporary, err := ops.createTemp(directory, ".state-*")
	if err != nil {
		return &failure{stage: StageCreate, cause: err}
	}
	temporaryPath := temporary.Name()
	open := true
	fail := func(stage Stage, cause error) error {
		cleanupErr := cleanupTemporary(temporary, temporaryPath, open, ops)
		if cleanupErr != nil {
			return errors.Join(&failure{stage: stage, cause: cause}, cleanupErr)
		}
		return &failure{stage: stage, cause: cause}
	}
	if err := temporary.Chmod(permissions); err != nil {
		return fail(StageChmod, err)
	}
	written, err := temporary.Write(data)
	if err != nil {
		return fail(StageWrite, err)
	}
	if written != len(data) {
		return fail(StageWrite, io.ErrShortWrite)
	}
	if err := temporary.Sync(); err != nil {
		return fail(StageFileSync, err)
	}
	if err := temporary.Close(); err != nil {
		open = false
		return fail(StageClose, err)
	}
	open = false
	if err := ops.rename(temporaryPath, path); err != nil {
		return fail(StageRename, err)
	}
	if err := ops.syncDirectory(directory); err != nil {
		return &failure{stage: StageDirectorySync, cause: err}
	}
	return nil
}

func cleanupTemporary(file temporaryFile, path string, open bool, ops operations) error {
	var closeErr error
	if open {
		closeErr = file.Close()
	}
	removeErr := ops.remove(path)
	if errors.Is(removeErr, os.ErrNotExist) {
		// EXPECTED_ABSENCE: an operation may have consumed the temporary path before cleanup.
		removeErr = nil
	}
	if closeErr != nil || removeErr != nil {
		return &failure{stage: StageCleanup, cause: errors.Join(closeErr, removeErr)}
	}
	return nil
}

// Move atomically renames a state file and durably commits its directory entry.
func Move(from, to string) error { return moveWith(from, to, defaultOperations()) }

func moveWith(from, to string, ops operations) error {
	if err := ops.rename(from, to); err != nil {
		return &failure{stage: StageRename, cause: err}
	}
	if err := ops.syncDirectory(filepath.Dir(to)); err != nil {
		return &failure{stage: StageDirectorySync, cause: err}
	}
	return nil
}
