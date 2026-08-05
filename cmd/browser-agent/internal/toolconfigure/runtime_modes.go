// runtime_modes.go — Configures runtime telemetry, security, and action-jitter modes.
// Why: These lightweight handlers own process-local runtime behavior toggles.

package toolconfigure

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
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
	case syncruntime.SecurityModeNormal:
		d.SetSecurityMode(syncruntime.SecurityModeNormal, nil)
		return mcp.Succeed(req, "Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             syncruntime.SecurityModeNormal,
			"production_parity":         true,
			"insecure_rewrites_applied": []string{},
		})
	case syncruntime.SecurityModeInsecureProxy:
		if !params.Confirm {
			return mcp.Fail(req, mcp.ErrInvalidParam,
				"security_mode=insecure_proxy requires explicit confirmation",
				"Retry with confirm=true to acknowledge altered-environment debugging mode",
				mcp.WithParam("confirm"))
		}
		rewrites := []string{"csp_headers"}
		d.SetSecurityMode(syncruntime.SecurityModeInsecureProxy, rewrites)
		return mcp.Succeed(req, "Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             syncruntime.SecurityModeInsecureProxy,
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

// HandleTelemetry handles configure(what="telemetry").
func HandleTelemetry(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TelemetryMode string `json:"telemetry_mode"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.TelemetryMode == "" {
		return mcp.Succeed(req, "Telemetry mode", map[string]any{
			"status": "ok", "telemetry_mode": d.GetTelemetryMode(),
		})
	}

	mode, ok := NormalizeTelemetryMode(params.TelemetryMode)
	if !ok {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"Invalid telemetry_mode: "+params.TelemetryMode,
			"Use telemetry_mode: off, auto, or full",
			mcp.WithParam("telemetry_mode"))
	}

	d.SetTelemetryMode(mode)
	return mcp.Succeed(req, "Telemetry mode updated", map[string]any{
		"status": "ok", "telemetry_mode": mode,
	})
}

// NormalizeTelemetryMode validates and normalizes a telemetry mode string.
func NormalizeTelemetryMode(input string) (string, bool) {
	switch strings.TrimSpace(input) {
	case "off", "auto", "full":
		return input, true
	default:
		return "", false
	}
}

// HandleActionJitter handles configure(what="action_jitter").
func HandleActionJitter(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		ActionJitterMs *int `json:"action_jitter_ms"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.ActionJitterMs != nil {
		value := *params.ActionJitterMs
		if value < 0 {
			value = 0
		}
		if value > 5000 {
			value = 5000
		}
		d.InteractActionSetJitter(value)
	}

	return mcp.Succeed(req, "Action jitter configured", map[string]any{
		"action_jitter_ms": d.InteractActionGetJitter(),
	})
}
