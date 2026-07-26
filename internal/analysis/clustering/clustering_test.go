// Purpose: Tests for error clustering and deduplication.
// Docs: docs/features/feature/error-clustering/index.md

package clustering

import (
	"strings"
	"testing"
	"time"
)

// --- Stack Frame Parsing ---

func TestParseStackFrameStandard(t *testing.T) {
	t.Parallel()
	frame := parseStackFrame("    at UserProfile.render (user-profile.js:42:15)")
	if frame.Function != "UserProfile.render" {
		t.Fatalf("expected function=UserProfile.render, got %s", frame.Function)
	}
	if frame.File != "user-profile.js" {
		t.Fatalf("expected file=user-profile.js, got %s", frame.File)
	}
	if frame.Line != 42 {
		t.Fatalf("expected line=42, got %d", frame.Line)
	}
}

func TestParseStackFrameAnonymous(t *testing.T) {
	t.Parallel()
	frame := parseStackFrame("    at app.js:100:5")
	if frame.Function != "<anonymous>" {
		t.Fatalf("expected function=<anonymous>, got %s", frame.Function)
	}
	if frame.File != "app.js" {
		t.Fatalf("expected file=app.js, got %s", frame.File)
	}
	if frame.Line != 100 {
		t.Fatalf("expected line=100, got %d", frame.Line)
	}
}

func TestParseStackFrameNodeModules(t *testing.T) {
	t.Parallel()
	frame := parseStackFrame("    at Object.render (node_modules/react-dom/cjs/react-dom.development.js:12345:10)")
	if !frame.IsFramework {
		t.Fatal("node_modules/react frame should be marked as framework")
	}
}

func TestParseStackFrameWebpack(t *testing.T) {
	t.Parallel()
	frame := parseStackFrame("    at __webpack_require__ (webpack/bootstrap:1234:10)")
	if !frame.IsFramework {
		t.Fatal("webpack frame should be marked as framework")
	}
}

func TestParseStackFrameAppCode(t *testing.T) {
	t.Parallel()
	frame := parseStackFrame("    at Dashboard.componentDidMount (src/components/Dashboard.js:55:8)")
	if frame.IsFramework {
		t.Fatal("app code frame should NOT be marked as framework")
	}
}

// --- Message Normalization ---

func TestNormalizeMessageUUID(t *testing.T) {
	t.Parallel()
	msg := "Failed to load user 550e8400-e29b-41d4-a716-446655440000"
	norm := normalizeErrorMessage(msg)
	if !strings.Contains(norm, "{uuid}") {
		t.Fatalf("expected UUID replaced with {uuid}, got: %s", norm)
	}
}

func TestNormalizeMessageNumericID(t *testing.T) {
	t.Parallel()
	msg := "Record 12345 not found"
	norm := normalizeErrorMessage(msg)
	if !strings.Contains(norm, "{id}") {
		t.Fatalf("expected numeric ID replaced with {id}, got: %s", norm)
	}
}

func TestNormalizeMessageURL(t *testing.T) {
	t.Parallel()
	msg := "Failed to fetch https://api.example.com/users/123"
	norm := normalizeErrorMessage(msg)
	if !strings.Contains(norm, "{url}") {
		t.Fatalf("expected URL replaced with {url}, got: %s", norm)
	}
}

func TestNormalizeMessagePreservesShape(t *testing.T) {
	t.Parallel()
	msg := "Cannot read property 'name' of undefined"
	norm := normalizeErrorMessage(msg)
	if norm != msg {
		t.Fatalf("message without variables should be unchanged, got: %s", norm)
	}
}

func TestNormalizeMessageTimestamp(t *testing.T) {
	t.Parallel()
	msg := "Error at 2026-01-24T15:30:00Z processing request"
	norm := normalizeErrorMessage(msg)
	if !strings.Contains(norm, "{timestamp}") {
		t.Fatalf("expected timestamp replaced with {timestamp}, got: %s", norm)
	}
}

// --- Cluster Formation ---

func TestClusterFormationByStackSimilarity(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	// Two errors sharing stack frames
	cm.AddError(ErrorInstance{
		Message:   "TypeError: Cannot read property 'name' of undefined",
		Stack:     "    at UserProfile.render (user-profile.js:42:15)\n    at Dashboard.render (dashboard.js:100:5)",
		Timestamp: time.Now(),
	})
	cm.AddError(ErrorInstance{
		Message:   "TypeError: Cannot read property 'email' of undefined",
		Stack:     "    at UserProfile.render (user-profile.js:42:15)\n    at Sidebar.render (sidebar.js:55:3)",
		Timestamp: time.Now(),
	})

	clusters := cm.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster (shared stack frame), got %d", len(clusters))
	}
	if clusters[0].InstanceCount != 2 {
		t.Fatalf("expected 2 instances, got %d", clusters[0].InstanceCount)
	}
}

