// upload.go — Terminal session image upload handler.
// Why: Allows agents to upload images (screenshots, diagrams) through a terminal session.

package pty

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	uploadMaxSize = 10 << 20 // 10 MB.
	uploadDirName = "terminal-uploads"
)

// Allowed MIME type prefixes for upload validation.
var allowedContentTypes = []string{
	"image/png",
	"image/jpeg",
	"image/gif",
	"image/webp",
	"image/svg+xml",
}

// ErrUploadTooLarge is returned when the upload exceeds the size limit.
var ErrUploadTooLarge = errors.New("pty: upload exceeds 10MB limit")

// ErrUploadInvalidType is returned when the content type is not an allowed image type.
var ErrUploadInvalidType = errors.New("pty: invalid content type, must be an image")

// UploadResult contains the result of a successful upload.
type UploadResult struct {
	RelPath string // Relative path from workspace root.
	Size    int64  // Bytes written.
}

// Upload saves an image to the workspace upload directory for a session.
// Returns the relative path suitable for referencing in results or telemetry.
func Upload(workspaceDir, sessionID, contentType, filename string, r io.Reader) (*UploadResult, error) {
	if !isAllowedContentType(contentType) {
		return nil, ErrUploadInvalidType
	}

	safe := sanitizeFilename(filename)
	if safe == "" {
		safe = fmt.Sprintf("upload-%d", time.Now().UnixMilli())
	}
	if filepath.Ext(safe) == "" {
		safe += extForContentType(contentType)
	}

	safeSession := sanitizeSessionID(sessionID)
	dir := filepath.Join(workspaceDir, uploadDirName, safeSession)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}

	path := filepath.Join(dir, safe)
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}

	limited := io.LimitReader(r, uploadMaxSize+1)
	n, err := io.Copy(f, limited)
	if err != nil {
		f.Close()
		os.Remove(path)
		return nil, fmt.Errorf("write file: %w", err)
	}
	if n > uploadMaxSize {
		f.Close()
		os.Remove(path)
		return nil, ErrUploadTooLarge
	}

	// Check the Close() error explicitly rather than deferring it away: a buffered
	// filesystem can surface a write failure (disk full, network mount drop) only
	// at Close, so a deferred-and-ignored Close would report a truncated file as a
	// successful upload. Fail loud and drop the partial. (CLAUDE.md rule 25.)
	if err := f.Close(); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("finalize file: %w", err)
	}

	relPath := filepath.Join(uploadDirName, safeSession, safe)
	return &UploadResult{RelPath: relPath, Size: n}, nil
}

func isAllowedContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	for _, allowed := range allowedContentTypes {
		if strings.HasPrefix(ct, allowed) {
			return true
		}
	}
	return false
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	if name == "." || name == ".." {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sanitizeSessionID reduces a session id to a single safe path segment so it can
// never escape the uploads directory. The id reaches here straight from the
// /terminal/start request (HandleTerminalStart does not constrain it), so a value
// like "../../foo" must not become a real parent-directory traversal. filepath.Base
// strips any path, and the char filter neutralizes cross-OS separators/dots.
func sanitizeSessionID(id string) string {
	id = filepath.Base(id)
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "default"
	}
	return out
}

func extForContentType(ct string) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.HasPrefix(ct, "image/png"):
		return ".png"
	case strings.HasPrefix(ct, "image/jpeg"):
		return ".jpeg"
	case strings.HasPrefix(ct, "image/gif"):
		return ".gif"
	case strings.HasPrefix(ct, "image/webp"):
		return ".webp"
	case strings.HasPrefix(ct, "image/svg+xml"):
		return ".svg"
	default:
		return ".bin"
	}
}
