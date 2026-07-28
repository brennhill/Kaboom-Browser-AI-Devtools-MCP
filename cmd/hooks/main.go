// main.go — Entry point for the kaboom-hooks binary.
// Standalone CLI for AI coding agent hooks (Claude Code, Gemini CLI, Codex).
// Can be installed independently or as part of the full Kaboom suite.

package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/hook"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "0.9.0"

const defaultDaemonPort = "7890"

var subcommands = map[string]func() int{
	"compress-output": runCompressOutput,
	"quality-gate":    runQualityGate,
	"session-track":   runSessionTrack,
	"blast-radius":    runBlastRadius,
	"decision-guard":  runDecisionGuard,
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	arg := os.Args[1]

	if arg == "--version" || arg == "-v" {
		fmt.Println(version)
		os.Exit(0)
	}
	if arg == "--help" || arg == "-h" {
		printUsage()
		os.Exit(0)
	}

	fn, ok := subcommands[arg]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", arg)
		printUsage()
		os.Exit(2)
	}
	os.Exit(fn())
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `kaboom-hooks — AI coding agent hooks (Claude Code, Gemini CLI, Codex)

Usage: kaboom-hooks <command>

Commands:
  quality-gate      Check edited files against project standards
  compress-output   Compress verbose test/build output
  session-track     Track file reads/edits, detect redundant reads
  blast-radius      Warn about downstream impact of edits
  decision-guard    Enforce locked architectural decisions

Flags:
  --version         Show version
  --help            Show this help

These hooks read PostToolUse/AfterTool JSON from stdin and write
additionalContext JSON to stdout. Auto-detects Claude/Gemini/Codex.

Claude Code (.claude/settings.json):
  "hooks": {
    "PostToolUse": [
      {"matcher": "Edit|Write", "hooks": [
        {"type": "command", "command": "kaboom-hooks quality-gate", "timeout": 10},
        {"type": "command", "command": "kaboom-hooks blast-radius", "timeout": 10},
        {"type": "command", "command": "kaboom-hooks decision-guard", "timeout": 10},
        {"type": "command", "command": "kaboom-hooks session-track", "timeout": 10}
      ]},
      {"matcher": "Read", "hooks": [
        {"type": "command", "command": "kaboom-hooks session-track", "timeout": 10}
      ]},
      {"matcher": "Bash", "hooks": [
        {"type": "command", "command": "kaboom-hooks compress-output", "timeout": 10},
        {"type": "command", "command": "kaboom-hooks session-track", "timeout": 10}
      ]}
    ]
  }

Install: curl -fsSL https://gokaboom.dev/install.sh | sh -s -- --hooks-only
Full:    curl -fsSL https://gokaboom.dev/install.sh | sh
`)
}

// runCompressOutput reads hook input from stdin, compresses Bash output,
// and writes additionalContext JSON to stdout.
func runCompressOutput() int {
	input, err := hook.ReadInput(os.Stdin)
	if err != nil {
		return 0 // Silent exit on bad input — hook protocol.
	}

	result := hook.CompressOutput(input)
	if result == nil {
		return 0
	}

	// Write output first — minimize latency for Claude Code hook response.
	if err := hook.WriteOutput(os.Stdout, result.FormatContext()); err != nil {
		return 1
	}

	// Post stats to daemon after output (best-effort, may block up to 200ms).
	port := os.Getenv("KABOOM_PORT")
	if port == "" {
		port = defaultDaemonPort
	}
	postTokenSavings(port, result)

	return 0
}

// runQualityGate reads hook input from stdin, checks the file against
// project standards, and writes additionalContext JSON to stdout.
func runQualityGate() int {
	input, err := hook.ReadInput(os.Stdin)
	if err != nil {
		return 0 // Silent exit on bad input — hook protocol.
	}

	result := hook.RunQualityGate(input)
	if result == nil {
		return 0
	}

	if err := hook.WriteOutput(os.Stdout, result.FormatContext()); err != nil {
		return 1
	}
	return 0
}

// runSessionTrack records the tool use and injects session context.
func runSessionTrack() int {
	input, err := hook.ReadInput(os.Stdin)
	if err != nil {
		return 0
	}

	sessionDir, err := hook.SessionDir()
	if err != nil {
		return 0 // Graceful degradation — can't track without session dir.
	}

	// Clean stale sessions in the background.
	go hook.CleanStaleSessions()

	result := hook.RunSessionTrack(input, sessionDir)
	if result == nil {
		return 0
	}

	if err := hook.WriteOutput(os.Stdout, result.FormatContext()); err != nil {
		return 1
	}
	return 0
}

// runBlastRadius checks for downstream impact of file edits.
func runBlastRadius() int {
	input, err := hook.ReadInput(os.Stdin)
	if err != nil {
		return 0
	}

	fields := input.ParseToolInput()
	projectRoot := hook.FindProjectRoot(fields.FilePath)
	if projectRoot == "" {
		return 0
	}

	sessionDir, _ := hook.SessionDir()

	result := hook.RunBlastRadius(input, projectRoot, sessionDir)
	if result == nil {
		return 0
	}

	if err := hook.WriteOutput(os.Stdout, result.FormatContext()); err != nil {
		return 1
	}
	return 0
}

// runDecisionGuard enforces locked architectural decisions.
func runDecisionGuard() int {
	input, err := hook.ReadInput(os.Stdin)
	if err != nil {
		return 0
	}

	fields := input.ParseToolInput()
	projectRoot := hook.FindProjectRoot(fields.FilePath)
	if projectRoot == "" {
		return 0
	}

	result := hook.RunDecisionGuard(input, projectRoot)
	if result == nil {
		return 0
	}

	if err := hook.WriteOutput(os.Stdout, result.FormatContext()); err != nil {
		return 1
	}
	return 0
}

// postTokenSavings sends compression stats to the daemon (best-effort).
// Uses a short timeout to avoid delaying process exit.
func postTokenSavings(port string, r *hook.CompressResult) {
	body := fmt.Sprintf(`{"category":%q,"tokens_before":%d,"tokens_after":%d}`,
		r.Category, r.TokensBefore, r.TokensAfter)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	u := fmt.Sprintf("http://127.0.0.1:%s/api/token-savings", port)
	resp, err := client.Post(u, "application/json", strings.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}
