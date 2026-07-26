// isolation.go -- Configures stdout/stderr isolation in bridge mode so MCP JSON-RPC framing cannot be corrupted by diagnostic output.
// Why: Duplicates the original stdout for MCP transport and redirects os.Stdout/Stderr to a wrapper log file.
// Docs: docs/features/feature/bridge-restart/index.md

// Package stdioisolate owns the process-level stdio surgery the bridge needs
// before it can speak MCP: one duplicated fd reserved for JSON-RPC framing, and
// os.Stdout/os.Stderr redirected into a wrapper log so no diagnostic write can
// ever land in the protocol stream. It deliberately knows nothing about the
// bridge's daemon supervision or transport loop — the host passes in the two
// logging seams it needs, which keeps the dependency arrow one-way.
package stdioisolate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// WrapperLogFileName is the name of the bridge wrapper log file.
const WrapperLogFileName = "bridge-wrapper.log"

var (
	setupMu       sync.Mutex
	configured    bool
	mcpTransport  atomic.Pointer[os.File] // set once during setup, read on every MCP write
	wrapperLogOut *os.File
)

// ActiveMCPTransportWriter returns the file used for MCP JSON-RPC transport.
// In normal mode this is os.Stdout; in bridge isolation mode it's a dedicated
// duplicate of the original stdout pipe.
// Lock-free on the read path: uses atomic.Pointer since the value is set once at setup.
func ActiveMCPTransportWriter() *os.File {
	if f := mcpTransport.Load(); f != nil {
		return f
	}
	return os.Stdout
}

// Ensure configures bridge mode so stdout/stderr noise cannot corrupt MCP
// JSON-RPC framing on stdout. setStderrSink and stderrf are the host's
// diagnostic seams: once the wrapper log is open the host is told to route its
// stderr there, because the process stderr has just been redirected away.
func Ensure(logFileHint string, setStderrSink func(w io.Writer), stderrf func(format string, args ...any)) error {
	setupMu.Lock()
	defer setupMu.Unlock()
	if configured {
		return nil
	}

	transport, err := duplicateStdoutForTransport(os.Stdout)
	if err != nil {
		return fmt.Errorf("duplicate transport stdout: %w", err)
	}

	wrapperLogPath := ResolveWrapperLogPath(logFileHint)
	// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
	if mkErr := os.MkdirAll(filepath.Dir(wrapperLogPath), 0o750); mkErr != nil {
		_ = transport.Close()
		return fmt.Errorf("create bridge log directory: %w", mkErr)
	}
	// #nosec G304 -- path resolved from runtime state directory or temp fallback
	logOut, openErr := os.OpenFile(wrapperLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // nosemgrep: go_filesystem_rule-fileread -- runtime log sink
	if openErr != nil {
		_ = transport.Close()
		return fmt.Errorf("open bridge log file: %w", openErr)
	}

	if redirErr := redirectProcessStdStreams(logOut); redirErr != nil {
		_ = transport.Close()
		_ = logOut.Close()
		return fmt.Errorf("redirect std streams: %w", redirErr)
	}

	mcpTransport.Store(transport)
	wrapperLogOut = logOut
	configured = true
	setStderrSink(logOut)
	stderrf("[kaboom-bridge] stdio isolation enabled; wrapper logs -> %s\n", wrapperLogPath)

	return nil
}

// ResolveWrapperLogPath picks where wrapper diagnostics land: the runtime state
// dir when it resolves, otherwise beside the caller's log file, otherwise temp.
func ResolveWrapperLogPath(logFileHint string) string {
	if path, err := state.InRoot("logs", WrapperLogFileName); err == nil {
		return path
	}
	if strings.TrimSpace(logFileHint) != "" {
		baseDir := filepath.Dir(logFileHint)
		return filepath.Join(baseDir, WrapperLogFileName)
	}
	return filepath.Join(os.TempDir(), "kaboom", "logs", WrapperLogFileName)
}
