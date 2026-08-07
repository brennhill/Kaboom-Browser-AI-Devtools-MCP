// dispatcher_test.go — Tests canonical analyze-mode registration and routing.

package analyzedispatch

import (
	"slices"
	"testing"
)

func TestDispatcherRegistersNavigationPatterns(t *testing.T) {
	dispatcher := NewDispatcher(Config{})
	if !slices.Contains(dispatcher.ValidModes(), "navigation_patterns") {
		t.Fatalf("navigation_patterns missing from modes: %v", dispatcher.ValidModes())
	}
}
