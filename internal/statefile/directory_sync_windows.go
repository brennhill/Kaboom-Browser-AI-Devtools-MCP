// directory_sync_windows.go — Isolates unsupported Windows directory-sync semantics.

//go:build windows

package statefile

func syncDirectory(string) error {
	// Windows os.File.Sync has no portable directory-handle durability contract;
	// same-directory rename remains atomic after the temporary file is synced.
	return nil
}
