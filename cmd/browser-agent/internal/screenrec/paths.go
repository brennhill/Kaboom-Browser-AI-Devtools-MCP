// Purpose: Filesystem/path and naming helpers for video recording artifacts.
// Why: Isolates slug/path safety and filename generation from recording state orchestration.
// Docs: docs/features/feature/tab-recording/index.md

package screenrec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// Dir returns the runtime recordings directory, creating it if needed.
func Dir() (string, error) {
	dir, err := state.RecordingsDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine recordings directory: %w", err)
	}
	// #nosec G301 -- directory: owner rwx, group rx for traversal
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("cannot create recordings directory: %w", err)
	}
	return dir, nil
}

func ReadDirs() []string {
	primaryDir, err := Dir()
	if err != nil {
		return nil
	}
	dirs := []string{primaryDir}

	legacyDir, err := state.LegacyRecordingsDir()
	if err != nil || legacyDir == "" || legacyDir == primaryDir {
		return dirs
	}
	if info, statErr := os.Stat(legacyDir); statErr == nil && info.IsDir() {
		dirs = append(dirs, legacyDir)
	}

	return dirs
}

func pathWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isSlugChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}

// sanitizeVideoSlug normalizes a recording name to a filesystem-safe slug.
func sanitizeVideoSlug(s string) string {
	s = strings.ToLower(s)
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if isSlugChar(s[i]) {
			result = append(result, s[i])
		} else {
			result = append(result, '-')
		}
	}
	s = collapseHyphens(string(result))
	if s == "" {
		s = "recording"
	}
	return s
}

func collapseHyphens(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// generateVideoFilename creates a unique filename: {slug}--{YYYY-MM-DD-HHmmss}.webm
func generateVideoFilename(name string) string {
	slug := sanitizeVideoSlug(name)
	ts := time.Now().Format("2006-01-02-150405")
	return fmt.Sprintf("%s--%s", slug, ts)
}

// clampFPS applies default and bounds to a requested FPS value.
func clampFPS(fps int) int {
	if fps == 0 {
		fps = 15
	}
	if fps < 5 {
		return 5
	}
	if fps > 60 {
		return 60
	}
	return fps
}

// validAudioModes lists allowed values for the audio parameter.
var validAudioModes = map[string]bool{
	"":     true,
	"tab":  true,
	"mic":  true,
	"both": true,
}

// resolveRecordingPath picks a unique .webm path inside dir, handling collisions.
func resolveRecordingPath(dir, name string) (fullName string, videoPath string) {
	fullName = generateVideoFilename(name)
	videoPath = filepath.Join(dir, fullName+".webm")
	if _, err := os.Stat(videoPath); err == nil {
		slug := sanitizeVideoSlug(name)
		fullName = fmt.Sprintf("%s--%s", slug, time.Now().Format("2006-01-02-150405.000000000"))
		videoPath = filepath.Join(dir, fullName+".webm")
	}
	return fullName, videoPath
}
