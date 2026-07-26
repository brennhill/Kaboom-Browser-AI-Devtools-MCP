// Purpose: Registers and serves multi-client registry HTTP endpoints (/clients, /clients/{id}).
// Why: Keeps client-registry HTTP behavior isolated from broader route wiring for maintainability and test focus.

package main

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpguard"
	"net/http"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func resolveClientRegistry(cap *capture.Store, w http.ResponseWriter) (capture.ClientRegistry, bool) {
	reg := cap.GetClientRegistry()
	if reg == nil {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]string{
			"error": "client_registry_unavailable",
		})
		return nil, false
	}
	return reg, true
}

// registerClientRegistryRoutes binds the multi-client registry endpoints.
func registerClientRegistryRoutes(mux *http.ServeMux, cap *capture.Store) {
	mux.HandleFunc("/clients", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientsList(w, r, cap)
	})))
	mux.HandleFunc("/clients/", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, r *http.Request) {
		handleClientByID(w, r, cap)
	})))
}

// handleClientsList handles GET/POST on /clients for multi-client management.
func handleClientsList(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	reg, ok := resolveClientRegistry(cap, w)
	if !ok {
		return
	}

	switch r.Method {
	case "GET":
		clients := reg.List()
		jsonResponse(w, http.StatusOK, map[string]any{
			"clients": clients,
			"count":   reg.Count(),
		})
	case "POST":
		r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
		var body struct {
			CWD string `json:"cwd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		cs := reg.Register(body.CWD)
		jsonResponse(w, http.StatusOK, map[string]any{
			"result": cs,
		})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// handleClientByID handles GET/DELETE on /clients/{id} for specific client operations.
func handleClientByID(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	reg, ok := resolveClientRegistry(cap, w)
	if !ok {
		return
	}

	clientID := strings.TrimPrefix(r.URL.Path, "/clients/")
	if clientID == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing client ID"})
		return
	}

	switch r.Method {
	case "GET":
		cs := reg.Get(clientID)
		if cs == nil {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		jsonResponse(w, http.StatusOK, cs)
	case "DELETE":
		if !reg.Unregister(clientID) {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"unregistered": true})
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}
