// Purpose: Re-exports core MCP wire types as package-level aliases.
// Why: Allows cmd/browser-agent code to reference MCP types by short names without direct internal/mcp imports.

package main

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

const (
	mcpServerName            = identity.MCPServerName
	mcpProtocolVersionLatest = "2025-06-18"
	mcpProtocolVersionLegacy = "2024-11-05"
)

var legacyMCPServerNames = append([]string(nil), identity.LegacyMCPServerNames...)
var randomInt63 = toolresp.RandomInt63
var NewToolCallLimiter = toolresp.NewToolCallLimiter

type ToolCallLimiter = toolresp.ToolCallLimiter

type AnnotationRect = annotation.Rect
type Annotation = annotation.Annotation
type AnnotationDetail = annotation.Detail
type AnnotationSession = annotation.Session
type NamedAnnotationSession = annotation.NamedSession
type AnnotationStore = annotation.Store

func NewAnnotationStore(detailTTL time.Duration) *annotation.Store {
	return annotation.NewStore(detailTTL)
}

func negotiateProtocolVersion(rawParams json.RawMessage) string {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"` // SPEC:MCP
	}
	if len(rawParams) > 0 {
		_ = json.Unmarshal(rawParams, &params)
	}
	switch params.ProtocolVersion {
	case mcpProtocolVersionLatest, mcpProtocolVersionLegacy:
		return params.ProtocolVersion
	default:
		return mcpProtocolVersionLatest
	}
}

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
