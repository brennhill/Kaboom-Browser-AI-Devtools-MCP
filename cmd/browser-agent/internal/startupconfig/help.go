// help.go — Owns the canonical process and direct-tool command help text.

package startupconfig

// HelpText documents the supported process modes and direct tool interface.
const HelpText = `
Kaboom - Agentic Browser Devtools - rapid e2e web development

Usage: kaboom [options]

Options:
  --port <number>        Port to listen on (default: 7890)
  --log-file <path>      Path to log file (default: in runtime state dir)
  --state-dir <path>     Directory for runtime state (default: OS app state dir)
  --parallel             Opt-in parallel mode (isolated state dir, no takeover)
  --max-entries <number> Max log entries before rotation (default: 1000)
  --stop                 Stop the running server on the specified port
  --force                Force kill ALL running kaboom daemons (used during install)
  --instances            List every Kaboom instance registered on this machine
  --reap                 Reclaim dead, wedged, and over-cap instances (--dry-run to preview)
  --api-key <key>        Require API key auth (optional)
  --connect              Connect to existing server (multi-client mode)
  --client-id <id>       Override client ID (default: derived from CWD)
  --doctor               Run setup diagnostics
  --fastpath-min-samples Minimum telemetry samples required for threshold check (default: 50)
  --fastpath-max-failure-ratio Maximum allowed fast-path failure ratio for --doctor (disabled by default)
  --version              Show version
  --help                 Show this help message

Kaboom always runs in MCP mode: the HTTP server starts in the background
(for the browser extension) and MCP protocol runs over stdio (for Claude Code, Cursor, etc.).
The server persists until explicitly stopped with --stop or killed.

Examples:
  kaboom                              # Start server (daemon mode)
  kaboom --stop                       # Stop server on default port
  kaboom --stop --port 8080           # Stop server on specific port
  kaboom --force                      # Force kill all daemons (for clean upgrade)
  kaboom --instances                  # Show every daemon and bridge on this machine
  kaboom --reap --dry-run             # Preview what would be reclaimed
  kaboom --api-key s3cret             # Start with API key auth
  kaboom --connect --port 7890        # Connect to existing server
  kaboom --doctor                     # Verify setup before running
  kaboom --port 8080 --max-entries 500

CLI Mode (direct tool access):
  kaboom observe errors --limit 50
  kaboom analyze dom --selector "button"
  kaboom observe logs --min-level warn
  kaboom generate har --save-to out.har
  kaboom configure health
  kaboom interact click --selector "#btn"

  CLI flags: --port, --format (human|json|csv), --timeout (ms)
  Env vars: KABOOM_PORT, KABOOM_FORMAT, KABOOM_STATE_DIR

MCP Configuration:
  kaboom-agentic-browser --install     Auto-install to all detected AI clients
  kaboom-agentic-browser --config      Show configuration and detected clients
  kaboom-agentic-browser --doctor      Run diagnostics on installed configs

  Supported clients: Codex, Claude Code, Claude Desktop, Cursor, Windsurf, VS Code
`
