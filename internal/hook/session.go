// Purpose: Persists and classifies bounded per-agent hook session activity.
// Why: Keeps durable touch history and the tracking decisions derived from it together.
// Docs: docs/features/feature/session-tracking/index.md

package hook

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
)

type touchAppendFile interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type touchFileOpener func(string) (touchAppendFile, error)

type sessionDirectoryOperations struct {
	userHomeDir func() (string, error)
	mkdirAll    func(string, os.FileMode) error
	stat        func(string) (os.FileInfo, error)
	readFile    func(string) ([]byte, error)
	writeFile   func(string, []byte, os.FileMode) error
	getwd       func() (string, error)
	getppid     func() int
	now         func() time.Time
}

func defaultSessionDirectoryOperations() sessionDirectoryOperations {
	return sessionDirectoryOperations{
		userHomeDir: os.UserHomeDir,
		mkdirAll:    os.MkdirAll,
		stat:        os.Stat,
		readFile:    os.ReadFile,
		writeFile:   statefile.Write,
		getwd:       os.Getwd,
		getppid:     os.Getppid,
		now:         time.Now,
	}
}

func openTouchFile(path string) (touchAppendFile, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

const (
	sessionBaseDir  = ".kaboom/sessions"
	touchesFile     = "touches.jsonl"
	metaFile        = "meta.json"
	staleSessionAge = 8 * time.Hour
	maxTouchLinelen = 512
	maxSummaryLen   = 100
	maxCleanupScan  = 100
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
		return sessionIDComponent(id)
	}
	if id := os.Getenv("CODEX_SESSION_ID"); id != "" {
		return sessionIDComponent(id)
	}
	ppid := os.Getppid()
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		logHookDiagnostic("session_identity_working_directory_failed")
		cwd = "working-directory-unavailable"
	}
	return hashSessionID(fmt.Sprintf("%d:%s", ppid, cwd))
}

func sessionIDComponent(id string) string {
	for _, character := range id {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') &&
			character != '-' && character != '_' {
			return hashSessionID(id)
		}
	}
	if len(id) > 16 {
		return id[:16]
	}
	return id
}

func hashSessionID(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:8])
}

// SessionDir returns the path to the current session's data directory.
// Creates the directory if it doesn't exist.
func SessionDir() (string, error) {
	return sessionDirWithOperations(defaultSessionDirectoryOperations())
}

func sessionDirWithOperations(ops sessionDirectoryOperations) (string, error) {
	home, err := ops.userHomeDir()
	if err != nil {
		return "", errors.New("session_home_directory_failed")
	}
	dir := filepath.Join(home, sessionBaseDir, SessionID())
	if err := ops.mkdirAll(dir, 0o700); err != nil {
		return "", errors.New("session_directory_create_failed")
	}
	metaPath := filepath.Join(dir, metaFile)
	if _, statErr := ops.stat(metaPath); statErr == nil {
		data, readErr := ops.readFile(metaPath)
		if readErr != nil {
			return "", errors.New("session_metadata_read_failed")
		}
		var meta sessionMeta
		if json.Unmarshal(data, &meta) != nil || meta.StartTime.IsZero() || meta.Cwd == "" || meta.Ppid <= 0 {
			return "", errors.New("session_metadata_corrupt")
		}
		return dir, nil
	} else if !os.IsNotExist(statErr) {
		return "", errors.New("session_metadata_stat_failed")
	}
	// EXPECTED_ABSENCE: the current hook session has not created metadata yet.
	cwd, cwdErr := ops.getwd()
	if cwdErr != nil {
		return "", errors.New("session_working_directory_failed")
	}
	meta := sessionMeta{StartTime: ops.now(), Cwd: cwd, Ppid: ops.getppid()}
	data, marshalErr := json.Marshal(meta)
	if marshalErr != nil {
		return "", errors.New("session_metadata_encode_failed")
	}
	if err := ops.writeFile(metaPath, data, 0o600); err != nil {
		return "", errors.New("session_metadata_write_failed")
	}
	return dir, nil
}

