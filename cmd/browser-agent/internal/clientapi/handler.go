// handler.go — Owns client registry HTTP listing, registration, lookup, and deletion.

package clientapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
)

// Register installs the client registry routes on mux.
func Register(mux *http.ServeMux, captured *capture.Capture, maxBodySize int64) {
	mux.HandleFunc("/clients", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, request *http.Request) {
		handleList(w, request, captured, maxBodySize)
	})))
	mux.HandleFunc("/clients/", httpguard.CORS(httpguard.ExtensionOnly(func(w http.ResponseWriter, request *http.Request) {
		handleClient(w, request, captured)
	})))
}

func registry(captured *capture.Capture, w http.ResponseWriter) (*clientreg.ClientRegistry, bool) {
	if captured == nil || captured.Clients().Registry() == nil {
		httpapi.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "client_registry_unavailable"})
		return nil, false
	}
	return captured.Clients().Registry(), true
}

func handleList(w http.ResponseWriter, request *http.Request, captured *capture.Capture, maxBodySize int64) {
	clients, ok := registry(captured, w)
	if !ok {
		return
	}
	switch request.Method {
	case http.MethodGet:
		httpapi.JSON(w, http.StatusOK, map[string]any{"clients": clients.List(), "count": clients.Count()})
	case http.MethodPost:
		request.Body = http.MaxBytesReader(w, request.Body, maxBodySize)
		var body struct {
			CWD string `json:"cwd"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			httpapi.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		registered := clients.Register(body.CWD)
		httpapi.JSON(w, http.StatusOK, map[string]any{"result": clientInfo(clients, registered.ID)})
	default:
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func handleClient(w http.ResponseWriter, request *http.Request, captured *capture.Capture) {
	clients, ok := registry(captured, w)
	if !ok {
		return
	}
	clientID := strings.TrimPrefix(request.URL.Path, "/clients/")
	if clientID == "" {
		httpapi.JSON(w, http.StatusBadRequest, map[string]string{"error": "Missing client ID"})
		return
	}
	switch request.Method {
	case http.MethodGet:
		if clients.Get(clientID) == nil {
			httpapi.JSON(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		httpapi.JSON(w, http.StatusOK, clientInfo(clients, clientID))
	case http.MethodDelete:
		if !clients.Unregister(clientID) {
			httpapi.JSON(w, http.StatusNotFound, map[string]string{"error": "Client not found"})
			return
		}
		httpapi.JSON(w, http.StatusOK, map[string]bool{"unregistered": true})
	default:
		httpapi.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func clientInfo(clients *clientreg.ClientRegistry, clientID string) *clientreg.ClientInfo {
	for _, info := range clients.List() {
		if info.ID == clientID {
			copy := info
			return &copy
		}
	}
	return nil
}
