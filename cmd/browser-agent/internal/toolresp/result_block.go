// result_block.go — Reading and rewriting the JSON payload inside an MCP tool result.
//
// Extension command results arrive as a text content block with a human prefix followed by
// a JSON object ("list_interactive results\n{...}"). Handlers that annotate that payload —
// element-index generation stamps, truncation markers, accessibility handles — all need the
// same three moves: find the first block that parses, edit the decoded map, put it back
// without disturbing the prefix. Keeping those moves here means a handler that annotates a
// payload cannot accidentally drop the prefix or re-encode a different content block.

package toolresp

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ResultBlock is one decoded JSON payload inside a tool result, with everything needed to
// write it back into the exact content block it came from.
type ResultBlock struct {
	result       mcp.MCPToolResult
	contentIndex int
	prefix       string
	// Data is the decoded payload. Mutate it, then call Replace.
	Data map[string]any
}

// DecodeFirstJSONBlock returns the first content block of resp whose text contains a JSON
// object. An error result yields no block: annotating a failure payload would dress a
// refusal up as a result.
func DecodeFirstJSONBlock(resp mcp.JSONRPCResponse) (ResultBlock, bool) {
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return ResultBlock{}, false
	}
	for i, content := range result.Content {
		jsonStart := strings.Index(content.Text, "{")
		if jsonStart < 0 {
			continue
		}
		var data map[string]any
		if json.Unmarshal([]byte(content.Text[jsonStart:]), &data) == nil {
			return ResultBlock{
				result:       result,
				contentIndex: i,
				prefix:       content.Text[:jsonStart],
				Data:         data,
			}, true
		}
	}
	return ResultBlock{}, false
}

// Replace writes the block's Data back into resp, preserving the human prefix.
func (b ResultBlock) Replace(resp mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	data, err := json.Marshal(b.Data)
	if err != nil {
		return resp
	}
	b.result.Content[b.contentIndex].Text = b.prefix + string(data)
	resp.Result = mcp.SafeMarshal(b.result, string(resp.Result))
	return resp
}

// SetNestedElements writes elements back wherever the payload happens to carry them.
// Command results nest the list one or two levels deep depending on the command, and a
// writer that only handled the top level would silently discard a truncation.
func SetNestedElements(data map[string]any, elements []any) {
	if _, ok := data["elements"]; ok {
		data["elements"] = elements
		return
	}
	result, ok := data["result"].(map[string]any)
	if !ok {
		return
	}
	if _, ok := result["elements"]; ok {
		result["elements"] = elements
		return
	}
	if nested, ok := result["result"].(map[string]any); ok {
		if _, ok := nested["elements"]; ok {
			nested["elements"] = elements
		}
	}
}
