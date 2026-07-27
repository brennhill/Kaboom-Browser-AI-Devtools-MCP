// debug_file.go — Writes optional append-only diagnostic traces outside MCP transports.

package diag

import (
	"fmt"
	"os"
	"sync"
	"time"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

type DebugFile struct {
	mu      sync.Mutex
	path    string
	enabled bool
}

func NewDebugFile(path string, enabled bool) *DebugFile {
	return &DebugFile{path: path, enabled: enabled && path != ""}
}

func NewDebugFileFromEnv() *DebugFile {
	if os.Getenv("KABOOM_DEBUG") == "off" {
		return NewDebugFile("", false)
	}
	if path := os.Getenv("KABOOM_MCP_DEBUG_FILE"); path != "" {
		return NewDebugFile(path, true)
	}
	path, err := statecfg.InRoot("logs", "bridge-debug.jsonl")
	if err != nil {
		return NewDebugFile("", false)
	}
	return NewDebugFile(path, true)
}

func (d *DebugFile) Printf(format string, args ...any) {
	if d == nil || !d.enabled {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	file, err := os.OpenFile(d.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = fmt.Fprintf(file, "%s "+format+"\n", append([]any{timestamp}, args...)...)
}
