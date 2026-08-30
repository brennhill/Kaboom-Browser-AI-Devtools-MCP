// export_test.go — Test-only handles onto this package's internals.
// Why: a symbol exported so a test can reach it becomes production API that
// nothing in production calls, and later readers cannot tell the difference. This
// file keeps those handles out of the shipped surface entirely.

package instancereg

// WriteRecordForTest lets the external test package plant specific registry
// states — a dead owner, a recycled pid, a stale heartbeat — that no production
// code path would ever write.
var WriteRecordForTest = writeRecordTo
