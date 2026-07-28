// deps.go — Dependency injection for the toolgenerate sub-package.
// Purpose: Declares the external dependencies generate handlers need from the main package.
// Why: Decouples generate handlers from the main package's god object without circular imports.

package toolgenerate

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// Deps names the concrete owners and operations used by generate handlers.
// The composition root supplies these once when it constructs the dispatcher.
type Deps struct {
	Capture              *capture.Capture
	AnnotationStore      *annotation.Store
	Version              string
	ExecuteA11yQuery     func(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error)
	IsExtensionConnected func() bool
}
