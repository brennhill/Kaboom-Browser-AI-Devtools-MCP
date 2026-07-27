// security_mode.go — Handles configure(what="security_mode") for toggling security modes.
// Why: Isolates security mode mutation from the configure router.

package toolconfigure

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// HandleSecurityMode handles configure(what="security_mode").
func HandleSecurityMode(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	if !d.HasCapture() {
		return mcp.Fail(req, mcp.ErrNotInitialized,
			"Capture subsystem not initialized",
			"Internal error — do not retry")
	}

	var params struct {
		Mode    string `json:"mode"`
		Confirm bool   `json:"confirm"`
	}
	mcp.LenientUnmarshal(args, &params)

	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	if mode == "" {
		current, productionParity, rewrites := d.GetSecurityMode()
		return mcp.Succeed(req, "Security mode", map[string]any{
			"status":                    "ok",
			"security_mode":             current,
			"production_parity":         productionParity,
			"insecure_rewrites_applied": rewrites,
			"requires_confirmation_for_insecure_mode": true,
		})
	}

	switch mode {
	case capture.SecurityModeNormal:
		d.SetSecurityMode(capture.SecurityModeNormal, nil)
		return mcp.Succeed(req, "Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             capture.SecurityModeNormal,
			"production_parity":         true,
			"insecure_rewrites_applied": []string{},
		})
	case capture.SecurityModeInsecureProxy:
		if !params.Confirm {
			return mcp.Fail(req, mcp.ErrInvalidParam,
				"security_mode=insecure_proxy requires explicit confirmation",
				"Retry with confirm=true to acknowledge altered-environment debugging mode",
				mcp.WithParam("confirm"))
		}
		rewrites := []string{"csp_headers"}
		d.SetSecurityMode(capture.SecurityModeInsecureProxy, rewrites)
		return mcp.Succeed(req, "Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             capture.SecurityModeInsecureProxy,
			"production_parity":         false,
			"insecure_rewrites_applied": rewrites,
			"warning":                   "Altered environment active. Findings are not production-parity evidence.",
		})
	default:
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"Invalid security mode: "+params.Mode,
			"Use mode: normal or insecure_proxy",
			mcp.WithParam("mode"))
	}
}
