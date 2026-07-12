// runtime.go — Handles configure runtime controls: daemon restart and test-boundary tracking.
// Isolates runtime/environment mutations from the configure router.

package configurehandler

import (
	"encoding/json"
	"os"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// restartSelfSignalDelay is the pause before sending SIGTERM to self during a daemon restart,
// giving the JSON-RPC response time to flush to the client.
const restartSelfSignalDelay = 100 * time.Millisecond

// HandleRestart handles configure(what="restart") that reaches the daemon. Sends self-SIGTERM
// so the bridge auto-respawns a fresh daemon (covers a responsive daemon needing a clean restart).
func HandleRestart(req mcp.JSONRPCRequest) mcp.JSONRPCResponse {
	resp := succeed(req, "Daemon restarting", map[string]any{
		"status":    "ok",
		"restarted": true,
		"message":   "Daemon shutting down — bridge will respawn automatically",
	})

	// Send SIGTERM to self after a brief delay so the response is sent first.
	util.SafeGo(func() {
		time.Sleep(restartSelfSignalDelay)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
	})

	return resp
}

// HandleTestBoundaryStart handles configure(what="test_boundary_start").
func HandleTestBoundaryStart(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	result, errResp := cfg.ParseTestBoundaryStart(req.ID, args)
	if errResp != nil {
		return *errResp
	}
	d.StartTestBoundary(result.TestID)
	return cfg.BuildTestBoundaryStartResponse(req.ID, result)
}

// HandleTestBoundaryEnd handles configure(what="test_boundary_end").
func HandleTestBoundaryEnd(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	result, errResp := cfg.ParseTestBoundaryEnd(req.ID, args)
	if errResp != nil {
		return *errResp
	}
	wasActive := d.EndTestBoundary(result.TestID)
	if !wasActive {
		return fail(req, mcp.ErrInvalidParam,
			"No active test boundary for test_id '"+result.TestID+"'",
			"Call configure({what: 'test_boundary_start', test_id: '"+result.TestID+"'}) first",
			mcp.WithParam("test_id"))
	}
	return cfg.BuildTestBoundaryEndResponse(req.ID, result, wasActive)
}
