// artifacts_sarif_impl.go — Implements generate(sarif) artifact assembly.
// Why: Keeps accessibility audit conversion flow scoped to SARIF concerns.

package toolgenerate

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/export"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// HandleExportSARIF generates a SARIF export from accessibility audit results.
func HandleExportSARIF(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var arguments struct {
		Scope         string `json:"scope"`
		IncludePasses bool   `json:"include_passes"`
		SaveTo        string `json:"save_to"`
		// Internal-use path for workflows that already executed accessibility.
		A11yResult json.RawMessage `json:"a11y_result"`
	}
	if len(args) > 0 {
		if resp, stop := mcp.ParseArgs(req, args, &arguments); stop {
			return resp
		}
	}

	// Use precomputed a11y results when available; otherwise run a11y audit.
	a11yResult := arguments.A11yResult
	if len(a11yResult) == 0 {
		if d.IsExtensionConnected() {
			var err error
			a11yResult, err = d.ExecuteA11yQuery(arguments.Scope, nil, nil, false) //nolint:staticcheck // frame=nil is correct
			if err != nil {
				a11yResult = json.RawMessage("{}")
			}
		} else {
			a11yResult = json.RawMessage("{}")
		}
	}

	// Convert to SARIF.
	sarifLog, err := export.ExportSARIF(a11yResult, export.SARIFExportOptions{
		Scope:         arguments.Scope,
		IncludePasses: arguments.IncludePasses,
		SaveTo:        arguments.SaveTo,
	})
	if err != nil {
		return mcp.Fail(req, mcp.ErrNoData, "SARIF export failed: "+err.Error(), "Check a11y audit results and try again.")
	}

	// Marshal SARIFLog to a generic map for the MCP response.
	sarifJSON, err := json.Marshal(sarifLog)
	if err != nil {
		return mcp.Fail(req, mcp.ErrNoData, "SARIF marshal failed: "+err.Error(), "Report this bug.")
	}
	var sarifMap map[string]any
	if err := json.Unmarshal(sarifJSON, &sarifMap); err != nil {
		return mcp.Fail(req, mcp.ErrNoData, "SARIF unmarshal failed: "+err.Error(), "Report this bug.")
	}

	return mcp.Succeed(req, "SARIF export complete", sarifMap)
}
