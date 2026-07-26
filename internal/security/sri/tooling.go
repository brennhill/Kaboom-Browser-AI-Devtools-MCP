// tooling.go — MCP generate_sri parameter parsing and dispatch.
// Purpose: Parses MCP tool parameters and dispatches SRI generation with formatted output.
// Why: Separates MCP tool integration from SRI hash computation and generation logic.
package sri

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// HandleGenerate parses params and returns SRI generation output.
//
// Failure semantics:
// - Invalid JSON params return an explicit error and no partial output.
func HandleGenerate(params json.RawMessage, bodies []capture.NetworkBody, pageURLs []string) (any, error) {
	var toolParams Params
	if len(params) > 0 {
		if err := json.Unmarshal(params, &toolParams); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	gen := NewGenerator()
	result := gen.Generate(bodies, pageURLs, toolParams)
	return result, nil
}
