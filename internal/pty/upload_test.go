// upload_test.go — Tests for terminal session image upload.

package pty

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpload_Success(t *testing.T) {
	dir := t.TempDir()
	r := strings.NewReader("fake png data")
	result, err := Upload(dir, "sess-1", "image/png", "screenshot.png", r)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	expected := filepath.Join(uploadDirName, "sess-1", "screenshot.png")
	if result.RelPath != expected {
		t.Fatalf("expected path %q, got %q", expected, result.RelPath)
	}
	if result.Size != 13 {
		t.Fatalf("expected size 13, got %d", result.Size)
	}

	data, err := os.ReadFile(filepath.Join(dir, result.RelPath))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "fake png data" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestUpload_TooLarge(t *testing.T) {
	dir := t.TempDir()
	r := io.LimitReader(zeroReader{}, uploadMaxSize+1)
	_, err := Upload(dir, "sess-1", "image/png", "big.png", r)
	if !errors.Is(err, ErrUploadTooLarge) {
		t.Fatalf("expected ErrUploadTooLarge, got: %v", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// failingReader yields one chunk of data and then fails, simulating a stream
// that dies mid-upload (dropped connection, truncated body).
type failingReader struct{ done bool }

func (f *failingReader) Read(p []byte) (int, error) {
	if !f.done {
		f.done = true
		return copy(p, []byte("partial data")), nil
	}
	return 0, errors.New("simulated stream failure")
}

func TestUpload_WriteErrorRemovesPartial(t *testing.T) {
	dir := t.TempDir()
	_, err := Upload(dir, "sess-1", "image/png", "screenshot.png", &failingReader{})
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
	// The partial file must not survive a failed upload — a leftover would be a
	// truncated image reported (by its mere presence) as a valid capture.
	partial := filepath.Join(dir, uploadDirName, "sess-1", "screenshot.png")
	if _, statErr := os.Stat(partial); !os.IsNotExist(statErr) {
		t.Fatalf("expected partial file removed, stat err = %v", statErr)
	}
}

func TestUpload_InvalidContentType(t *testing.T) {
	dir := t.TempDir()
	r := strings.NewReader("data")
	_, err := Upload(dir, "sess-1", "text/html", "page.html", r)
	if !errors.Is(err, ErrUploadInvalidType) {
		t.Fatalf("expected ErrUploadInvalidType, got: %v", err)
	}
}

func TestUpload_SanitizesFilename(t *testing.T) {
	dir := t.TempDir()
	r := strings.NewReader("data")
	result, err := Upload(dir, "sess-1", "image/png", "../../../etc/passwd", r)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if strings.Contains(result.RelPath, "..") {
		t.Fatalf("path traversal not sanitized: %s", result.RelPath)
	}
}

func TestUpload_EmptyFilename(t *testing.T) {
	dir := t.TempDir()
	r := strings.NewReader("data")
	result, err := Upload(dir, "sess-1", "image/png", "", r)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if result.RelPath == "" {
		t.Fatal("expected non-empty path for empty filename")
	}
	if filepath.Ext(result.RelPath) != ".png" {
		t.Fatalf("expected .png extension, got %s", filepath.Ext(result.RelPath))
	}
}

func TestUpload_AddsExtension(t *testing.T) {
	dir := t.TempDir()
	r := strings.NewReader("data")
	result, err := Upload(dir, "sess-1", "image/jpeg", "photo", r)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if filepath.Ext(result.RelPath) != ".jpeg" {
		t.Fatalf("expected .jpeg extension, got %s", filepath.Ext(result.RelPath))
	}
}

func TestUpload_AllowedContentTypes(t *testing.T) {
	for _, ct := range []string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml"} {
		dir := t.TempDir()
		r := strings.NewReader("data")
		_, err := Upload(dir, "sess-1", ct, "file", r)
		if err != nil {
			t.Fatalf("content type %q should be allowed, got: %v", ct, err)
		}
	}
}

func TestUpload_RejectedContentTypes(t *testing.T) {
	for _, ct := range []string{"text/html", "application/json", "video/mp4", ""} {
		dir := t.TempDir()
		r := strings.NewReader("data")
		_, err := Upload(dir, "sess-1", ct, "file", r)
		if !errors.Is(err, ErrUploadInvalidType) {
			t.Fatalf("content type %q should be rejected, got: %v", ct, err)
		}
	}
}