func TestClusterFormationByMessageSimilarity(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	cm.AddError(ErrorInstance{
		Message:   "Failed to fetch https://api.example.com/users/1",
		Stack:     "    at fetchData (api.js:10:5)",
		Timestamp: time.Now(),
	})
	cm.AddError(ErrorInstance{
		Message:   "Failed to fetch https://api.example.com/users/2",
		Stack:     "    at fetchData (api.js:10:5)",
		Timestamp: time.Now(),
	})

	clusters := cm.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster (same normalized message + stack), got %d", len(clusters))
	}
}

func TestClusterFormationByTemporalProximityAlone(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	now := time.Now()
	cm.AddError(ErrorInstance{
		Message:   "Error A",
		Stack:     "    at funcA (a.js:1:1)",
		Timestamp: now,
	})
	cm.AddError(ErrorInstance{
		Message:   "Error B",
		Stack:     "    at funcB (b.js:1:1)",
		Timestamp: now.Add(500 * time.Millisecond),
	})

	clusters := cm.GetClusters()
	// Temporal proximity ALONE is not enough (needs 2 signals)
	if len(clusters) != 0 {
		t.Fatalf("temporal proximity alone should not cluster, got %d clusters", len(clusters))
	}
}

func TestClusterFormationStackPlusTemporal(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	now := time.Now()
	cm.AddError(ErrorInstance{
		Message:   "Error in component A",
		Stack:     "    at shared.helper (shared.js:10:5)\n    at compA (a.js:20:3)",
		Timestamp: now,
	})
	cm.AddError(ErrorInstance{
		Message:   "Error in component B",
		Stack:     "    at shared.helper (shared.js:10:5)\n    at compB (b.js:30:3)",
		Timestamp: now.Add(500 * time.Millisecond),
	})

	clusters := cm.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("stack + temporal should cluster, got %d clusters", len(clusters))
	}
}

func TestSingleErrorNotClustered(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	cm.AddError(ErrorInstance{
		Message:   "Unique error",
		Stack:     "    at unique (unique.js:1:1)",
		Timestamp: time.Now(),
	})

	clusters := cm.GetClusters()
	if len(clusters) != 0 {
		t.Fatalf("single error should not form a cluster, got %d", len(clusters))
	}
	if cm.UnclusteredCount() != 1 {
		t.Fatalf("expected 1 unclustered error, got %d", cm.UnclusteredCount())
	}
}

func TestErrorWithoutStackClusteredByMessage(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	cm.AddError(ErrorInstance{
		Message:   "Network error: failed to fetch",
		Stack:     "",
		Timestamp: time.Now(),
	})
	cm.AddError(ErrorInstance{
		Message:   "Network error: failed to fetch",
		Stack:     "",
		Timestamp: time.Now(),
	})

	clusters := cm.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("same message without stack should cluster, got %d", len(clusters))
	}
}

// --- Root Cause Inference ---

func TestRootCauseDeepestCommonFrame(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	cm.AddError(ErrorInstance{
		Message:   "TypeError: x is undefined",
		Stack:     "    at inner (utils.js:5:1)\n    at outer (app.js:10:1)",
		Timestamp: time.Now(),
	})
	cm.AddError(ErrorInstance{
		Message:   "TypeError: y is undefined",
		Stack:     "    at inner (utils.js:5:1)\n    at other (dashboard.js:20:1)",
		Timestamp: time.Now(),
	})

	clusters := cm.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if !strings.Contains(clusters[0].RootCause, "utils.js:5") {
		t.Fatalf("root cause should be deepest common frame (utils.js:5), got: %s", clusters[0].RootCause)
	}
}

func TestFrameworkFramesExcluded(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	cm.AddError(ErrorInstance{
		Message:   "Error A",
		Stack:     "    at myFunc (app.js:10:1)\n    at React.render (node_modules/react/index.js:100:5)",
		Timestamp: time.Now(),
	})
	cm.AddError(ErrorInstance{
		Message:   "Error B",
		Stack:     "    at myFunc (app.js:10:1)\n    at React.render (node_modules/react/index.js:100:5)",
		Timestamp: time.Now(),
	})

	clusters := cm.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	// Root cause should be app code, not framework
	if strings.Contains(clusters[0].RootCause, "node_modules") {
		t.Fatalf("root cause should not be framework code, got: %s", clusters[0].RootCause)
	}
}

// --- Cluster Lifecycle ---

func TestClusterExpiresAfterInactivity(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	cm.expiryDuration = 100 * time.Millisecond // Short expiry for test

	cm.AddError(ErrorInstance{
		Message:   "Repeated error",
		Stack:     "    at func (a.js:1:1)",
		Timestamp: time.Now(),
	})
	cm.AddError(ErrorInstance{
		Message:   "Repeated error",
		Stack:     "    at func (a.js:1:1)",
		Timestamp: time.Now(),
	})

	if len(cm.GetClusters()) != 1 {
		t.Fatal("expected 1 cluster before expiry")
	}

	time.Sleep(150 * time.Millisecond)
	cm.Cleanup()

	if len(cm.GetClusters()) != 0 {
		t.Fatal("cluster should expire after inactivity")
	}
}

