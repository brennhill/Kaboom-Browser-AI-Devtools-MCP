// telemetry.go — Handles configure(what="telemetry") for toggling telemetry modes.
// Why: Isolates telemetry mode mutation from the configure router.

package configurehandler

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// HandleTelemetry handles configure(what="telemetry").
func HandleTelemetry(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		TelemetryMode string `json:"telemetry_mode"`
	}
	lenientUnmarshal(args, &params)

	if params.TelemetryMode == "" {
		return succeed(req, "Telemetry mode", map[string]any{
			"status":         "ok",
			"telemetry_mode": d.GetTelemetryMode(),
		})
	}

	mode, ok := NormalizeTelemetryMode(params.TelemetryMode)
	if !ok {
		return fail(req, mcp.ErrInvalidParam,
			"Invalid telemetry_mode: "+params.TelemetryMode,
			"Use telemetry_mode: off, auto, or full",
			mcp.WithParam("telemetry_mode"))
	}

	d.SetTelemetryMode(mode)
	return succeed(req, "Telemetry mode updated", map[string]any{
		"status":         "ok",
		"telemetry_mode": mode,
	})
}

// NormalizeTelemetryMode validates and normalizes a telemetry mode string.
// Returns the canonical mode and true, or empty string and false for invalid values.
func NormalizeTelemetryMode(input string) (string, bool) {
	switch strings.TrimSpace(input) {
	case "off", "auto", "full":
		return input, true
	default:
		return "", false
	}
}
