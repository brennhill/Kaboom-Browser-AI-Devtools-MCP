// types.go — Core data types for the push delivery pipeline.
package push

import (
	"encoding/json"
	"time"
)

// PushEvent represents a browser-originated event to be pushed to the AI client.
type PushEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "annotations" | "screenshot" | "chat"
	Timestamp time.Time `json:"timestamp"`
	PageURL   string    `json:"page_url"`
	TabID     int       `json:"tab_id"`

	// Screenshot fields
	ScreenshotB64 string `json:"screenshot_b64,omitempty"`

	// Annotation fields
	Annotations  json.RawMessage `json:"annotations,omitempty"`
	AnnotSession string          `json:"annot_session,omitempty"`

	// Chat message field
	Message string `json:"message,omitempty"`

	// User note (screenshots/annotations)
	Note string `json:"note,omitempty"`
}

// ClientCapabilities describes what the connected MCP client supports.
type ClientCapabilities struct {
	SupportsSampling      bool   `json:"supports_sampling"`
	SupportsNotifications bool   `json:"supports_notifications"`
	ClientName            string `json:"client_name"`
}

// SamplingRequest is a JSON-RPC request to create a new AI message turn.
type SamplingRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int64          `json:"id"`
	Method  string         `json:"method"`
	Params  SamplingParams `json:"params"`
}

// SamplingParams holds the parameters for a sampling/createMessage request.
type SamplingParams struct {
	Messages       []SamplingMessage `json:"messages"`
	MaxTokens      int               `json:"maxTokens"`                // SPEC:MCP — camelCase per MCP sampling spec
	SystemPrompt   string            `json:"systemPrompt,omitempty"`   // SPEC:MCP
	IncludeContext string            `json:"includeContext,omitempty"` // SPEC:MCP
}

// SamplingMessage is a single message in a sampling request.
type SamplingMessage struct {
	Role    string          `json:"role"`    // SPEC:MCP
	Content SamplingContent `json:"content"` // SPEC:MCP
}

// SamplingContent is either text or image content.
type SamplingContent struct {
	Type     string `json:"type"`               // SPEC:MCP — "text" | "image"
	Text     string `json:"text,omitempty"`     // SPEC:MCP
	Data     string `json:"data,omitempty"`     // SPEC:MCP — base64
	MimeType string `json:"mimeType,omitempty"` // SPEC:MCP
}
