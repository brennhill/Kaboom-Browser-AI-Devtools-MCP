// Purpose: Decides which directories and files a repository walk may read.
// Why: Every hook scanner must prune the same tree, or two of them disagree about
//      what the project contains.
// Docs: docs/features/feature/convention-engine/index.md

package projectscan

import (
	"os"
	"path/filepath"
	"strings"
)

// MaxFileSize is the point at which a file stops being source somebody wrote.
//
// Bundles and minified output routinely run to megabytes on one line, and reading
// them costs more than the whole rest of the walk while contributing patterns no
// human chose.
const MaxFileSize = 100 * 1024

// isSkippedDir reports whether a directory's contents are not this project's own
// source.
//
// Scanning them attributes a dependency's conventions to the project — a repo with
// node_modules present would be told its convention is whatever React uses. A
// function rather than a package map so no caller can add an entry at runtime and
// change what a later scan sees.
func isSkippedDir(name string) bool {
	switch name {
	case ".git", "vendor", "node_modules", "dist", "build", ".next", "__pycache__", ".cache", ".claude":
		return true
	}
	return false
}

// DirectoryDecision reports what a WalkDir callback should do with an entry.
//
// The boolean says whether the decision applies: false means the entry is a file
// the caller should go on to consider itself. Returned as (error, bool) because
// the error is filepath.SkipDir — a walk instruction, not a failure.
func DirectoryDecision(d os.DirEntry) (error, bool) {
	if !d.IsDir() {
		return nil, false
	}
	if isSkippedDir(d.Name()) || (strings.HasPrefix(d.Name(), ".") && d.Name() != ".") {
		return filepath.SkipDir, true
	}
	return nil, true
}

// TooLarge reports whether an entry is past MaxFileSize, or cannot be measured.
//
// An unmeasurable entry is skipped rather than read: it is a file that vanished
// or that this process cannot stat, and reading it is the case that hangs on a
// dead network mount.
func TooLarge(d os.DirEntry) bool {
	info, err := d.Info()
	return err != nil || info.Size() > MaxFileSize
}
