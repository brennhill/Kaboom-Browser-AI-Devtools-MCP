// http.go — Serves bounded extension-only performance trace lifecycle uploads.

package perftrace

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxChunkBodyBytes int64 = 4 << 20

type HTTPHandler struct{ manager *Manager }

func NewHTTPHandler(manager *Manager) *HTTPHandler { return &HTTPHandler{manager: manager} }

func (h *HTTPHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	var req WirePerformanceTraceStartRequest
	if !decodePOST(w, r, &req, 4096) {
		return
	}
	started, recovered, err := h.manager.StartReplacing(req.TabID, req.ReplaceActive)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusCreated, WirePerformanceTraceStartResponse{TraceID: started.TraceID, Recovered: recovered})
}

func (h *HTTPHandler) HandleChunk(w http.ResponseWriter, r *http.Request) {
	var req WirePerformanceTraceChunkRequest
	if !decodePOST(w, r, &req, maxChunkBodyBytes) {
		return
	}
	if err := h.manager.Append(req); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"accepted": true})
}

func (h *HTTPHandler) HandleFinish(w http.ResponseWriter, r *http.Request) {
	var req WirePerformanceTraceFinishRequest
	if !decodePOST(w, r, &req, 4096) {
		return
	}
	result, err := h.manager.Finish(req)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *HTTPHandler) HandleAbort(w http.ResponseWriter, r *http.Request) {
	var req WirePerformanceTraceAbortRequest
	if !decodePOST(w, r, &req, 4096) {
		return
	}
	if err := h.manager.Abort(req.TraceID); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"aborted": true})
}

func decodePOST(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, errors.New("POST required"))
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		status := http.StatusBadRequest
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, status, err)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, errors.New("request body must contain one JSON value"))
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
