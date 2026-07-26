// Purpose: Provides non-stuttering type aliases for the session package.
// Why: Improves call-site readability while preserving backward compatibility for existing API names.

package session

// Manager is the non-stuttering spelling of SessionManager used at call sites.
//
// The DiffResult / NetworkDiff / NetworkChange aliases that used to live here
// are gone: the diff types moved to the snapdiff package, where they no longer
// stutter (snapdiff.Result, snapdiff.NetworkDiff, snapdiff.NetworkChange) and
// the aliases had no remaining users.
type Manager = SessionManager
