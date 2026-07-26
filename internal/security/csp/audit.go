// audit.go — The audit block a Response carries when session overrides were applied.
// Purpose: Defines the CSP audit structure for tracking session overrides and persistent whitelists.
// Why: Isolates CSP audit types from the main security configuration.
//
// This type was filed under security_config_* by name but nothing in the config
// cluster ever referenced it — it is part of the CSP response schema, and its
// only reader is Response.Audit. It lives with the schema it belongs to.
package csp

// Audit reports which whitelist overrides shaped a generated policy and where
// they came from, so a reviewer can tell session-only overrides from persistent
// configuration.
type Audit struct {
	SessionOverrides    []string `json:"session_overrides,omitempty"`
	PersistentWhitelist []string `json:"persistent_whitelist,omitempty"`
	OverrideSource      string   `json:"override_source,omitempty"`
}
