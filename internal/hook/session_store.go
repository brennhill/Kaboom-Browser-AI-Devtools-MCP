// session_store.go — Shared session state for all hooks.
// Manages append-only JSONL session log in ~/.kaboom/sessions/<session-id>/.
// All hooks import this package for cross-hook session awareness.

package hook

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	sessionBaseDir  = ".kaboom/sessions"
	touchesFile     = "touches.jsonl"
	metaFile        = "meta.json"
	staleSessionAge = 8 * time.Hour
	maxTouchLinelen = 512
	maxSummaryLen   = 100
)

// TouchEntry represents a single tool use recorded in the session log.
type TouchEntry struct {
	Timestamp time.Time `json:"t"`
	Tool      string    `json:"tool"`
	File      string    `json:"file,omitempty"`
	Action    string    `json:"action"`
	Summary   string    `json:"summary,omitempty"`
}

// sessionMeta holds session metadata.
type sessionMeta struct {
	StartTime time.Time `json:"start_time"`
	Cwd       string    `json:"cwd"`
	Ppid      int       `json:"ppid"`
}

// SessionID derives a stable session identifier.
// Prefers agent-provided session IDs, falls back to (ppid, cwd) hash.
func SessionID() string {
	if id := os.Getenv("GEMINI_SESSION_ID"); id != "" {
		return truncateID(id)
	}
	if id := os.Getenv("CODEX_SESSION_ID"); id != "" {
		return truncateID(id)
	}
	ppid := os.Getppid()
	cwd, _ := os.Getwd()
	h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", ppid, cwd)))
	return hex.EncodeToString(h[:8])
}

func truncateID(id string) string {
	if len(id) > 16 {
		return id[:16]
	}
	return id
}

// SessionDir returns the path to the current session's data directory.
// Creates the directory if it doesn't exist.
func SessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, sessionBaseDir, SessionID())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Write meta.json on first access.
	metaPath := filepath.Join(dir, metaFile)
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		cwd, _ := os.Getwd()
		meta := sessionMeta{
			StartTime: time.Now(),
			Cwd:       cwd,
			Ppid:      os.Getppid(),
		}
		data, _ := json.Marshal(meta)
		_ = os.WriteFile(metaPath, data, 0o644)
	}
	return dir, nil
}

// AppendTouch writes a touch entry to the session log.
func AppendTouch(sessionDir string, entry TouchEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(
		filepath.Join(sessionDir, touchesFile),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o644,
	)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// ReadTouches returns all session entries, newest first.
func ReadTouches(sessionDir string) ([]TouchEntry, error) {
	path := filepath.Join(sessionDir, touchesFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []TouchEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, maxTouchLinelen), maxTouchLinelen)
	for scanner.Scan() {
		var e TouchEntry
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			entries = append(entries, e)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// Newest first.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	return entries, nil
}

// FilesEdited returns file paths edited this session.
func FilesEdited(sessionDir string) []string {
	entries, err := ReadTouches(sessionDir)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var files []string
	for _, e := range entries {
		if (e.Action == "edit" || e.Action == "write") && e.File != "" && !seen[e.File] {
			seen[e.File] = true
			files = append(files, e.File)
		}
	}
	return files
}

// LastBashResult returns the most recent Bash command and its summary.
func LastBashResult(sessionDir string) (command string, summary string, found bool) {
	entries, err := ReadTouches(sessionDir)
	if err != nil {
		return "", "", false
	}
	for _, e := range entries {
		if e.Tool == "Bash" || e.Tool == "run_shell_command" {
			return e.Summary, "", true
		}
	}
	return "", "", false
}

// WasFileRead returns true if the file was already read this session, and when.
func WasFileRead(sessionDir string, filePath string) (bool, time.Time) {
	entries, err := ReadTouches(sessionDir)
	if err != nil {
		return false, time.Time{}
	}
	for _, e := range entries {
		if e.Action == "read" && e.File == filePath {
			return true, e.Timestamp
		}
	}
	return false, time.Time{}
}

// WasFileEdited returns true if the file was edited since the given time.
func WasFileEdited(sessionDir string, filePath string, since time.Time) (bool, time.Time) {
	entries, err := ReadTouches(sessionDir)
	if err != nil {
		return false, time.Time{}
	}
	for _, e := range entries {
		if (e.Action == "edit" || e.Action == "write") && e.File == filePath && e.Timestamp.After(since) {
			return true, e.Timestamp
		}
	}
	return false, time.Time{}
}

// SessionSummary returns a human-readable summary of the session so far.
func SessionSummary(sessionDir string) string {
	entries, err := ReadTouches(sessionDir)
	if err != nil || len(entries) == 0 {
		return ""
	}

	reads, edits, commands := 0, 0, 0
	var lastBash string
	var lastBashHasPass, lastBashHasFail bool

	// Entries are newest-first, but we want to count all.
	for _, e := range entries {
		switch e.Action {
		case "read":
			reads++
		case "edit", "write":
			edits++
		case "bash":
			commands++
			if lastBash == "" {
				lastBash = e.Summary
				lastBashHasPass = strings.Contains(strings.ToLower(e.Summary), "pass")
				lastBashHasFail = strings.Contains(strings.ToLower(e.Summary), "fail")
			}
		}
	}

	summary := fmt.Sprintf("[Session] %d files read, %d edited, %d commands.", reads, edits, commands)
	if lastBash != "" {
		if lastBashHasFail {
			summary += fmt.Sprintf(" Last test: FAIL (%s)", truncSummary(lastBash))
		} else if lastBashHasPass {
			summary += fmt.Sprintf(" Last test: PASS (%s)", truncSummary(lastBash))
		}
	}
	return summary
}

func truncSummary(s string) string {
	if len(s) > maxSummaryLen {
		return s[:maxSummaryLen-3] + "..."
	}
	return s
}

// CleanStaleSessions removes session directories older than staleSessionAge.
// Runs in the calling goroutine (caller should use `go` if non-blocking is desired).
func CleanStaleSessions() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	base := filepath.Join(home, sessionBaseDir)
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(base, entry.Name(), metaFile)
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta sessionMeta
		if json.Unmarshal(data, &meta) != nil {
			continue
		}
		if now.Sub(meta.StartTime) > staleSessionAge {
			_ = os.RemoveAll(filepath.Join(base, entry.Name()))
		}
	}
}
