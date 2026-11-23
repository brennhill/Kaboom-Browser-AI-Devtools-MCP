// Purpose: Package bridge — stdio transport framing, connection detection, and per-tool timeout assignment.
// Why: Isolates MCP transport concerns so the server core stays protocol-agnostic.
// Docs: docs/features/feature/bridge-restart/index.md

/*
Package bridge handles the transport layer between MCP clients and the Kaboom daemon.

Key types:
  - StdioFraming: indicates whether a message used line-delimited or Content-Length framing.

Key functions:
  - ReadStdioMessage: reads one MCP message supporting both framing modes.
  - IsConnectionError: detects network/DNS errors indicating the daemon is unreachable.
  - ToolCallTimeout: returns fast (10s), slow (35s), or blocking (65s) timeout based on tool category.
*/
package bridge
