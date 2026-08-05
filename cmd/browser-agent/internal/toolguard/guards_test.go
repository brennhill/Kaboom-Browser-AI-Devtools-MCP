// guards_test.go — Defines browser-runtime guard policy contracts.
package toolguard

import (
	"testing"
	"time"
)

func TestDefaultExtensionReadinessTimeoutIsBounded(t *testing.T) {
	if DefaultExtensionReadinessTimeout <= 0 || DefaultExtensionReadinessTimeout > 10*time.Second {
		t.Fatalf("DefaultExtensionReadinessTimeout = %v, want (0, 10s]", DefaultExtensionReadinessTimeout)
	}
}
