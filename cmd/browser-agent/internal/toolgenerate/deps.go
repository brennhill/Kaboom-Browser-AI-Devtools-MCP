// deps.go — Dependency injection for the toolgenerate sub-package.
// Purpose: Declares the external dependencies generate handlers need from the main package.
// Why: Decouples generate handlers from the main package's god object without circular imports.

package toolgenerate

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// Deps provides all dependencies the generate-local handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
type Deps interface {
	// GetCapture returns the capture store.
	GetCapture() *capture.Capture

	// GetAnnotationStore returns the annotation store.
	GetAnnotationStore() *annotation.Store

	// GetVersion returns the server version string.
	GetVersion() string

	// ExecuteA11yQuery runs an accessibility audit and returns results.
	ExecuteA11yQuery(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error)

	// IsExtensionConnected reports whether the browser extension is connected.
	IsExtensionConnected() bool
}
