// export_test.go — Test-only handles onto this package's internals.

package procwatch

// ParentGoneForTest exposes the reparenting rule so its table can be tested
// directly, without making it production API that only Watch should apply.
var ParentGoneForTest = parentGone
