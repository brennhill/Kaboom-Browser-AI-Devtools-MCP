// Purpose: Tests for OS-level upload automation and file dialog handling.
// Docs: docs/features/feature/file-upload/index.md

// os_automation_test.go — Tests for Chrome PID auto-detection and OS automation.
package osauto

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// testSecurity returns a permissive Security config for tests.
func testSecurity(t *testing.T) *uploadsec.Security {
	t.Helper()
	return uploadsec.NewSecurity("/", nil)
}

// createTestFile creates a temporary file with the given name and content.
func createTestFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return path
}

// ============================================
// DetectBrowserPID tests
// ============================================

func TestOSAutomation_DetectBrowserPID_ReturnsValidPID(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only integration test")
	}
	pid, err := DetectBrowserPID()
	if err != nil {
		t.Skipf("Chrome not running: %v", err)
	}
	if pid <= 0 {
		t.Errorf("DetectBrowserPID() returned invalid PID: %d", pid)
	}
}

func TestOSAutomation_DetectBrowserPID_ErrorContainsPgrep_macOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only test")
	}
	pid, err := DetectBrowserPID()
	if err != nil {
		if !strings.Contains(err.Error(), "pgrep") {
			t.Errorf("macOS error should mention pgrep, got: %s", err.Error())
		}
		if !strings.Contains(err.Error(), "Google Chrome") {
			t.Errorf("macOS error should mention 'Google Chrome', got: %s", err.Error())
		}
	} else if pid <= 0 {
		t.Errorf("no error but invalid PID: %d", pid)
	}
}

func TestOSAutomation_DetectBrowserPID_ErrorContainsChrome_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}
	pid, err := DetectBrowserPID()
	if err != nil {
		if !strings.Contains(err.Error(), "chrome") {
			t.Errorf("Linux error should mention chrome, got: %s", err.Error())
		}
		if !strings.Contains(err.Error(), "google-chrome") {
			t.Errorf("Linux error should mention google-chrome, got: %s", err.Error())
		}
	} else if pid <= 0 {
		t.Errorf("no error but invalid PID: %d", pid)
	}
}

// ============================================
// HandleOSAutomation with BrowserPID=0
// ============================================

func TestOSAutomation_BrowserPID_Zero_CallsDetection(t *testing.T) {
	sec := testSecurity(t)
	testFile := createTestFile(t, "pid-test.txt", "test content for PID detection")

	resp := HandleOSAutomation(upload.OSAutomationInjectRequest{
		FilePath:   testFile,
		BrowserPID: 0,
	}, sec)

	if resp.Error != "" && strings.Contains(resp.Error, "Missing or invalid browser_pid") {
		t.Errorf("BrowserPID=0 should trigger auto-detection, not the old error. Got: %s", resp.Error)
	}
	if !resp.Success && resp.Error != "" {
		t.Logf("PID detection result (expected): %s", resp.Error)
	}
}

func TestOSAutomation_BrowserPID_Positive_SkipsDetection(t *testing.T) {
	sec := testSecurity(t)
	testFile := createTestFile(t, "pid-positive.txt", "test content")

	resp := HandleOSAutomation(upload.OSAutomationInjectRequest{
		FilePath:   testFile,
		BrowserPID: 99999,
	}, sec)

	if resp.Error != "" && strings.Contains(resp.Error, "Cannot detect Chrome") {
		t.Errorf("Positive BrowserPID should skip detection. Got: %s", resp.Error)
	}
}

// ============================================
// Enhanced error messages
// ============================================

func TestOSAutomation_MacOS_ErrorIncludesTermProgram(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only test")
	}

	prev := os.Getenv("TERM_PROGRAM")
	os.Setenv("TERM_PROGRAM", "iTerm2")
	t.Cleanup(func() {
		if prev != "" {
			os.Setenv("TERM_PROGRAM", prev)
		} else {
			os.Unsetenv("TERM_PROGRAM")
		}
	})

	testFile := createTestFile(t, "term-test.txt", "test content")
	resp := executeMacOSAutomation(upload.OSAutomationInjectRequest{
		FilePath:   testFile,
		BrowserPID: 1,
	}, time.Now())

	if !resp.Success && strings.Contains(resp.Error, "AppleScript failed") {
		if !strings.Contains(resp.Error, "iTerm2") {
			t.Errorf("macOS error should include TERM_PROGRAM value 'iTerm2', got: %s", resp.Error)
		}
	}
}

func TestOSAutomation_Linux_ErrorIncludesXdotoolInstall(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}
	resp := executeLinuxAutomation(upload.OSAutomationInjectRequest{
		FilePath:   "/tmp/test.txt",
		BrowserPID: 1,
	}, time.Now())

	if !resp.Success {
		if strings.Contains(resp.Error, "xdotool not found") {
			if !strings.Contains(resp.Error, "sudo apt install xdotool") {
				t.Errorf("xdotool missing error should include install instructions, got: %s", resp.Error)
			}
			if !strings.Contains(resp.Error, "sudo dnf install xdotool") {
				t.Errorf("xdotool missing error should include dnf install, got: %s", resp.Error)
			}
		}
	}
}
