// Purpose: Implement doc.go internal behavior used by MCP runtime features.
// Why: Maintains stable server behavior across tool and transport paths.
// Docs: docs/features/feature/request-session-correlation/index.md

// doc.go — Package documentation for named session snapshots.

// Package session manages named snapshots of browser state and the diff_sessions
// MCP tool that captures, lists, compares and deletes them.
//
// The SessionManager reads live state through a CaptureStateReader, stores
// bounded named snapshots with insertion-order eviction, and answers Compare by
// delegating to the pure diff engine in the snapdiff subpackage.
package session
