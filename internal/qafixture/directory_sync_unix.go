// directory_sync_unix.go — Commits fixture-registry rename entries on directory-sync platforms.

//go:build !windows

package qafixture

import "os"

func syncRegistryDirectory(path string) error {
	directory, err := os.Open(path) // #nosec G304 -- path is the registry owner's local state directory.
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
