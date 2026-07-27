// artifacts_har_impl.go — Implements generate(har) artifact assembly.
// Why: Keeps HAR export/serialization behavior isolated from other generate formats.

package toolgenerate

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/export"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// HandleExportHAR generates a HAR export from captured network data.
func HandleExportHAR(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		SaveTo    string `json:"save_to"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	cap := d.GetCapture()
	bodies := cap.GetNetworkBodies()
	waterfall := cap.GetNetworkWaterfallEntries()
	filter := capture.NetworkBodyFilter{
		URLFilter: params.URL,
		Method:    params.Method,
		StatusMin: params.StatusMin,
		StatusMax: params.StatusMax,
	}

	ver := d.GetVersion()

	if params.SaveTo != "" {
		result, err := export.ExportHARMergedToFile(bodies, waterfall, filter, ver, params.SaveTo)
		if err != nil {
			return mcp.Fail(req, mcp.ErrExportFailed, "HAR file export failed: "+err.Error(), "Check the save_to path and try again")
		}
		return mcp.Succeed(req, fmt.Sprintf("HAR exported to %s (%d entries)", result.SavedTo, result.EntriesCount), result)
	}

	harLog := export.ExportHARMerged(bodies, waterfall, filter, ver)
	summary := fmt.Sprintf("HAR export (%d entries)", len(harLog.Log.Entries))
	return mcp.Succeed(req, summary, harLog)
}