// AppendTouch writes a touch entry to the session log.
func AppendTouch(sessionDir string, entry TouchEntry) error {
	return appendTouchWithOpener(sessionDir, entry, openTouchFile)
}

func appendTouchWithOpener(sessionDir string, entry TouchEntry, open touchFileOpener) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := open(filepath.Join(sessionDir, touchesFile))
	if err != nil {
		return fmt.Errorf("session_touch_open_failed")
	}
	line := append(data, '\n')
	written, writeErr := f.Write(line)
	if writeErr != nil || written != len(line) {
		closeErr := f.Close()
		if written != len(line) && writeErr == nil {
			writeErr = io.ErrShortWrite
		}
		primary := fmt.Errorf("session_touch_write_failed: %w", writeErr)
		if closeErr != nil {
			return errors.Join(primary, fmt.Errorf("session_touch_close_failed"))
		}
		return primary
	}
	if err := f.Sync(); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return errors.Join(fmt.Errorf("session_touch_sync_failed"), fmt.Errorf("session_touch_close_failed"))
		}
		return fmt.Errorf("session_touch_sync_failed")
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("session_touch_close_failed")
	}
	return nil
}

// ReadTouches returns all session entries, newest first.
func ReadTouches(sessionDir string) ([]TouchEntry, error) {
	path := filepath.Join(sessionDir, touchesFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// EXPECTED_ABSENCE: a session has no touch log before its first event.
			return nil, nil
		}
		return nil, errors.New("session_touch_log_open_failed")
	}

	var entries []TouchEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, maxTouchLinelen), maxTouchLinelen)
	for scanner.Scan() {
		var e TouchEntry
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			return nil, closeTouchLogWithPrimary(f, errors.New("session_touch_log_corrupt"))
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, closeTouchLogWithPrimary(f, errors.New("session_touch_log_read_failed"))
	}
	if err := f.Close(); err != nil {
		return nil, errors.New("session_touch_log_close_failed")
	}
	// Newest first.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	return entries, nil
}

func closeTouchLogWithPrimary(file *os.File, primary error) error {
	if err := file.Close(); err != nil {
		return errors.Join(primary, errors.New("session_touch_log_close_failed"))
	}
	return primary
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
	return wasFileRead(entries, filePath)
}

func wasFileRead(entries []TouchEntry, filePath string) (bool, time.Time) {
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
	return wasFileEdited(entries, filePath, since)
}

func wasFileEdited(entries []TouchEntry, filePath string, since time.Time) (bool, time.Time) {
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
	return sessionSummary(entries)
}

func sessionSummary(entries []TouchEntry) string {
	if len(entries) == 0 {
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
		logHookDiagnostic("session_cleanup_home_directory_failed")
		return
	}
	base := filepath.Join(home, sessionBaseDir)
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			// EXPECTED_ABSENCE: no session directory exists before the first hook session.
			return
		}
		logHookDiagnostic("session_cleanup_list_failed")
		return
	}
	if len(entries) > maxCleanupScan {
		entries = entries[:maxCleanupScan]
		logHookDiagnostic("session_cleanup_scan_truncated")
	}
	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(base, entry.Name(), metaFile)
		data, err := os.ReadFile(metaPath)
		if err != nil {
			logHookDiagnostic("session_cleanup_metadata_read_failed")
			continue
		}
		var meta sessionMeta
		if json.Unmarshal(data, &meta) != nil {
			logHookDiagnostic("session_cleanup_metadata_corrupt")
			continue
		}
		if now.Sub(meta.StartTime) > staleSessionAge {
			if err := os.RemoveAll(filepath.Join(base, entry.Name())); err != nil {
				logHookDiagnostic("session_cleanup_remove_failed")
			}
		}
	}
}

func logHookDiagnostic(code string) {
	// TERMINAL_LOG_SINK: if the local stderr sink itself is unavailable, there
	// is no second local channel that can report that failure without recursion.
	_, _ = fmt.Fprintf(os.Stderr, "{\"kaboom_hook_diagnostic\":%q}\n", code)
}

