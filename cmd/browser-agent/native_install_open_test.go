// native_install_open_test.go — Tests for the install-time extension folder open.
package main

import (
	"path/filepath"
	"testing"
)

func TestFileManagerOpenCommand(t *testing.T) {
	cases := []struct {
		goos     string
		wantName string
		wantOK   bool
	}{
		{"darwin", "open", true},
		{"windows", "explorer", true},
		{"linux", "xdg-open", true},
		{"plan9", "", false},
	}
	for _, tc := range cases {
		name, args, ok := fileManagerOpenCommand(tc.goos, "/some/dir")
		if ok != tc.wantOK || name != tc.wantName {
			t.Fatalf("fileManagerOpenCommand(%q) = (%q,%v), want (%q,%v)", tc.goos, name, ok, tc.wantName, tc.wantOK)
		}
		if ok && (len(args) != 1 || args[0] != "/some/dir") {
			t.Fatalf("fileManagerOpenCommand(%q) args = %v, want [/some/dir]", tc.goos, args)
		}
	}
}

func TestExtensionAutoOpenDisabled(t *testing.T) {
	cases := []struct {
		name string
		key  string
		val  string
		want bool
	}{
		{"unset", "", "", false},
		{"no_open_1", "KABOOM_NO_OPEN", "1", true},
		{"install_no_open_true", "KABOOM_INSTALL_NO_OPEN", "true", true},
		{"no_open_0", "KABOOM_NO_OPEN", "0", false},
		{"no_open_false", "KABOOM_NO_OPEN", "false", false},
		{"no_open_empty", "KABOOM_NO_OPEN", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear both so a stray env from the runner cannot leak across cases.
			t.Setenv("KABOOM_NO_OPEN", "")
			t.Setenv("KABOOM_INSTALL_NO_OPEN", "")
			if tc.key != "" {
				t.Setenv(tc.key, tc.val)
			}
			if got := extensionAutoOpenDisabled(); got != tc.want {
				t.Fatalf("extensionAutoOpenDisabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOpenExtensionFolder_Guards(t *testing.T) {
	// Opt-out must short-circuit before any launch, even for a real directory.
	t.Setenv("KABOOM_NO_OPEN", "1")
	if openExtensionFolder(t.TempDir()) {
		t.Fatal("opt-out must prevent opening")
	}

	// A missing directory is never opened (and does not error).
	t.Setenv("KABOOM_NO_OPEN", "")
	if openExtensionFolder(filepath.Join(t.TempDir(), "absent")) {
		t.Fatal("a missing directory must not be opened")
	}
	if openExtensionFolder("") {
		t.Fatal("an empty path must not be opened")
	}
}
