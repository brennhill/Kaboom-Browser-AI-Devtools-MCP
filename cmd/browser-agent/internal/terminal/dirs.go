// dirs.go — Directory browsing for the terminal's root-folder picker.
// Why: A PTY's working directory is fixed at spawn, so choosing it is a
// first-class action — but the browser cannot resolve an absolute path on its
// own. `webkitdirectory` and showDirectoryPicker() both withhold it by design,
// so the daemon, which already runs shells in these directories, lists them.
// Docs: docs/features/feature/terminal/index.md

package terminal

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxDirEntries caps one listing. A home directory with thousands of children
// would otherwise build a picker nobody can use and a response nobody reads.
const MaxDirEntries = 500

// DirEntry is one selectable directory.
type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// DirListing is the response for /terminal/dirs.
type DirListing struct {
	Path      string     `json:"path"`
	Parent    string     `json:"parent"`
	Entries   []DirEntry `json:"entries"`
	Truncated bool       `json:"truncated"`
}

// resolveDirRequest turns the requested path into an absolute one.
// An empty path means the user's home directory, which is where a project
// almost always lives and is the least surprising place to start browsing.
func resolveDirRequest(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "~" || strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		if raw == "" || raw == "~" {
			return filepath.Clean(home), true
		}
		return filepath.Clean(filepath.Join(home, raw[2:])), true
	}
	if !filepath.IsAbs(raw) {
		return "", false
	}
	return filepath.Clean(raw), true
}

// parentOf returns the directory above dir, or "" at the filesystem root.
func parentOf(dir string) string {
	parent := filepath.Dir(dir)
	if parent == dir {
		return ""
	}
	return parent
}

// HandleTerminalDirs lists the sub-directories of a path so the side panel can
// offer a folder picker.
//
// Directories only: a file cannot be a working directory, so offering one would
// only produce a spawn failure later. Dot-directories are hidden — they clutter
// the list and are almost never a project root.
func HandleTerminalDirs(w http.ResponseWriter, r *http.Request, deps Deps) {
	if r.Method != "GET" {
		deps.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	dir, ok := resolveDirRequest(r.URL.Query().Get("path"))
	if !ok {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_path",
			"message": "path must be absolute or start with ~",
		})
		return
	}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			deps.JSONResponse(w, http.StatusNotFound, map[string]string{"error": "not_found", "path": dir})
			return
		}
		deps.JSONResponse(w, http.StatusForbidden, map[string]string{"error": "stat_failed", "path": dir})
		return
	}
	if !info.IsDir() {
		deps.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "not_a_directory", "path": dir})
		return
	}

	names, err := os.ReadDir(dir)
	if err != nil {
		// Unreadable is a normal outcome (permissions), not a server fault — the
		// picker shows the path with no children rather than an error page.
		deps.JSONResponse(w, http.StatusForbidden, map[string]string{"error": "read_failed", "path": dir})
		return
	}

	entries := make([]DirEntry, 0, len(names))
	for _, name := range names {
		if !name.IsDir() || strings.HasPrefix(name.Name(), ".") {
			continue
		}
		entries = append(entries, DirEntry{Name: name.Name(), Path: filepath.Join(dir, name.Name())})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	truncated := false
	if len(entries) > MaxDirEntries {
		entries = entries[:MaxDirEntries]
		truncated = true
	}

	deps.JSONResponse(w, http.StatusOK, DirListing{
		Path:      dir,
		Parent:    parentOf(dir),
		Entries:   entries,
		Truncated: truncated,
	})
}
