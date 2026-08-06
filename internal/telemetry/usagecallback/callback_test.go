// features_callback_test.go — Tests for extension UI feature usage flowing
// through the features callback into the usage counter with ext: prefix.

package usagecallback

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

func TestFeaturesCallbackWiresIntoUsageTracker(t *testing.T) {
	t.Parallel()

	counter := telemetry.NewUsageTracker()
	cb := New(counter)

	cb(map[string]bool{
		"screenshot":  true,
		"annotations": true,
		"video":       false,
		"dom_action":  true,
	})

	counts := counter.DebugCounts()

	if counts["ext:screenshot"] != 1 {
		t.Errorf("ext:screenshot = %d, want 1", counts["ext:screenshot"])
	}
	if counts["ext:annotations"] != 1 {
		t.Errorf("ext:annotations = %d, want 1", counts["ext:annotations"])
	}
	if counts["ext:dom_action"] != 1 {
		t.Errorf("ext:dom_action = %d, want 1", counts["ext:dom_action"])
	}
	if counts["ext:video"] != 0 {
		t.Errorf("ext:video = %d, want 0 (was false)", counts["ext:video"])
	}
}

func TestFeaturesCallback_OnlyTrueValuesIncrement(t *testing.T) {
	t.Parallel()

	counter := telemetry.NewUsageTracker()
	cb := New(counter)

	cb(map[string]bool{
		"screenshot":  false,
		"annotations": false,
	})

	counts := counter.DebugCounts()
	if len(counts) != 0 {
		t.Errorf("Expected empty counts for all-false features, got %v", counts)
	}
}

func TestFeaturesCallback_MultipleInvocations_Accumulate(t *testing.T) {
	t.Parallel()

	counter := telemetry.NewUsageTracker()
	cb := New(counter)

	cb(map[string]bool{"screenshot": true})
	cb(map[string]bool{"screenshot": true, "video": true})

	counts := counter.DebugCounts()
	if counts["ext:screenshot"] != 2 {
		t.Errorf("ext:screenshot = %d, want 2", counts["ext:screenshot"])
	}
	if counts["ext:video"] != 1 {
		t.Errorf("ext:video = %d, want 1", counts["ext:video"])
	}
}
