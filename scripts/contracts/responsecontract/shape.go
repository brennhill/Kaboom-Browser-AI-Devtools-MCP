// shape.go — Derives a machine-checkable response shape from a real MCP tool response.
//
// PURPOSE: MCP tool responses had no declared output contract (internal/mcp
// declares InputSchema and nothing for output), so a handler could drop, rename
// or retype a field and every layer stayed green. A Shape is the declarable
// half of a response: the field PATHS and their JSON TYPES, never their values.
// Values change every run; paths and types change only when someone edits a
// handler, which is exactly the event a gate should catch.
//
// Docs: docs/features/feature/quality-gates/index.md
package responsecontract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// kindDirect is a daemon-local response: the payload IS the answer.
const kindDirect = "direct"

// kindEnvelope is a browser-mediated response: an async lifecycle envelope
// (correlation_id + lifecycle_status) whose answer, when it has one, is nested
// under .result. That dual shape was folklore until it was declared here.
const kindEnvelope = "envelope"

// maxDepth bounds path expansion so a deeply nested payload cannot produce an
// unbounded contract file. Four levels reaches result.<field>.<field>.<field>.
const maxDepth = 4

// Field is one declared response field: where it lives and what type it holds.
type Field struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// Shape is the declared response contract for one tool mode.
type Shape struct {
	Kind   string  `json:"kind"`
	Fields []Field `json:"fields"`
}

// jsonType names the JSON type of a decoded value. Numbers are reported as
// "number" rather than int/float: JSON has one numeric type, and a handler that
// returned 0.0 where it used to return 0 has not changed its contract.
func jsonType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64, int, int64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

// walk appends the field at path and recurses into objects and array elements.
func walk(fields *[]Field, path string, value any, depth int) {
	*fields = append(*fields, Field{Path: path, Type: jsonType(value)})
	if depth >= maxDepth {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			walk(fields, joinPath(path, key), typed[key], depth+1)
		}
	case []any:
		// Element 0 only. Recording every element would make the contract
		// depend on how much fixture data happened to be in the buffer.
		if len(typed) > 0 {
			walk(fields, path+"[]", typed[0], depth+1)
		}
	}
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func sortedKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// shapeOfPayload derives the Shape of an already-decoded response payload.
func shapeOfPayload(payload map[string]any) Shape {
	kind := kindDirect
	_, hasCorrelation := payload["correlation_id"]
	_, hasLifecycle := payload["lifecycle_status"]
	if hasCorrelation && hasLifecycle {
		kind = kindEnvelope
	}
	fields := make([]Field, 0, len(payload))
	for _, key := range sortedKeys(payload) {
		walk(&fields, key, payload[key], 1)
	}
	sort.Slice(fields, func(a, b int) bool { return fields[a].Path < fields[b].Path })
	return Shape{Kind: kind, Fields: fields}
}

// payloadOf extracts the JSON object that follows an MCP response's prose
// summary. It takes the first line that BEGINS with '{', not the first '{'
// anywhere: analyze/draw_session's recovery hint embeds {what:'draw_history'}
// in its prose, and slicing from the first brace parsed the hint as the payload.
func payloadOf(text string) (map[string]any, error) {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var payload map[string]any
		body := strings.Join(lines[index:], "\n")
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			return nil, fmt.Errorf("response text carries a JSON line that does not decode: %w", err)
		}
		return payload, nil
	}
	return nil, fmt.Errorf("response text carries no JSON payload line")
}

// ShapeOfResponse derives the Shape of a JSON-RPC tool response.
func ShapeOfResponse(response mcp.JSONRPCResponse) (Shape, error) {
	if response.Error != nil {
		return Shape{}, fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return Shape{}, fmt.Errorf("decode tool result: %w", err)
	}
	if result.IsError {
		return Shape{}, fmt.Errorf("response is an MCP error, not a declarable shape")
	}
	if len(result.Content) == 0 {
		return Shape{}, fmt.Errorf("response carries no content blocks")
	}
	payload, err := payloadOf(result.Content[0].Text)
	if err != nil {
		return Shape{}, err
	}
	return shapeOfPayload(payload), nil
}
