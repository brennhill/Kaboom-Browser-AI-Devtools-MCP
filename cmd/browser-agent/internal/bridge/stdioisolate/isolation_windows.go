//go:build windows
// +build windows

// isolation_windows.go -- Windows fallback for stdout duplication -- reuses the existing stdout handle since Windows lacks dup2.
// Why: Provides a no-op platform shim so bridge IO isolation compiles on Windows without syscall dependencies.
// Docs: docs/features/feature/bridge-restart/index.md

package stdioisolate

import "os"

func duplicateStdoutForTransport(stdout *os.File) (*os.File, error) {
	// Windows fallback: keep existing stdout handle as transport writer and rely
	// on process-level stream reassignment below.
	return stdout, nil
}

func redirectProcessStdStreams(target *os.File) error {
	os.Stdout = target
	os.Stderr = target
	return nil
}