type SessionTrackResult struct {
	Context string
	Action  string // "recorded", "redundant_read", "summary"
}

// FormatContext returns the additionalContext string for the hook output.
func (r *SessionTrackResult) FormatContext() string {
	return r.Context
}

// RunSessionTrack records the tool use and optionally injects session context.
// Returns nil if nothing to inject (but still records the touch).
func RunSessionTrack(input Input, sessionDir string) *SessionTrackResult {
	return runSessionTrack(input, sessionDir, ReadTouches, AppendTouch)
}

func runSessionTrack(
	input Input,
	sessionDir string,
	readTouches func(string) ([]TouchEntry, error),
	appendTouch func(string, TouchEntry) error,
) *SessionTrackResult {
	entries, err := readTouches(sessionDir)
	if err != nil {
		return &SessionTrackResult{
			Context: "[Session] Tracking unavailable (session_touch_read_failed). The latest action was not recorded.",
			Action:  "persistence_failed",
		}
	}
	fields := input.ParseToolInput()
	action := classifyAction(input.ToolName)
	filePath := fields.FilePath

	summary := buildTouchSummary(input, fields)

	entry := TouchEntry{
		Timestamp: time.Now(),
		Tool:      input.ToolName,
		File:      filePath,
		Action:    action,
		Summary:   summary,
	}

	// Check for redundant read BEFORE recording this touch.
	var result *SessionTrackResult
	if action == "read" && filePath != "" {
		result = checkRedundantRead(entries, filePath)
	}

	// Always attempt to record the touch; a failed append supersedes contextual
	// hints so the caller never mistakes an unpersisted event for session state.
	if err := appendTouch(sessionDir, entry); err != nil {
		return &SessionTrackResult{
			Context: "[Session] Tracking unavailable (session_touch_append_failed). The latest action was not recorded.",
			Action:  "persistence_failed",
		}
	}

	// For edits/writes, inject session summary.
	if result == nil && (action == "edit" || action == "write") {
		if s := sessionSummary(append(entries, entry)); s != "" {
			result = &SessionTrackResult{Context: s, Action: "summary"}
		}
	}

	return result
}

// checkRedundantRead checks if a file was already read this session.
func checkRedundantRead(entries []TouchEntry, filePath string) *SessionTrackResult {
	wasRead, readAt := wasFileRead(entries, filePath)
	if !wasRead {
		return nil
	}

	elapsed := time.Since(readAt)
	elapsedStr := formatDuration(elapsed)

	// Check if it was edited since the last read.
	wasEdited, editAt := wasFileEdited(entries, filePath, readAt)
	if wasEdited {
		editElapsed := formatDuration(time.Since(editAt))
		return &SessionTrackResult{
			Context: fmt.Sprintf("[Session] You read this file %s ago. You edited it %s ago.", elapsedStr, editElapsed),
			Action:  "redundant_read",
		}
	}

	return &SessionTrackResult{
		Context: fmt.Sprintf("[Session] You read this file %s ago. No edits since.", elapsedStr),
		Action:  "redundant_read",
	}
}

// classifyAction maps tool names to action types.
func classifyAction(toolName string) string {
	switch toolName {
	case "Read", "read_file":
		return "read"
	case "Edit", "replace_in_file", "edit_file":
		return "edit"
	case "Write", "write_file":
		return "write"
	case "Bash", "run_shell_command":
		return "bash"
	default:
		return "other"
	}
}

// buildTouchSummary extracts a short summary from the tool input.
func buildTouchSummary(input Input, fields ToolInputFields) string {
	switch classifyAction(input.ToolName) {
	case "edit":
		return truncStr(fields.NewString, maxSummaryLen)
	case "bash":
		return truncStr(fields.Command, maxSummaryLen)
	case "write":
		return truncStr(fields.Content, maxSummaryLen)
	}
	return ""
}

func truncStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d sec", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d hr", int(d.Hours()))
}
