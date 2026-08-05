// fingerprint_test.go — Verifies immutable bridge binary identity diagnostics.

package fingerprint

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCaptureReadsBinaryIdentity(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	exePath := tmp + "/kaboom-agentic-browser-test"
	content := []byte("header" + goBuildIDPrefix + "test-build-id\"tail")
	if err := os.WriteFile(exePath, content, 0o755); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	got := Capture("0.9.0", func() (string, error) { return exePath, nil })
	if got["binary_path"] != exePath || got["binary_version"] != "0.9.0" {
		t.Fatalf("Capture() identity = %#v", got)
	}
	if got["binary_build_id"] != "test-build-id" {
		t.Fatalf("binary_build_id = %v", got["binary_build_id"])
	}
	sha, ok := got["binary_sha256"].(string)
	if !ok || len(sha) != 64 {
		t.Fatalf("binary_sha256 = %v, want 64-char hex string", got["binary_sha256"])
	}
}

func TestCaptureReportsExecutableLookupFailure(t *testing.T) {
	t.Parallel()
	got := Capture("0.9.0", func() (string, error) { return "", errors.New("boom") })
	if got["binary_path"] != "" || got["binary_build_id"] != "unknown" || got["binary_sha256"] != "unknown" {
		t.Fatalf("Capture() defaults = %#v", got)
	}
	pathErr, _ := got["binary_path_error"].(string)
	if !strings.Contains(pathErr, "boom") {
		t.Fatalf("binary_path_error = %q, want contains boom", pathErr)
	}
}

func TestCaptureReportsUnreadableAndUnidentifiedBinaries(t *testing.T) {
	t.Parallel()
	missing := t.TempDir() + "/missing"
	missingResult := Capture("0.9.0", func() (string, error) { return missing, nil })
	if missingResult["binary_build_id_error"] == nil || missingResult["binary_sha256_error"] == nil {
		t.Fatalf("Capture(missing) errors = %#v", missingResult)
	}

	plain := t.TempDir() + "/plain"
	if err := os.WriteFile(plain, []byte("plain content without build id"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	plainResult := Capture("0.9.0", func() (string, error) { return plain, nil })
	if plainResult["binary_build_id_error"] == nil {
		t.Fatalf("Capture(plain) = %#v, want build ID error", plainResult)
	}
	if plainResult["binary_sha256_error"] != nil {
		t.Fatalf("Capture(plain) = %#v, unexpected SHA error", plainResult)
	}
}

func TestExtractGoBuildIDRejectsAbsentOrUnterminatedValues(t *testing.T) {
	t.Parallel()
	if got := extractGoBuildID([]byte("abc" + goBuildIDPrefix + "build-id-123\"xyz")); got != "build-id-123" {
		t.Fatalf("extractGoBuildID() = %q", got)
	}
	if got := extractGoBuildID([]byte("no build id here")); got != "" {
		t.Fatalf("extractGoBuildID(absent) = %q", got)
	}
	if got := extractGoBuildID([]byte("abc" + goBuildIDPrefix + "missing-quote")); got != "" {
		t.Fatalf("extractGoBuildID(unterminated) = %q", got)
	}
}
