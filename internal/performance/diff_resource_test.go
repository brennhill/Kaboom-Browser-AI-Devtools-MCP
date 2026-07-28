// diff_resource_test.go — Tests added, removed, and resized resource comparison.
// Docs: docs/features/feature/performance-audit/index.md

package performance

import (
	"testing"
)

// ============================================
// ResourceDiff: Added/Removed/Resized Resources
// ============================================

func TestResourceDiff_RemovedResource(t *testing.T) {
	t.Parallel()
	before := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
		{URL: "/old-bundle.js", Type: "script", TransferSize: 512000, Duration: 200},
		{URL: "/style.css", Type: "stylesheet", TransferSize: 20000, Duration: 50},
	}
	after := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
		{URL: "/style.css", Type: "stylesheet", TransferSize: 20000, Duration: 50},
	}

	diff := ComputeResourceDiffForNav(before, after)

	if len(diff.Removed) != 1 {
		t.Fatalf("Expected 1 removed, got %d", len(diff.Removed))
	}
	if diff.Removed[0].URL != "/old-bundle.js" {
		t.Errorf("Removed URL = %q, want /old-bundle.js", diff.Removed[0].URL)
	}
	if diff.Removed[0].SizeBytes != 512000 {
		t.Errorf("Removed size = %d, want 512000", diff.Removed[0].SizeBytes)
	}
}

func TestResourceDiff_AddedResource(t *testing.T) {
	t.Parallel()
	before := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
	}
	after := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
		{URL: "/analytics.js", Type: "script", TransferSize: 45000, Duration: 80},
	}

	diff := ComputeResourceDiffForNav(before, after)

	if len(diff.Added) != 1 {
		t.Fatalf("Expected 1 added, got %d", len(diff.Added))
	}
	if diff.Added[0].URL != "/analytics.js" {
		t.Errorf("Added URL = %q, want /analytics.js", diff.Added[0].URL)
	}
}

func TestResourceDiff_ResizedResource(t *testing.T) {
	t.Parallel()
	before := []ResourceEntry{
		{URL: "/main.js", Type: "script", TransferSize: 512000, Duration: 200},
	}
	after := []ResourceEntry{
		{URL: "/main.js", Type: "script", TransferSize: 256000, Duration: 150},
	}

	diff := ComputeResourceDiffForNav(before, after)

	if len(diff.Resized) != 1 {
		t.Fatalf("Expected 1 resized, got %d", len(diff.Resized))
	}
	if diff.Resized[0].URL != "/main.js" {
		t.Errorf("Resized URL = %q, want /main.js", diff.Resized[0].URL)
	}
	if diff.Resized[0].BaselineBytes != 512000 {
		t.Errorf("Baseline = %d, want 512000", diff.Resized[0].BaselineBytes)
	}
	if diff.Resized[0].CurrentBytes != 256000 {
		t.Errorf("Current = %d, want 256000", diff.Resized[0].CurrentBytes)
	}
}

func TestResourceDiff_SmallChangeIgnored(t *testing.T) {
	t.Parallel()
	// <10% change AND <1KB should be ignored
	before := []ResourceEntry{
		{URL: "/tiny.js", Type: "script", TransferSize: 500, Duration: 10},
	}
	after := []ResourceEntry{
		{URL: "/tiny.js", Type: "script", TransferSize: 520, Duration: 10},
	}

	diff := ComputeResourceDiffForNav(before, after)

	if len(diff.Resized) != 0 {
		t.Errorf("Tiny change should be ignored, got %d resized", len(diff.Resized))
	}
}

func TestResourceDiff_EmptyBaseline(t *testing.T) {
	t.Parallel()
	after := []ResourceEntry{
		{URL: "/app.js", Type: "script", TransferSize: 256000, Duration: 100},
	}

	diff := ComputeResourceDiffForNav(nil, after)

	// All resources are "added" when baseline is empty
	if len(diff.Added) != 1 {
		t.Errorf("Empty baseline: all resources should be added, got %d", len(diff.Added))
	}
}
