// interact_upload_handler.go — Defines the dedicated interact upload sub-handler.
// Purpose: Narrows ToolHandler responsibilities by isolating upload-specific validation/queueing.
// Docs: docs/features/feature/file-upload/index.md

package interacthandler

// UploadInteractHandler handles file upload operations.
type UploadInteractHandler struct {
	deps          *Deps
	actionHandler *InteractActionHandler
}

// NewUploadInteractHandler creates a new UploadInteractHandler with the given dependencies.
func NewUploadInteractHandler(deps *Deps, actionHandler *InteractActionHandler) *UploadInteractHandler {
	return &UploadInteractHandler{deps: deps, actionHandler: actionHandler}
}
