// handlers.go — HTTP handler functions for upload route dispatch, delegating core logic to internal/upload.
// Why: Provides the HTTP-facing upload endpoints while keeping validation and streaming in a testable internal package.
// Docs: docs/features/feature/file-upload/index.md

package uploadhandler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/osauto"
)

// JSONResponder writes a JSON response with the given HTTP status code.
type JSONResponder func(w http.ResponseWriter, status int, data any)

// Exported function variables for direct calls by test helpers and adapters.
var (
	HandleFileRead    = upload.HandleFileRead
	HandleDialogInject = upload.HandleDialogInject
)

// Internal function variables for HTTP handler delegation (testable via replacement).
var (
	fileReadFn      = upload.HandleFileRead
	dialogInjectFn  = upload.HandleDialogInject
	formSubmitFn    = upload.HandleFormSubmit
	osAutomationFn  = osauto.HandleOSAutomation
	dismissDialogFn = osauto.DismissFileDialog
)

// ============================================
// Stage 1: File Read (POST /api/file/read)
// ============================================

// HandleFileReadHTTP serves stage-1 file metadata reads for upload workflows.
//
// Failure semantics:
// - Invalid JSON/body size violations return 400.
// - File-not-found maps to 404; permission errors map to 403; other validation errors map to 400.
func HandleFileReadHTTP(w http.ResponseWriter, r *http.Request, securityConfig *Security, jsonResponse JSONResponder) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB max for request body
	var req FileReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, FileReadResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := fileReadFn(req, securityConfig, false)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		status := http.StatusBadRequest
		if strings.Contains(resp.Error, "not found") || strings.Contains(resp.Error, "no such file") {
			status = http.StatusNotFound
		} else if strings.Contains(resp.Error, "permission") {
			status = http.StatusForbidden
		}
		jsonResponse(w, status, resp)
	}
}

// ============================================
// Stage 2: File Dialog Injection (POST /api/file/dialog/inject)
// ============================================

// HandleFileDialogInjectHTTP serves stage-2 dialog injection preparation.
//
// Failure semantics:
// - Invalid payloads return 400; stage implementation errors are returned as validation failures.
func HandleFileDialogInjectHTTP(w http.ResponseWriter, r *http.Request, securityConfig *Security, jsonResponse JSONResponder) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var req FileDialogInjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, StageResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := dialogInjectFn(req, securityConfig)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusBadRequest, resp)
	}
}

// ============================================
// Stage 3: Form Submission (POST /api/form/submit)
// ============================================

// HandleFormSubmitHTTP serves stage-3 submit orchestration for upload flows.
//
// Failure semantics:
// - Request decode errors return 400; internal stage failures are returned as 400 to keep client retry semantics explicit.
func HandleFormSubmitHTTP(w http.ResponseWriter, r *http.Request, securityConfig *Security, jsonResponse JSONResponder) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10MB max for form metadata
	var req FormSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, StageResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := formSubmitFn(req, securityConfig)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusBadRequest, resp)
	}
}

// ============================================
// Stage 4: OS Automation (POST /api/os-automation/inject)
// ============================================

// HandleOSAutomationHTTP serves stage-4 OS automation bridge.
//
// Invariants:
// - Execution is gated by explicit osAutomationEnabled runtime flag.
//
// Failure semantics:
// - Disabled mode returns 403 and does not attempt automation primitives.
func HandleOSAutomationHTTP(w http.ResponseWriter, r *http.Request, osAutomationEnabled bool, securityConfig *Security, jsonResponse JSONResponder) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !osAutomationEnabled {
		jsonResponse(w, http.StatusForbidden, StageResponse{
			Success: false,
			Stage:   4,
			Error:   "OS-level upload automation is disabled. Start server with --enable-os-upload-automation flag.",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var req OSAutomationInjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, StageResponse{
			Success: false,
			Stage:   4,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := osAutomationFn(req, securityConfig)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusBadRequest, resp)
	}
}

// HandleOSAutomationDismissHTTP sends Escape to close an orphaned native file dialog.
//
// Failure semantics:
// - Disabled mode returns 403.
// - Automation transport failures return 500 because the request passed validation but could not complete.
func HandleOSAutomationDismissHTTP(w http.ResponseWriter, r *http.Request, osAutomationEnabled bool, jsonResponse JSONResponder) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !osAutomationEnabled {
		jsonResponse(w, http.StatusForbidden, StageResponse{
			Success: false,
			Stage:   4,
			Error:   "OS automation is disabled.",
		})
		return
	}

	resp := dismissDialogFn()
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusInternalServerError, resp)
	}
}
