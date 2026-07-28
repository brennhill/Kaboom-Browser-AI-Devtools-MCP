// upload.go — Handler: validates an interact upload request and queues it for the
// extension's 4-stage escalation (extension inject, form submit, direct API, OS
// automation).
//
// Package interactupload owns the upload action of the interact tool. Like
// interactstate it is its own package because it is its own handler type with its
// own dependency (internal/upload) — and because everything it does before
// queueing is pure validation of a caller-supplied path, which is exactly the code
// that benefits from being addressable without a browser.
//
// Docs: docs/features/feature/file-upload/index.md
package interactupload

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolresp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
)

// EvidenceArmer is the one thing this handler needs from the interact action
// handler: arming the before/after evidence capture for the command it is about
// to queue. Declared as a one-method interface so upload does not depend on the
// action handler's whole surface — and so the dependency arrow cannot grow
// without someone widening this interface on purpose.
type EvidenceArmer interface {
	ArmEvidenceForCommand(correlationID, action string, args json.RawMessage, clientID string)
}

// Deps are the host-owned seams this handler needs.
type Deps struct {
	// RequirePilot checks that pilot mode is enabled.
	RequirePilot toolguard.Check
	// RequireExtension checks that the extension is connected.
	RequireExtension toolguard.Check
	// RequireTabTracking checks that tab tracking is active.
	RequireTabTracking toolguard.Check
	// EnqueuePendingQuery queues a command for the extension.
	EnqueuePendingQuery func(req mcp.JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (mcp.JSONRPCResponse, bool)
	// RecordAIAction records an AI-driven action to the enhanced actions buffer.
	RecordAIAction func(action, url string, extra map[string]any)
}

// Handler handles file upload operations.
type Handler struct {
	deps     *Deps
	evidence EvidenceArmer
}

// New creates a new Handler with the given dependencies.
func New(deps *Deps, evidence EvidenceArmer) *Handler {
	return &Handler{deps: deps, evidence: evidence}
}

// uploadParams holds the parsed and validated upload parameters.
type uploadParams struct {
	Selector            string `json:"selector"`
	APIEndpoint         string `json:"api_endpoint,omitempty"`
	FilePath            string `json:"file_path"`
	Submit              bool   `json:"submit,omitempty"`
	EscalationTimeoutMs int    `json:"escalation_timeout_ms,omitempty"`
}

// HandleUpload dispatches the "upload" interact action.
// Validates parameters and queues the upload operation.
func (u *Handler) HandleUpload(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params uploadParams
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	if errResp := validateUploadParams(req, params); errResp != nil {
		return *errResp
	}

	if resp, blocked := u.deps.RequirePilot(req); blocked {
		return resp
	}
	if resp, blocked := u.deps.RequireExtension(req); blocked {
		return resp
	}
	if resp, blocked := u.deps.RequireTabTracking(req); blocked {
		return resp
	}

	info, errResp := validateUploadFile(req, params.FilePath)
	if errResp != nil {
		return *errResp
	}

	return u.queueUpload(req, args, params, info)
}

// validateUploadParams checks required parameters for the upload action.
func validateUploadParams(req mcp.JSONRPCRequest, params uploadParams) *mcp.JSONRPCResponse {
	if params.FilePath == "" {
		resp := mcp.Fail(req, mcp.ErrMissingParam,
			"Required parameter 'file_path' is missing",
			"Add the 'file_path' parameter with an absolute path to the file",
			mcp.WithParam("file_path"))
		return &resp
	}
	if params.Selector == "" && params.APIEndpoint == "" {
		resp := mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'selector' is missing. Provide a CSS selector for the file input element, or use 'api_endpoint' for direct API uploads.", "Add the 'selector' parameter (e.g., '#Filedata') or 'api_endpoint'", mcp.WithParam("selector"))
		return &resp
	}
	if !filepath.IsAbs(params.FilePath) {
		resp := mcp.Fail(req, mcp.ErrPathNotAllowed, "file_path must be an absolute path. Relative paths are not allowed for security.", "Use an absolute path like '/Users/user/Videos/video.mp4'", mcp.WithParam("file_path"))
		return &resp
	}
	return nil
}

// validateUploadFile checks that the file exists, is readable, and is not a directory.
func validateUploadFile(req mcp.JSONRPCRequest, filePath string) (os.FileInfo, *mcp.JSONRPCResponse) {
	info, err := os.Stat(filePath)
	if err != nil {
		resp := uploadFileStatError(req, filePath, err)
		return nil, &resp
	}
	if info.IsDir() {
		resp := mcp.Fail(req, mcp.ErrInvalidParam, "Path is a directory, not a file: "+filePath, "Provide a path to a file, not a directory", mcp.WithParam("file_path"))
		return nil, &resp
	}
	return info, nil
}

func uploadFileStatError(req mcp.JSONRPCRequest, filePath string, err error) mcp.JSONRPCResponse {
	if os.IsNotExist(err) {
		return mcp.Fail(req, mcp.ErrInvalidParam, "File not found: "+filePath+". Verify the file path is correct.", "Check the file path and try again", mcp.WithParam("file_path"))
	}
	if os.IsPermission(err) {
		return mcp.Fail(req, mcp.ErrPathNotAllowed, "Permission denied reading file: "+filePath+". Check file permissions.", "Fix file permissions with: chmod +r "+filePath, mcp.WithParam("file_path"))
	}
	return mcp.Fail(req, mcp.ErrInternal, "Failed to access file: "+err.Error(), "Check the file path and permissions")
}

// queueUpload builds the upload payload and queues it for the extension.
func (u *Handler) queueUpload(req mcp.JSONRPCRequest, args json.RawMessage, params uploadParams, info os.FileInfo) mcp.JSONRPCResponse {
	if params.EscalationTimeoutMs <= 0 {
		params.EscalationTimeoutMs = upload.DefaultEscalationTimeoutMs
	}

	fileName := filepath.Base(params.FilePath)
	mimeType := upload.DetectMimeType(fileName)
	fileSize := info.Size()
	progressTier := upload.GetProgressTier(fileSize)
	correlationID := toolresp.NewCorrelationID("upload")
	u.evidence.ArmEvidenceForCommand(correlationID, "upload", args, req.ClientID)

	uploadPayload := map[string]any{
		"action": "upload", "selector": params.Selector,
		"file_path": params.FilePath, "file_name": fileName,
		"file_size": fileSize, "mime_type": mimeType,
		"submit": params.Submit, "escalation_timeout_ms": params.EscalationTimeoutMs,
		"progress_tier": string(progressTier),
	}
	if params.APIEndpoint != "" {
		uploadPayload["api_endpoint"] = params.APIEndpoint
	}

	// Error impossible: map contains only primitive types from input
	payloadJSON, _ := json.Marshal(uploadPayload)
	query := queries.PendingQuery{Type: "upload", Params: payloadJSON, CorrelationID: correlationID}
	if enqueueResp, blocked := u.deps.EnqueuePendingQuery(req, query, 10*time.Minute); blocked {
		return enqueueResp
	}

	u.deps.RecordAIAction("upload", "", map[string]any{
		"file_path": params.FilePath, "file_name": fileName,
		"file_size": fileSize, "selector": params.Selector,
		"progress_tier": string(progressTier),
	})

	return mcp.Succeed(req, "Upload queued", map[string]any{
		"status": "queued", "correlation_id": correlationID,
		"file_name": fileName, "file_size": fileSize,
		"mime_type": mimeType, "progress_tier": string(progressTier),
		"message": "Upload queued for execution. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})
}
