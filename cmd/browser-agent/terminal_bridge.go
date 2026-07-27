// terminal_bridge.go -- Wires the terminal sub-package into the main daemon.
// Why: Thin adapter between the main Server and the extracted terminal package,
// keeping the terminal subsystem decoupled from the god package.

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpguard"
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/wsframe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

// terminalDeps builds the Deps struct for the terminal package from main-package functions.
func terminalDeps() terminal.Deps {
	return terminal.Deps{
		JSONResponse:   jsonResponse,
		CORSMiddleware: httpguard.CORS,
		Stderrf:        diag.Printf,
		MaxPostBody:    maxPostBodySize,
		WSReadFrame:    wsframe.ReadFrame,
		WSWriteFrame:   wsframe.WriteFrame,
		WSAcceptKey:    wsframe.AcceptKey,
	}
}

// serverIntentDeps adapts *Server to the terminal.IntentDeps interface.
type serverIntentDeps struct{ s *Server }

func (d *serverIntentDeps) GetPtyRelays() terminal.RelayMap {
	if d.s.ptyRelays == nil {
		return nil
	}
	return d.s.ptyRelays
}
func (d *serverIntentDeps) GetIntentStore() *terminal.IntentStore { return d.s.intentStore }

// setupTerminalMux creates a new ServeMux with only terminal routes.
func setupTerminalMux(server *Server, mgr *pty.Manager, cap *capture.Store) (*http.ServeMux, *terminal.Map) {
	deps := terminalDeps()
	deps.LogEvent = func(event string, fields map[string]any) { server.logLifecycle(event, 0, fields) }
	return terminal.SetupMux(deps, server, &serverIntentDeps{s: server}, mgr, cap)
}

// startTerminalServer launches the terminal HTTP server on the given port.
func startTerminalServer(port int, mux *http.ServeMux) (*http.Server, <-chan struct{}, error) {
	deps := terminalDeps()
	return terminal.StartServer(deps, port, mux)
}

// handleActiveCodebase delegates to the terminal package.
func handleActiveCodebase(w http.ResponseWriter, r *http.Request, server *Server) {
	deps := terminalDeps()
	terminal.HandleActiveCodebase(w, r, deps, server)
}
