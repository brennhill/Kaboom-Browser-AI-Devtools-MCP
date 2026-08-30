// export_test.go — Test-only handles onto this package's internals.
// Why: a symbol exported so a test can reach it becomes production API that
// nothing in production calls, and a later reader cannot tell the two apart.

package instancegov

// AutoParallelCapForTest exposes the core-count derivation so its bounds can be
// pinned without making the knob part of the shipped API.
var AutoParallelCapForTest = autoParallelCap

// OldestFirstForTest exposes the eviction ordering so the corrupt-timestamp rule
// can be tested directly, without offering callers a way to re-derive an eviction
// decision that only Surplus should make.
var OldestFirstForTest = oldestFirst
