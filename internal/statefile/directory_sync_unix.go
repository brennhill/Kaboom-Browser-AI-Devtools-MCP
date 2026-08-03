// directory_sync_unix.go — Commits renamed state-file directory entries on Unix.

//go:build !windows

package statefile

import "os"

func syncDirectory(path string) error {
	directory, err := os.Open(path) // #nosec G304 -- callers provide a locally owned state directory.
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
