// Purpose: Package mcp — MCP protocol types, structured errors, validation, and response helpers.
// Why: Gives all tools consistent protocol handling and machine-readable error semantics.
// Docs: docs/features/feature/query-service/index.md

/*
Package mcp defines the core MCP (Model Context Protocol) types, structured error
handling, parameter validation, and response formatting used across all five tools.

Key types:
  - JSONRPCRequest: incoming JSON-RPC 2.0 request with client ID isolation.
  - MCPToolResult: tool call result with content blocks and error flag.
  - MCPTool: tool schema definition (name, description, input schema).
  - ToolError: structured error with code, message, retry hint, and diagnostic context.

Key functions:
  - SafeMarshal: defensive JSON marshaling with fallback.
  - getJSONFieldNames: extracts known JSON field names from struct tags for validation.
  - NewToolError: creates a structured error with diagnostic hints.

File layout:
  - protocol.go: JSON-RPC 2.0 request/response types and the MCPTool schema struct.
  - types.go: MCP content blocks, tool results, and resource/initialize payloads.
  - errors.go: error codes, StructuredError, and the With* option functions.
  - validation.go: reflection-based unknown-parameter detection.
  - deps.go: provider interfaces tools require from the host server.
  - response.go: JSON marshal helpers plus Succeed/SucceedText/Fail/ParseArgs.
  - response_content.go: image/warning content-block utilities.
  - response_clamp.go: payload-size clamping with JSON-aware boundary truncation.
*/
package mcp
