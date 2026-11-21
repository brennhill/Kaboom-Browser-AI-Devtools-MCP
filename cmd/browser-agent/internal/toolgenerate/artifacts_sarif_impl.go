// artifacts_sarif_impl.go — Implements generate(sarif) artifact assembly.
// Why: Keeps accessibility audit conversion flow scoped to SARIF concerns.

package toolgenerate

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/export/sarif"
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

	document, err := BuildSARIF(d.Version, a11yResult, arguments.Scope, arguments.SaveTo, arguments.IncludePasses)
	if err != nil {
		fix := "Check a11y audit results and try again."
		var internal *sarifInternalError
		if errors.As(err, &internal) {
			fix = "Report this bug."
		}
		return mcp.Fail(req, mcp.ErrNoData, err.Error(), fix)
	}
	return mcp.Succeed(req, "SARIF export complete", document)
}

// sarifInternalError marks a SARIF document that failed to round-trip (a bug,
// not bad audit input).
type sarifInternalError struct{ message string }

func (e *sarifInternalError) Error() string { return e.message }

// BuildSARIF converts accessibility audit results into a SARIF document map
// (saved to saveTo when set). Errors carry the user-facing message.
func BuildSARIF(version string, a11yResult json.RawMessage, scope, saveTo string, includePasses bool) (map[string]any, error) {
	sarifLog, err := sarif.ExportSARIF(a11yResult, sarif.SARIFExportOptions{
		Scope:         scope,
		IncludePasses: includePasses,
		SaveTo:        saveTo,
		Version:       version,
	})
	if err != nil {
		return nil, fmt.Errorf("SARIF export failed: %w", err)
	}
	sarifJSON, err := json.Marshal(sarifLog)
	if err != nil {
		return nil, &sarifInternalError{message: "SARIF marshal failed: " + err.Error()}
	}
	var sarifMap map[string]any
	if err := json.Unmarshal(sarifJSON, &sarifMap); err != nil {
		return nil, &sarifInternalError{message: "SARIF unmarshal failed: " + err.Error()}
	}
	return sarifMap, nil
}
