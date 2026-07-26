// artifacts_security_impl.go — Implements generate(csp) and generate(sri) artifact assembly.
// Why: Groups security-focused artifact generation paths under one focused module.

package toolgenerate

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/sri"
	gen "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/generate"
)

// HandleGenerateCSP generates a Content Security Policy from captured network data.
func HandleGenerateCSP(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var arguments struct {
		Mode string `json:"mode"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &arguments); stop {
			return resp
		}
	}

	mode := arguments.Mode
	if mode == "" {
		mode = "moderate"
	}
	switch mode {
	case "strict", "moderate", "report_only":
		// valid
	default:
		return fail(req, mcp.ErrInvalidParam, "Invalid mode: "+mode, "Use strict, moderate, or report_only",
			mcp.WithParam("mode"))
	}

	networkBodies := d.GetCapture().GetNetworkBodies()
	if len(networkBodies) == 0 {
		return succeed(req, "CSP policy unavailable", map[string]any{
			"status": "unavailable", "mode": mode, "policy": "",
			"reason": "No network requests captured yet. CSP generation requires observing network traffic to identify resource origins.",
			"hint":   "Navigate the tracked page to load resources (scripts, stylesheets, images, fonts), then call generate(csp) again.",
		})
	}

	directives := gen.BuildCSPDirectives(networkBodies)
	policy := gen.BuildCSPPolicyString(directives)

	return succeed(req, "CSP policy generated", map[string]any{
		"status": "ok", "mode": mode, "policy": policy,
		"directives": directives, "origins_observed": len(networkBodies),
	})
}

// HandleGenerateSRI generates Subresource Integrity hashes for third-party scripts/styles.
func HandleGenerateSRI(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	cap := d.GetCapture()
	networkBodies := cap.GetNetworkBodies()
	if len(networkBodies) == 0 {
		return succeed(req, "SRI unavailable", map[string]any{
			"status": "unavailable",
			"hint":   "Navigate pages to capture network traffic first.",
		})
	}

	_, _, tabURL := cap.GetTrackingStatus()
	pageURLs := []string{tabURL}
	result, err := sri.HandleGenerate(args, networkBodies, pageURLs)
	if err != nil {
		return fail(req, mcp.ErrInvalidParam, "SRI generation failed: "+err.Error(), "Fix parameters and call again")
	}

	return succeed(req, "SRI hashes generated", result)
}
