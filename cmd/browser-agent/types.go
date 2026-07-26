// Purpose: Re-exports core MCP wire types as package-level aliases.
// Why: Allows cmd/browser-agent code to reference MCP types by short names without direct internal/mcp imports.

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// JSONRPCVersion is the JSON-RPC protocol version string re-exported from internal/mcp.
const JSONRPCVersion = mcp.JSONRPCVersion

// LogEntry represents a single log entry (alias to internal/mcp).
type LogEntry = mcp.LogEntry

// Core wire types
type JSONRPCRequest = mcp.JSONRPCRequest
type JSONRPCResponse = mcp.JSONRPCResponse
type JSONRPCError = mcp.JSONRPCError
type MCPTool = mcp.MCPTool

// MCP result types
type MCPContentBlock = mcp.MCPContentBlock
type MCPToolResult = mcp.MCPToolResult
type MCPInitializeResult = mcp.MCPInitializeResult
type MCPServerInfo = mcp.MCPServerInfo
type MCPCapabilities = mcp.MCPCapabilities
type MCPToolsCapability = mcp.MCPToolsCapability
type MCPResourcesCapability = mcp.MCPResourcesCapability
type MCPResource = mcp.MCPResource
type MCPResourcesListResult = mcp.MCPResourcesListResult
type MCPResourceContent = mcp.MCPResourceContent
type MCPResourcesReadResult = mcp.MCPResourcesReadResult
type MCPToolsListResult = mcp.MCPToolsListResult
type MCPResourceTemplatesListResult = mcp.MCPResourceTemplatesListResult
