//go:build windows
// +build windows

// Purpose: Provides no-op CheckHardlink on Windows where Nlink detection is not reliably available.
// Docs: docs/features/feature/file-upload/index.md

package uploadsec

import "os"

// CheckHardlink is a no-op on Windows where Nlink detection is not reliably available.
func CheckHardlink(_ os.FileInfo) error {
	return nil
}