func TestInstanceCapAt20(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	for i := 0; i < 25; i++ {
		cm.AddError(ErrorInstance{
			Message:   "Repeated error",
			Stack:     "    at func (a.js:1:1)",
			Timestamp: time.Now(),
		})
	}

	clusters := cm.GetClusters()
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if len(clusters[0].Instances) > 20 {
		t.Fatalf("instances should be capped at 20, got %d", len(clusters[0].Instances))
	}
	if clusters[0].InstanceCount != 25 {
		t.Fatalf("instance count should be 25 (full count), got %d", clusters[0].InstanceCount)
	}
}

func TestClusterCapAt50(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	// Create 55 unique clusters (each needs 2 matching errors)
	for i := 0; i < 55; i++ {
		msg := strings.Repeat("x", i+1) // Unique normalized message per cluster
		cm.AddError(ErrorInstance{
			Message:   msg,
			Stack:     "",
			Timestamp: time.Now(),
		})
		cm.AddError(ErrorInstance{
			Message:   msg,
			Stack:     "",
			Timestamp: time.Now(),
		})
	}

	clusters := cm.GetClusters()
	if len(clusters) > 50 {
		t.Fatalf("clusters should be capped at 50, got %d", len(clusters))
	}
}

// --- Alert on Cluster Formation ---

func TestClusterAlertAt3Instances(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	cm.AddError(ErrorInstance{
		Message:   "Error",
		Stack:     "    at func (a.js:1:1)",
		Timestamp: time.Now(),
	})
	cm.AddError(ErrorInstance{
		Message:   "Error",
		Stack:     "    at func (a.js:1:1)",
		Timestamp: time.Now(),
	})

	// No alert at 2 instances
	alert := cm.DrainAlert()
	if alert != nil {
		t.Fatal("should not alert at 2 instances")
	}

	cm.AddError(ErrorInstance{
		Message:   "Error",
		Stack:     "    at func (a.js:1:1)",
		Timestamp: time.Now(),
	})

	// Alert at 3 instances
	alert = cm.DrainAlert()
	if alert == nil {
		t.Fatal("expected alert at 3 instances")
	}
	if alert.Category != "error_cluster" {
		t.Fatalf("expected category=error_cluster, got %s", alert.Category)
	}
}

// --- Analyze Tool Response ---

func TestAnalyzeErrorsResponse(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	cm.AddError(ErrorInstance{
		Message:   "TypeError: x is undefined",
		Stack:     "    at render (app.js:10:1)",
		Timestamp: time.Now(),
	})
	cm.AddError(ErrorInstance{
		Message:   "TypeError: x is undefined",
		Stack:     "    at render (app.js:10:1)",
		Timestamp: time.Now(),
	})
	// One unclustered
	cm.AddError(ErrorInstance{
		Message:   "Unique error",
		Stack:     "    at unique (other.js:99:1)",
		Timestamp: time.Now(),
	})

	resp := cm.GetAnalysisResponse()
	if len(resp.Clusters) != 1 {
		t.Fatalf("expected 1 cluster in response, got %d", len(resp.Clusters))
	}
	if resp.UnclusteredErrors != 1 {
		t.Fatalf("expected 1 unclustered error, got %d", resp.UnclusteredErrors)
	}
	if resp.TotalErrors != 3 {
		t.Fatalf("expected 3 total errors, got %d", resp.TotalErrors)
	}
}

func TestAnalyzeErrorsEmptyResponse(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	resp := cm.GetAnalysisResponse()
	if len(resp.Clusters) != 0 {
		t.Fatalf("expected 0 clusters, got %d", len(resp.Clusters))
	}
	if resp.TotalErrors != 0 {
		t.Fatalf("expected 0 total errors, got %d", resp.TotalErrors)
	}
}

// --- Concurrent Access ---

func TestClusterManagerConcurrent(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	done := make(chan bool, 100)

	for i := 0; i < 50; i++ {
		go func() {
			cm.AddError(ErrorInstance{
				Message:   "Concurrent error",
				Stack:     "    at func (a.js:1:1)",
				Timestamp: time.Now(),
			})
			done <- true
		}()
		go func() {
			cm.GetClusters()
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

// --- Server Restart Clears Clusters ---

func TestNewClusterManagerIsEmpty(t *testing.T) {
	t.Parallel()
	cm := NewClusterManager()
	clusters := cm.GetClusters()
	if len(clusters) != 0 {
		t.Fatalf("new manager should have 0 clusters, got %d", len(clusters))
	}
}
