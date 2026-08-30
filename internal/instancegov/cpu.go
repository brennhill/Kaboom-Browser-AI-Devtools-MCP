// cpu.go — Core-count seam for the admission policy.
// Why: the parallel cap is derived from core count, and a test must be able to pin
// that derivation without depending on the machine it runs on.

package instancegov

import "runtime"

var numCPU = runtime.NumCPU
