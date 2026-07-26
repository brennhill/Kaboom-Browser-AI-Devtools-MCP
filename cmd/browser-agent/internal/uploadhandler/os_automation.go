// os_automation.go — Re-exports OS-level upload automation functions from internal/upload.
// Why: Stage 4 upload escalation requires OS automation gated behind --enable-os-upload-automation.
// Docs: docs/features/feature/file-upload/index.md

package uploadhandler

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/osauto"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// Handler function aliases.
var (
	HandleOSAutomation = osauto.HandleOSAutomation
	DetectBrowserPID   = osauto.DetectBrowserPID
	DismissFileDialog  = osauto.DismissFileDialog
	ExecuteOSAutomation = osauto.ExecuteOSAutomation
)

// Validator and sanitizer function aliases.
var (
	ValidatePathForOSAutomation   = uploadsec.ValidatePathForOSAutomation
	ValidateHTTPMethod            = uploadsec.ValidateHTTPMethod
	ValidateFormActionURL         = uploadsec.ValidateFormActionURL
	ValidateCookieHeader          = uploadsec.ValidateCookieHeader
	SanitizeForContentDisposition = uploadsec.SanitizeForContentDisposition
	SanitizeForAppleScript        = uploadsec.SanitizeForAppleScript
	SanitizeForSendKeys           = uploadsec.SanitizeForSendKeys
)
