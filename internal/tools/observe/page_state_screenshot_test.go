// analysis_save_test.go — Tests for saveScreenshotToPath.
package observe

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// validPNGDataURL returns a well-formed data URL with real base64 content.
func validPNGDataURL() string {
	raw := []byte("fake-png-image-bytes")
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
}

// validJPEGDataURL returns a well-formed JPEG data URL.
func validJPEGDataURL() string {
	raw := []byte("fake-jpeg-image-bytes")
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(raw)
}

// ============================================
// Valid saves
// ============================================

func TestSaveScreenshotToPath_PNG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")

	err := saveScreenshotToPath(path, validPNGDataURL())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("saved file is empty")
	}
	// Verify the bytes round-trip correctly.
	if string(data) != "fake-png-image-bytes" {
		t.Errorf("file content = %q, want %q", string(data), "fake-png-image-bytes")
	}
}

func TestSaveScreenshotToPath_JPG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.jpg")

	err := saveScreenshotToPath(path, validJPEGDataURL())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file does not exist: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("saved file is empty")
	}
}

func TestSaveScreenshotToPath_JPEG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.jpeg")

	err := saveScreenshotToPath(path, validJPEGDataURL())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file does not exist: %v", err)
	}
}

// ============================================
// Invalid extensions
// ============================================

func TestSaveScreenshotToPath_InvalidExtension_BMP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.bmp")

	err := saveScreenshotToPath(path, validPNGDataURL())
	if err == nil {
		t.Fatal("expected error for .bmp extension, got nil")
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("file should not have been created for invalid extension")
	}
}

func TestSaveScreenshotToPath_InvalidExtension_GIF(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.gif")

	err := saveScreenshotToPath(path, validPNGDataURL())
	if err == nil {
		t.Fatal("expected error for .gif extension, got nil")
	}
}

func TestSaveScreenshotToPath_InvalidExtension_TXT(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.txt")

	err := saveScreenshotToPath(path, validPNGDataURL())
	if err == nil {
		t.Fatal("expected error for .txt extension, got nil")
	}
}

func TestSaveScreenshotToPath_InvalidExtension_None(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "screenshot")

	err := saveScreenshotToPath(path, validPNGDataURL())
	if err == nil {
		t.Fatal("expected error for no extension, got nil")
	}
}

// ============================================
// Invalid data URL format
// ============================================

func TestSaveScreenshotToPath_InvalidDataURL_NoDataPrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")

	err := saveScreenshotToPath(path, "image/png;base64,iVBORw0KGgo=")
	if err == nil {
		t.Fatal("expected error for data URL without data: prefix, got nil")
	}
}

func TestSaveScreenshotToPath_InvalidDataURL_NoBase64Marker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")

	err := saveScreenshotToPath(path, "data:image/png;charset=utf-8,plaintext")
	if err == nil {
		t.Fatal("expected error for data URL without base64 marker, got nil")
	}
}

func TestSaveScreenshotToPath_InvalidDataURL_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")

	err := saveScreenshotToPath(path, "")
	if err == nil {
		t.Fatal("expected error for empty data URL, got nil")
	}
}

func TestSaveScreenshotToPath_InvalidDataURL_NoSemicolon(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")

	err := saveScreenshotToPath(path, "data:image/pngbase64,iVBORw0KGgo=")
	if err == nil {
		t.Fatal("expected error for data URL without semicolon, got nil")
	}
}

// ============================================
// Invalid base64 data
// ============================================

func TestSaveScreenshotToPath_InvalidBase64(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")

	// "!!not-valid-base64!!" is not valid base64
	err := saveScreenshotToPath(path, "data:image/png;base64,!!not-valid-base64!!")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

// ============================================
// Parent directory creation
// ============================================

func TestSaveScreenshotToPath_CreatesParentDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c", "shot.png")

	err := saveScreenshotToPath(nested, validPNGDataURL())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(nested)
	if err != nil {
		t.Fatalf("failed to read file in nested directory: %v", err)
	}
	if string(data) != "fake-png-image-bytes" {
		t.Errorf("file content = %q, want %q", string(data), "fake-png-image-bytes")
	}
}

// ============================================
// Overwrite existing file
// ============================================

func TestSaveScreenshotToPath_OverwritesExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")

	// Write an initial file with different content.
	if err := os.WriteFile(path, []byte("old-content"), 0o644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	err := saveScreenshotToPath(path, validPNGDataURL())
	if err != nil {
		t.Fatalf("unexpected error on overwrite: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read overwritten file: %v", err)
	}
	if string(data) == "old-content" {
		t.Error("file was not overwritten — still contains old content")
	}
	if string(data) != "fake-png-image-bytes" {
		t.Errorf("overwritten content = %q, want %q", string(data), "fake-png-image-bytes")
	}
}

// ============================================
// Extension case insensitivity
// ============================================

func TestSaveScreenshotToPath_UppercaseExtension(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.PNG")

	err := saveScreenshotToPath(path, validPNGDataURL())
	if err != nil {
		t.Fatalf("uppercase .PNG should be accepted: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file does not exist: %v", err)
	}
}

func TestSaveScreenshotToPath_MixedCaseExtension(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.Jpeg")

	err := saveScreenshotToPath(path, validJPEGDataURL())
	if err != nil {
		t.Fatalf("mixed-case .Jpeg should be accepted: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file does not exist: %v", err)
	}
}
