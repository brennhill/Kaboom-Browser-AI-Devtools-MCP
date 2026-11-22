// dirs_test.go — Directory browsing for the terminal root-folder picker.
package directorybrowser

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func listDirs(t *testing.T, query string) (int, DirListing) {
	t.Helper()
	req := httptest.NewRequest("GET", "/terminal/dirs"+query, nil)
	rec := httptest.NewRecorder()
	Handle(rec, req, respondJSON)

	var listing DirListing
	if err := json.Unmarshal(rec.Body.Bytes(), &listing); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	return rec.Code, listing
}

// A tree with directories, a file, and a hidden directory.
func makeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"beta", "alpha", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return root
}

func TestDirs_ListsOnlyDirectoriesSorted(t *testing.T) {
	root := makeTree(t)

	status, listing := listDirs(t, "?path="+root)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	// Files cannot be a working directory, so offering them would only produce a
	// spawn failure later. Sorted because an arbitrary order makes a picker
	// jump around between refreshes.
	var names []string
	for _, entry := range listing.Entries {
		names = append(names, entry.Name)
	}
	want := []string{"alpha", "beta"}
	if len(names) != len(want) {
		t.Fatalf("entries = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("entries = %v, want %v", names, want)
		}
	}
}

func TestDirs_EntriesCarryAbsolutePaths(t *testing.T) {
	root := makeTree(t)

	_, listing := listDirs(t, "?path="+root)

	// The picker sends the chosen path straight back as the PTY's cwd, so a
	// relative name would be resolved against the daemon's cwd, not the parent
	// the user was looking at.
	for _, entry := range listing.Entries {
		if !filepath.IsAbs(entry.Path) {
			t.Fatalf("entry %q has non-absolute path %q", entry.Name, entry.Path)
		}
		if filepath.Dir(entry.Path) != filepath.Clean(root) {
			t.Fatalf("entry path %q is not under %q", entry.Path, root)
		}
	}
}

func TestDirs_HidesDotDirectories(t *testing.T) {
	root := makeTree(t)

	_, listing := listDirs(t, "?path="+root)

	for _, entry := range listing.Entries {
		if entry.Name == ".hidden" {
			t.Fatal("dot-directories clutter the picker and are almost never a project root")
		}
	}
}

func TestDirs_ReportsParentForUpwardNavigation(t *testing.T) {
	root := makeTree(t)
	child := filepath.Join(root, "alpha")

	_, listing := listDirs(t, "?path="+child)

	if listing.Parent != filepath.Clean(root) {
		t.Fatalf("parent = %q, want %q", listing.Parent, filepath.Clean(root))
	}
}

func TestDirs_RootHasNoParent(t *testing.T) {
	// Without this the picker offers an "up" step that goes nowhere.
	_, listing := listDirs(t, "?path=/")

	if listing.Parent != "" {
		t.Fatalf("parent = %q, want empty at the filesystem root", listing.Parent)
	}
}

func TestDirs_DefaultsToHomeDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory in this environment")
	}

	_, listing := listDirs(t, "")

	if listing.Path != filepath.Clean(home) {
		t.Fatalf("path = %q, want home %q", listing.Path, home)
	}
}

func TestDirs_ExpandsTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory in this environment")
	}

	// The root folder field accepts what a user would type, and "~/dev" is what
	// they type. Passing it through unexpanded creates a literal "~" directory.
	_, listing := listDirs(t, "?path=~")

	if listing.Path != filepath.Clean(home) {
		t.Fatalf("path = %q, want home %q", listing.Path, home)
	}
}

func TestResolveDirRequest_ExpandsTildeSubdir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory in this environment")
	}

	// "~/dev/project" is what a user types; expanding only a bare "~" would leave
	// the sub-path resolving against the daemon's cwd instead of the user's home.
	got, ok := resolveDirRequest("~/dev/project")
	if !ok {
		t.Fatal("~/dev/project should resolve")
	}
	want := filepath.Join(home, "dev", "project")
	if got != want {
		t.Fatalf("resolveDirRequest(~/dev/project) = %q, want %q", got, want)
	}
}

func TestDirs_UnreadableDirectoryIsForbidden(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory read bits do not gate listing on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory read permissions")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// Restore perms so t.TempDir's cleanup can remove the tree.
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	// A directory that exists but cannot be read is a permission fault, not a
	// missing path — the picker shows an error rather than an empty listing.
	status, _ := listDirs(t, "?path="+locked)

	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for an unreadable directory", status)
	}
}

func TestDirs_MissingDirectoryIsNotFound(t *testing.T) {
	status, _ := listDirs(t, "?path="+filepath.Join(t.TempDir(), "nope"))

	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
}

func TestDirs_FilePathIsRejected(t *testing.T) {
	root := makeTree(t)

	status, _ := listDirs(t, "?path="+filepath.Join(root, "notes.txt"))

	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestDirs_RelativePathIsRejected(t *testing.T) {
	// A relative path would resolve against the daemon's cwd, which the user
	// cannot see and did not choose.
	status, _ := listDirs(t, "?path=some/relative/dir")

	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestDirs_RejectsNonGET(t *testing.T) {
	req := httptest.NewRequest("POST", "/terminal/dirs", nil)
	rec := httptest.NewRecorder()

	Handle(rec, req, respondJSON)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestDirs_TruncatesHugeDirectories(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < MaxDirEntries+10; i++ {
		if err := os.Mkdir(filepath.Join(root, "d"+string(rune('a'+i%26))+string(rune('a'+i/26))), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	_, listing := listDirs(t, "?path="+root)

	if len(listing.Entries) != MaxDirEntries {
		t.Fatalf("entries = %d, want %d", len(listing.Entries), MaxDirEntries)
	}
	// Silently returning a partial list reads as "this is everything".
	if !listing.Truncated {
		t.Fatal("a truncated listing must say so")
	}
}
