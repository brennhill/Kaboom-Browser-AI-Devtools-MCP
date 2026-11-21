// export_test.go — Test-only handles onto this package's internals.

package procidentity

// MatchesForTest exposes the identity comparison so the recycled-pid rule can be
// tested directly, without exporting it into the production API where a caller
// could re-derive liveness and get it subtly different from IsAlive.
var MatchesForTest = matches

// LookupForTest exposes the single-pid identity query to tests.
var LookupForTest = lookup

// BuildInfoForTest exposes the platform-neutral Info constructor used by the
// Windows snapshot so its validation is tested on every OS.
var BuildInfoForTest = buildInfo
