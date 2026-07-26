// Purpose: Defines cross-package interfaces consumed by capture for schema, CSP, and client-registry integrations.
// Why: Decouples capture from concrete implementations to avoid import cycles and tighten test seams.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import "encoding/json"

// SchemaStore defines the interface for API schema detection and tracking.
// Implemented by *analysis.SchemaStore. Methods called by HTTP handlers and observers.
// Has its own lock; safe to call outside Capture.mu.
type SchemaStore interface {
	// EndpointCount returns the number of unique endpoints observed
	EndpointCount() int
}

// CSPGenerator defines the interface for Content-Security-Policy generation.
// Implemented by *csp.Generator (internal/security/csp). Called by HTTP handlers.
// Has its own lock; safe to call outside Capture.mu.
type CSPGenerator interface {
	// HandleGenerateCSP is the MCP tool handler for CSP generation.
	// params is a JSON-encoded csp.Params; returns *csp.Response.
	HandleGenerateCSP(params json.RawMessage) (any, error)
}

// ClientRegistry defines the interface for managing connected MCP clients.
// Implemented by *session.ClientRegistry. Called by HTTP handlers.
// Lock hierarchy: ClientRegistry.mu is position 1 (outermost), before Capture.mu.
//
// Return types use any to avoid an import cycle (session imports capture):
//   - List() returns []session.ClientInfo
//   - Register() returns *session.ClientState
//   - Get() returns *session.ClientState (nil if not found)
type ClientRegistry interface {
	// Count returns the number of registered clients.
	Count() int
	// List returns all registered clients as []session.ClientInfo.
	List() any
	// Register creates or updates a client registration, returning *session.ClientState.
	Register(cwd string) any
	// Get returns a specific client by ID as *session.ClientState, or nil if not found.
	Get(id string) any
	// Unregister removes a client by ID and reports whether the client existed.
	Unregister(id string) bool
}
