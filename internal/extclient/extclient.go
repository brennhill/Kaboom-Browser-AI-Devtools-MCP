// extclient.go — Canonical identity of clients allowed on extension-facing endpoints.
// Why: the guard that admits a client and the sync runtime that decides whether to adopt
//      it as the authoritative extension session must never drift on what "the extension"
//      means. A harness validating the endpoint contract is not a second browser.
// Docs: docs/features/feature/self-testing/index.md

package extclient

import "strings"

const (
	// Extension is the browser extension's client identity, optionally suffixed
	// with "/<version>".
	Extension = "kaboom-extension"
	// ExtensionOffscreen is the offscreen recording worker, which runs inside the
	// same extension and speaks for the same session.
	ExtensionOffscreen = "kaboom-extension-offscreen"
	// Probe is a client that exercises extension-facing endpoints without being a
	// browser: UAT categories, contract tests, and diagnostics. Its requests are
	// validated and answered, but it never owns the extension session.
	Probe = "kaboom-probe"
)

// IsExtension reports whether the client speaks for the browser extension and may
// therefore own the session, receive queued commands, and report tracking state.
func IsExtension(clientID string) bool {
	return clientID == Extension ||
		clientID == ExtensionOffscreen ||
		strings.HasPrefix(clientID, Extension+"/")
}

// IsProbe reports whether the client is validating the endpoint contract rather than
// driving a browser. A probe must never displace a real extension: adopting it would
// bump the connection generation (superseding the extension's in-flight poll) and let
// its partial settings erase the tracked tab the extension reported.
func IsProbe(clientID string) bool {
	return clientID == Probe || strings.HasPrefix(clientID, Probe+"/")
}

// Allowed reports whether the client may reach extension-facing endpoints at all.
func Allowed(clientID string) bool {
	return IsExtension(clientID) || IsProbe(clientID)
}
