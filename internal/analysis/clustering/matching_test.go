// Purpose: Branch-coverage tests for cluster matching signal counting.
// Docs: docs/features/feature/error-clustering/index.md

package clustering

import (
	"testing"
	"time"
)

// ============================================
// matchesCluster — covers stackless message match and signal count
// ============================================

func TestMatchesCluster(t *testing.T) {
	cm := NewClusterManager()

	t.Run("stackless errors match on message alone", func(t *testing.T) {
		cluster := &ErrorCluster{
			NormalizedMsg: "connection refused",
			Instances:     []ErrorInstance{{Message: "connection refused", Stack: ""}},
		}
		err := ErrorInstance{Message: "connection refused", Stack: ""}
		if !cm.matchesCluster(cluster, err, nil, "connection refused") {
			t.Error("expected match for stackless errors with same message")
		}
	})

	t.Run("stackless errors do not match different message", func(t *testing.T) {
		cluster := &ErrorCluster{
			NormalizedMsg: "connection refused",
			Instances:     []ErrorInstance{{Message: "connection refused", Stack: ""}},
		}
		err := ErrorInstance{Message: "timeout", Stack: ""}
		if cm.matchesCluster(cluster, err, nil, "timeout") {
			t.Error("expected no match for different messages")
		}
	})

	t.Run("error with stack requires 2-of-3 signals", func(t *testing.T) {
		cluster := &ErrorCluster{
			NormalizedMsg: "TypeError: undefined",
			CommonFrames: []StackFrame{
				{Function: "handleClick", File: "app.js", Line: 42},
			},
			LastSeen:  time.Now(),
			Instances: []ErrorInstance{{Stack: "at handleClick (app.js:42)"}},
		}
		// Same message + same frame = 2 signals
		err := ErrorInstance{
			Message:   "TypeError: undefined",
			Stack:     "at handleClick (app.js:42)",
			Timestamp: time.Now(),
		}
		appFrames := []StackFrame{{Function: "handleClick", File: "app.js", Line: 42}}
		if !cm.matchesCluster(cluster, err, appFrames, "TypeError: undefined") {
			t.Error("expected match with 2 signals (message + frame)")
		}
	})

	t.Run("single signal not enough", func(t *testing.T) {
		cluster := &ErrorCluster{
			NormalizedMsg: "TypeError: undefined",
			CommonFrames:  []StackFrame{{Function: "handleClick", File: "app.js", Line: 42}},
			LastSeen:      time.Now().Add(-10 * time.Second), // old
			Instances:     []ErrorInstance{{Stack: "at handleClick (app.js:42)"}},
		}
		// Only message matches, frame different, time distant = 1 signal
		err := ErrorInstance{
			Message:   "TypeError: undefined",
			Stack:     "at render (other.js:99)",
			Timestamp: time.Now(),
		}
		if cm.matchesCluster(cluster, err, []StackFrame{{Function: "render", File: "other.js", Line: 99}}, "TypeError: undefined") {
			t.Error("expected no match with only 1 signal")
		}
	})
}
