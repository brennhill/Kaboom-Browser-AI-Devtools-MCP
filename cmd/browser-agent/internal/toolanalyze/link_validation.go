// link_validation.go — Server-side link validation for CORS-blocked URLs.
// Why: Separates synchronous server validation from asynchronous browser link checks.
// Docs: docs/features/feature/analyze-tool/index.md

package toolanalyze

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	az "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/analyze"
)

// HandleLinkValidation handles analyze(what="link_validation") — server-side HTTP checks.
func HandleLinkValidation(req mcp.JSONRPCRequest, args json.RawMessage, version string) mcp.JSONRPCResponse {
	var params az.LinkValidationParams
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
		}
	}

	if len(params.URLs) == 0 {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'urls' is missing or empty", "Provide an array of URLs to validate")
	}

	timeoutMS := az.ClampInt(params.TimeoutMS, 15000, 1000, 60000)
	maxWorkers := az.ClampInt(params.MaxWorkers, 20, 1, 100)

	validURLs := az.FilterHTTPURLs(params.URLs)
	if len(validURLs) == 0 {
		return mcp.Fail(req, mcp.ErrInvalidParam, "No valid HTTP/HTTPS URLs provided", "URLs must start with http:// or https://", mcp.WithParam("urls"))
	}
	if len(validURLs) > az.MaxLinkValidationURLs {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			fmt.Sprintf("Too many URLs: got %d, max %d", len(validURLs), az.MaxLinkValidationURLs),
			fmt.Sprintf("Reduce URLs to %d or fewer and retry", az.MaxLinkValidationURLs),
			mcp.WithParam("urls"))
	}

	results := az.ValidateLinksServerSide(validURLs, timeoutMS, maxWorkers, version)
	return mcp.Succeed(req, "Server-side link validation completed", map[string]any{
		"status":  "completed",
		"total":   len(results),
		"results": results,
	})
}
