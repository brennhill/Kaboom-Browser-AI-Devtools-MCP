// Purpose: Declares MCPContentBlock and MCPToolResult types used across all tool responses.
// Docs: docs/features/feature/query-service/index.md

package mcp

// JSONRPCVersion is the JSON-RPC protocol version string. Use this constant
// instead of the magic string "2.0" when constructing JSON-RPC responses.
const JSONRPCVersion = "2.0"

// MCPContentBlock represents a single content block in an MCP tool result.
// Supports both text (type="text") and image (type="image") content types.
// For text: Type + Text are used. For image: Type + Data + MimeType are used.
type MCPContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`     // SPEC:MCP — base64-encoded image data (type="image")
	MimeType string `json:"mimeType,omitempty"` // SPEC:MCP — MIME type for image content (e.g. "image/png", "image/jpeg")
}

// MCPToolResult represents the result of an MCP tool call.
type MCPToolResult struct {
	Content  []MCPContentBlock `json:"content"`
	IsError  bool              `json:"isError"` // SPEC:MCP
	Metadata map[string]any    `json:"metadata,omitempty"`
}

// MCPInitializeResult represents the result of an MCP initialize request.
type MCPInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"` // SPEC:MCP
	ServerInfo      MCPServerInfo   `json:"serverInfo"`      // SPEC:MCP
	Capabilities    MCPCapabilities `json:"capabilities"`
	Instructions    string          `json:"instructions,omitempty"`
}

// MCPServerInfo identifies the MCP server.
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPCapabilities declares the server's MCP capabilities.
type MCPCapabilities struct {
	Tools     MCPToolsCapability     `json:"tools"`
	Resources MCPResourcesCapability `json:"resources"`
}

// MCPToolsCapability declares tool support.
type MCPToolsCapability struct{}

// MCPResourcesCapability declares resource support.
type MCPResourcesCapability struct{}

// MCPResource describes an available resource.
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"` // SPEC:MCP
}

// MCPResourceContent represents the content of a resource.
type MCPResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"` // SPEC:MCP
	Text     string `json:"text,omitempty"`
}

// MCPResourcesListResult represents the result of a resources/list request.
type MCPResourcesListResult struct {
	Resources []MCPResource `json:"resources"`
}

// MCPResourcesReadResult represents the result of a resources/read request.
type MCPResourcesReadResult struct {
	Contents []MCPResourceContent `json:"contents"`
}

// MCPToolsListResult represents the result of a tools/list request.
type MCPToolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

// MCPResourceTemplatesListResult represents the result of a resources/templates/list request.
type MCPResourceTemplatesListResult struct {
	ResourceTemplates []any `json:"resourceTemplates"` // SPEC:MCP
}
