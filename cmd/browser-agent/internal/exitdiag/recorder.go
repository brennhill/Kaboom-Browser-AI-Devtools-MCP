// Purpose: Writes structured lifecycle diagnostic entries to crash log files on server exit or panic.
// Why: Preserves post-mortem evidence when the daemon exits unexpectedly for later troubleshooting.

package exitdiag

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

type Options struct {
	Version string
	Stderr  io.Writer
	Exit    func(int)
}

type Recorder struct {
	version string
	stderr  io.Writer
	exit    func(int)
}

func New(options Options) *Recorder {
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	if options.Exit == nil {
		options.Exit = os.Exit
	}
	return &Recorder{version: options.Version, stderr: options.Stderr, exit: options.Exit}
}

// Append writes a structured exit diagnostic entry to the first writable candidate.
func (r *Recorder) Append(event string, extra map[string]any) string {
	entry := map[string]any{
		"type":       "lifecycle",
		"event":      event,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"pid":        os.Getpid(),
		"version":    r.version,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
	for k, v := range extra {
		entry[k] = v
	}

	path, err := writeDiagnosticToCandidates(crashLogCandidates(), entry)
	if err != nil {
		return ""
	}
	return path
}

func crashLogCandidates() []string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, 3)
	add := func(path string) {
		if path == "" {
			return
		}
		if _, exists := seen[path]; exists {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	if p, err := state.CrashLogFile(); err == nil {
		add(p)
	}
	if p, err := state.LegacyCrashLogFile(); err == nil {
		add(p)
	}
	add(filepath.Join(os.TempDir(), "kaboom-crash.log"))
	return candidates
}

func writeDiagnosticToCandidates(candidates []string, entry map[string]any) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no crash-log candidates")
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}

	var lastErr error
	for _, path := range candidates {
		if path == "" {
			continue
		}
		// #nosec G301 -- diagnostics dir needs group read/execute for support workflows
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			lastErr = err
			continue
		}
		// #nosec G304 -- crash path is derived from local runtime state paths only
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // nosemgrep: go_filesystem_rule-fileread
		if err != nil {
			lastErr = err
			continue
		}
		if _, err := f.Write(data); err != nil {
			lastErr = err
			_ = f.Close()
			continue
		}
		if _, err := f.Write([]byte{'\n'}); err != nil {
			lastErr = err
			_ = f.Close()
			continue
		}
		if err := f.Close(); err != nil {
			lastErr = err
			continue
		}
		return path, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no writable crash-log candidates")
	}
	return "", lastErr
}

// Recover records crash details in both lifecycle logs before exiting.
func (r *Recorder) Recover(recovered any) {
	stack := make([]byte, 4096)
	stack = stack[:runtime.Stack(stack, false)]

	telemetry.AppError("daemon_panic", nil)
	fmt.Fprintln(r.stderr, "\n[Kaboom] FATAL ERROR")

	logFile, err := state.DefaultLogFile()
	if err != nil {
		logFile = filepath.Join(os.TempDir(), "kaboom.jsonl")
	}
	entry := map[string]any{
		"type":       "lifecycle",
		"event":      "crash",
		"reason":     fmt.Sprintf("%v", recovered),
		"stack":      string(stack),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
	if data, marshalErr := json.Marshal(entry); marshalErr == nil {
		_ = os.MkdirAll(filepath.Dir(logFile), 0o750)                                                          // #nosec G301 -- runtime diagnostics directory
		if file, openErr := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600); openErr == nil { // #nosec G304
			_, _ = file.Write(data)
			_, _ = file.Write([]byte{'\n'})
			_ = file.Close()
		}
	}

	if diagnosticPath := r.Append("panic", map[string]any{
		"reason": fmt.Sprintf("%v", recovered),
		"stack":  string(stack),
	}); diagnosticPath != "" {
		fmt.Fprintf(r.stderr, "[Kaboom] Crash details written to: %s\n", diagnosticPath)
	}
	r.exit(1)
}
