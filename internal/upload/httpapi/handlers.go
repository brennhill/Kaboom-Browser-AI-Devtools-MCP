// handlers.go — HTTP endpoint adapters for the canonical upload stages.
// Why: Keeps upload transport and stage behavior together without compatibility layers.
// Docs: docs/features/feature/file-upload/index.md

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/osauto"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// JSONResponder writes a JSON response with the given HTTP status code.
type JSONResponder func(w http.ResponseWriter, status int, data any)

type stageFunctions struct {
	fileRead      func(upload.FileReadRequest, *uploadsec.Security, bool) upload.FileReadResponse
	dialogInject  func(upload.FileDialogInjectRequest, *uploadsec.Security) upload.StageResponse
	formSubmit    func(context.Context, upload.FormSubmitRequest, *uploadsec.Security) upload.StageResponse
	osAutomation  func(upload.OSAutomationInjectRequest, *uploadsec.Security) upload.StageResponse
	dismissDialog func() upload.StageResponse
}

func defaultStageFunctions() stageFunctions {
	return stageFunctions{
		fileRead: upload.HandleFileRead, dialogInject: upload.HandleDialogInject,
		formSubmit: upload.HandleFormSubmitCtx, osAutomation: osauto.HandleOSAutomation,
		dismissDialog: osauto.DismissFileDialog,
	}
}

// Handlers owns upload HTTP configuration and stage dependencies.
type Handlers struct {
	security            *uploadsec.Security
	osAutomationEnabled bool
	respond             JSONResponder
	stages              stageFunctions
}

// NewHandlers constructs the canonical production upload HTTP owner.
func NewHandlers(security *uploadsec.Security, osAutomationEnabled bool, respond JSONResponder) *Handlers {
	return newHandlersWithStages(security, osAutomationEnabled, respond, defaultStageFunctions())
}

func newHandlersWithStages(security *uploadsec.Security, osAutomationEnabled bool, respond JSONResponder, stages stageFunctions) *Handlers {
	return &Handlers{security: security, osAutomationEnabled: osAutomationEnabled, respond: respond, stages: stages}
}

// ============================================
// Stage 1: File Read (POST /api/file/read)
// ============================================

// Handlers.HandleFileRead serves stage-1 file metadata reads for upload workflows.
//
// Failure semantics:
// - Invalid JSON/body size violations return 400.
// - File-not-found maps to 404; permission errors map to 403; other validation errors map to 400.
func (h *Handlers) HandleFileRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB max for request body
	var req upload.FileReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respond(w, http.StatusBadRequest, upload.FileReadResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := h.stages.fileRead(req, h.security, false)
	if resp.Success {
		h.respond(w, http.StatusOK, resp)
	} else {
		status := http.StatusBadRequest
		if strings.Contains(resp.Error, "not found") || strings.Contains(resp.Error, "no such file") {
			status = http.StatusNotFound
		} else if strings.Contains(resp.Error, "permission") {
			status = http.StatusForbidden
		}
		h.respond(w, status, resp)
	}
}

// ============================================
// Stage 2: File Dialog Injection (POST /api/file/dialog/inject)
// ============================================

// Handlers.HandleFileDialogInject serves stage-2 dialog injection preparation.
//
// Failure semantics:
// - Invalid payloads return 400; stage implementation errors are returned as validation failures.
func (h *Handlers) HandleFileDialogInject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var req upload.FileDialogInjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respond(w, http.StatusBadRequest, upload.StageResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := h.stages.dialogInject(req, h.security)
	if resp.Success {
		h.respond(w, http.StatusOK, resp)
	} else {
		h.respond(w, http.StatusBadRequest, resp)
	}
}

// ============================================
// Stage 3: Form Submission (POST /api/form/submit)
// ============================================

// Handlers.HandleFormSubmit serves stage-3 submit orchestration for upload flows.
//
// Failure semantics:
// - Request decode errors return 400; internal stage failures are returned as 400 to keep client retry semantics explicit.
func (h *Handlers) HandleFormSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10MB max for form metadata
	var req upload.FormSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respond(w, http.StatusBadRequest, upload.StageResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := h.stages.formSubmit(r.Context(), req, h.security)
	if resp.Success {
		h.respond(w, http.StatusOK, resp)
	} else {
		h.respond(w, http.StatusBadRequest, resp)
	}
}

// ============================================
// Stage 4: OS Automation (POST /api/os-automation/inject)
// ============================================

// Handlers.HandleOSAutomation serves stage-4 OS automation bridge.
//
// Invariants:
// - Execution is gated by explicit osAutomationEnabled runtime flag.
//
// Failure semantics:
// - Disabled mode returns 403 and does not attempt automation primitives.
func (h *Handlers) HandleOSAutomation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		h.respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !h.osAutomationEnabled {
		h.respond(w, http.StatusForbidden, upload.StageResponse{
			Success: false,
			Stage:   4,
			Error:   "OS-level upload automation is disabled. Start server with --enable-os-upload-automation flag.",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var req upload.OSAutomationInjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respond(w, http.StatusBadRequest, upload.StageResponse{
			Success: false,
			Stage:   4,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := h.stages.osAutomation(req, h.security)
	if resp.Success {
		h.respond(w, http.StatusOK, resp)
	} else {
		h.respond(w, http.StatusBadRequest, resp)
	}
}

// Handlers.HandleOSAutomationDismiss sends Escape to close an orphaned native file dialog.
//
// Failure semantics:
// - Disabled mode returns 403.
// - Automation transport failures return 500 because the request passed validation but could not complete.
func (h *Handlers) HandleOSAutomationDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		h.respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !h.osAutomationEnabled {
		h.respond(w, http.StatusForbidden, upload.StageResponse{
			Success: false,
			Stage:   4,
			Error:   "OS automation is disabled.",
		})
		return
	}

	resp := h.stages.dismissDialog()
	if resp.Success {
		h.respond(w, http.StatusOK, resp)
	} else {
		h.respond(w, http.StatusInternalServerError, resp)
	}
}
