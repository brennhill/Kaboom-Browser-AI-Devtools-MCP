// security.go — Re-exports cross-platform upload security types from internal/upload.
// Why: Surfaces upload security validation as package-level aliases for use by upload HTTP handlers.
// Docs: docs/features/feature/file-upload/index.md

package uploadhandler

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"

// ============================================
// Type Aliases
// ============================================

type Security = uploadsec.Security
type PathValidationResult = uploadsec.PathValidationResult
type PathDeniedError = uploadsec.PathDeniedError
type UploadDirRequiredError = uploadsec.UploadDirRequiredError

// ============================================
// Function and Variable Aliases
// ============================================

// ValidateUploadDir validates the --upload-dir flag at startup.
var ValidateUploadDir = uploadsec.ValidateUploadDir

var (
	MatchesDenylist     = uploadsec.MatchesDenylist
	MatchesUserDenylist = uploadsec.MatchesUserDenylist
	IsWithinDir         = uploadsec.IsWithinDir
	PathsEqualFold      = uploadsec.PathsEqualFold
	PathHasPrefixFold   = uploadsec.PathHasPrefixFold
)

// IsPrivateIP re-exports SSRF-safe IP validation.
var IsPrivateIP = uploadsec.IsPrivateIP

// NewSecurity creates a new upload security configuration.
var NewSecurity = uploadsec.NewSecurity

// SetSkipSSRFCheck enables/disables SSRF check bypass (for testing only).
var SetSkipSSRFCheck = uploadsec.SetSkipSSRFCheck

// SetSSRFAllowedHosts configures the SSRF allowed-hosts list.
var SetSSRFAllowedHosts = uploadsec.SetSSRFAllowedHosts

// NewSSRFSafeTransport creates an HTTP transport with SSRF protection.
var NewSSRFSafeTransport = uploadsec.NewSSRFSafeTransport
