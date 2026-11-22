// deps.go — Dependency injection for the toolgenerate sub-package.
// Purpose: Declares the external dependencies generate handlers need from the main package.
// Why: Decouples generate handlers from the main package's god object without circular imports.

package toolgenerate

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/query"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/store/logs"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/store/telemetry"
)

// CommandHistory is the terminal command history the PR summary reads.
// internal/browsercommand.Owner satisfies it.
type CommandHistory interface {
	CompletedCommands() []*queries.CommandResult
	FailedCommands() []*queries.CommandResult
}

// Deps names the concrete owners and operations used by generate handlers.
// The composition root supplies these once when it constructs the dispatcher.
type Deps struct {
	Telemetry     *telemetrystore.Store
	ExtensionLogs *logstore.Extension
	// CommandHistory reports recent async command outcomes (browser-command owner).
	CommandHistory       CommandHistory
	TrackingStatus       func() (enabled bool, tabID int, tabURL string)
	AnnotationStore      *annotation.Store
	Version              string
	ExecuteA11yQuery     func(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error)
	IsExtensionConnected func() bool
}
