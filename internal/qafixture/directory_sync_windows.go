// directory_sync_windows.go — Isolates unsupported Windows directory-sync semantics.

//go:build windows

package qafixture

func syncRegistryDirectory(string) error {
	// Windows os.File.Sync does not provide a portable directory-handle durability
	// contract. The same-directory MoveFileEx rename remains atomic, while file
	// contents were already flushed before replacement.
	return nil
}
