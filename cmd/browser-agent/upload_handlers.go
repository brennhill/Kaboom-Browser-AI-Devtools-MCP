// Purpose: Upload compatibility aliases and HTTP adapters for the uploadhandler package.
// Why: All package-main upload wiring changes with the extracted upload subsystem.
// Docs: docs/features/feature/file-upload/index.md

package main

import (
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"
)

type FileReadRequest = uploadhandler.FileReadRequest
type FileReadResponse = uploadhandler.FileReadResponse
type FileDialogInjectRequest = uploadhandler.FileDialogInjectRequest
type FormSubmitRequest = uploadhandler.FormSubmitRequest
type OSAutomationInjectRequest = uploadhandler.OSAutomationInjectRequest
type UploadStageResponse = uploadhandler.StageResponse
type ProgressTier = uploadhandler.ProgressTier

type UploadSecurity = uploadhandler.Security
type PathValidationResult = uploadhandler.PathValidationResult
type PathDeniedError = uploadhandler.PathDeniedError
type UploadDirRequiredError = uploadhandler.UploadDirRequiredError

const (
	ProgressTierSimple   = uploadhandler.ProgressTierSimple
	ProgressTierPeriodic = uploadhandler.ProgressTierPeriodic
	ProgressTierDetailed = uploadhandler.ProgressTierDetailed

	maxBase64FileSize          = uploadhandler.MaxBase64FileSize
	defaultEscalationTimeoutMs = uploadhandler.DefaultEscalationTimeoutMs
)

var (
	getProgressTier = uploadhandler.GetProgressTier
	detectMimeType  = uploadhandler.DetectMimeType

	ValidateUploadDir          = uploadhandler.ValidateUploadDir
	matchesDenylist            = uploadhandler.MatchesDenylist
	matchesUserDenylist        = uploadhandler.MatchesUserDenylist
	isWithinDir                = uploadhandler.IsWithinDir
	pathsEqualFold             = uploadhandler.PathsEqualFold
	pathHasPrefixFold          = uploadhandler.PathHasPrefixFold
	handleFileReadInternal     = uploadhandler.HandleFileRead
	handleDialogInjectInternal = uploadhandler.HandleDialogInject
)

// handleFileRead serves stage-1 file metadata reads for upload workflows.
func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	uploadhandler.HandleFileReadHTTP(w, r, uploadSecurityConfig, jsonResponse)
}

// handleFileDialogInject serves stage-2 dialog injection preparation.
func (s *Server) handleFileDialogInject(w http.ResponseWriter, r *http.Request) {
	uploadhandler.HandleFileDialogInjectHTTP(w, r, uploadSecurityConfig, jsonResponse)
}

// handleFormSubmit serves stage-3 submit orchestration for upload flows.
func (s *Server) handleFormSubmit(w http.ResponseWriter, r *http.Request) {
	uploadhandler.HandleFormSubmitHTTP(w, r, uploadSecurityConfig, jsonResponse)
}

// handleOSAutomation serves stage-4 OS automation bridge.
func (s *Server) handleOSAutomation(w http.ResponseWriter, r *http.Request, osAutomationEnabled bool) {
	uploadhandler.HandleOSAutomationHTTP(w, r, osAutomationEnabled, uploadSecurityConfig, jsonResponse)
}

// handleOSAutomationDismiss sends Escape to close an orphaned native file dialog.
func (s *Server) handleOSAutomationDismiss(w http.ResponseWriter, r *http.Request, osAutomationEnabled bool) {
	uploadhandler.HandleOSAutomationDismissHTTP(w, r, osAutomationEnabled, jsonResponse)
}
