// stop_test.go — Tests daemon stop and cleanup ownership.

package procctl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupPIDFilesRemovesCanonicalPIDFile(t *testing.T) {
	t.Setenv("KABOOM_STATE_DIR", t.TempDir())
	const port = 7890
	path := PIDFilePath(port)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("424242"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	CleanupPIDFiles()

	if pid := ReadPIDFile(port); pid != 0 {
		t.Fatalf("ReadPIDFile(%d) = %d after cleanup, want 0", port, pid)
	}
}
