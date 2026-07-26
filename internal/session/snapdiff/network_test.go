// Purpose: Tests for session network diff computation.
// Docs: docs/features/feature/pagination/index.md

// network_diff_test.go — Tests for network-diff.go.
// Covers: buildEndpointMap, formatDurationChange, diffNetwork with all change types.
package snapdiff

import (
	"testing"
)

func assertNetworkDiffEmpty(t *testing.T, diff NetworkDiff) {
	t.Helper()
	if len(diff.NewEndpoints) != 0 {
		t.Errorf("Expected 0 new endpoints, got %d", len(diff.NewEndpoints))
	}
	if len(diff.MissingEndpoints) != 0 {
		t.Errorf("Expected 0 missing endpoints, got %d", len(diff.MissingEndpoints))
	}
	if len(diff.StatusChanges) != 0 {
		t.Errorf("Expected 0 status changes, got %d", len(diff.StatusChanges))
	}
	if len(diff.NewErrors) != 0 {
		t.Errorf("Expected 0 new errors, got %d", len(diff.NewErrors))
	}
}

// ============================================
// buildEndpointMap
// ============================================

func TestBuildEndpointMap_Empty(t *testing.T) {
	t.Parallel()
	m := buildEndpointMap(nil)
	if len(m) != 0 {
		t.Errorf("Expected empty map for nil input, got %d entries", len(m))
	}

	m2 := buildEndpointMap([]SnapshotNetworkRequest{})
	if len(m2) != 0 {
		t.Errorf("Expected empty map for empty slice, got %d entries", len(m2))
	}
}

func TestBuildEndpointMap_KeyByMethodAndPath(t *testing.T) {
	t.Parallel()
	requests := []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users?page=1", Status: 200, Duration: 100},
		{Method: "POST", URL: "/api/users", Status: 201, Duration: 50},
	}

	m := buildEndpointMap(requests)
	if len(m) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(m))
	}

	// GET /api/users (query stripped by capture.ExtractURLPath)
	getKey := endpointKey{Method: "GET", Path: "/api/users"}
	getReq, ok := m[getKey]
	if !ok {
		t.Fatal("Expected GET /api/users in map")
	}
	if getReq.Status != 200 {
		t.Errorf("Expected GET status=200, got %d", getReq.Status)
	}

	// POST /api/users
	postKey := endpointKey{Method: "POST", Path: "/api/users"}
	postReq, ok := m[postKey]
	if !ok {
		t.Fatal("Expected POST /api/users in map")
	}
	if postReq.Status != 201 {
		t.Errorf("Expected POST status=201, got %d", postReq.Status)
	}
}

func TestBuildEndpointMap_DuplicateKeysLastWins(t *testing.T) {
	t.Parallel()
	requests := []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/data", Status: 200},
		{Method: "GET", URL: "/api/data", Status: 500},
	}

	m := buildEndpointMap(requests)
	if len(m) != 1 {
		t.Fatalf("Expected 1 entry for duplicate keys, got %d", len(m))
	}

	key := endpointKey{Method: "GET", Path: "/api/data"}
	if m[key].Status != 500 {
		t.Errorf("Expected last-wins status=500, got %d", m[key].Status)
	}
}

// ============================================
// formatDurationChange
// ============================================

func TestFormatDurationChange_PositiveDelta(t *testing.T) {
	t.Parallel()
	result := formatDurationChange(100, 250)
	if result != "+150ms" {
		t.Errorf("Expected '+150ms', got %q", result)
	}
}

func TestFormatDurationChange_NegativeDelta(t *testing.T) {
	t.Parallel()
	result := formatDurationChange(300, 150)
	if result != "-150ms" {
		t.Errorf("Expected '-150ms', got %q", result)
	}
}

func TestFormatDurationChange_ZeroDelta(t *testing.T) {
	t.Parallel()
	result := formatDurationChange(100, 100)
	if result != "+0ms" {
		t.Errorf("Expected '+0ms', got %q", result)
	}
}

func TestFormatDurationChange_BeforeZero(t *testing.T) {
	t.Parallel()
	result := formatDurationChange(0, 100)
	if result != "" {
		t.Errorf("Expected empty string when before=0, got %q", result)
	}
}

func TestFormatDurationChange_AfterZero(t *testing.T) {
	t.Parallel()
	result := formatDurationChange(100, 0)
	if result != "" {
		t.Errorf("Expected empty string when after=0, got %q", result)
	}
}

func TestFormatDurationChange_BothZero(t *testing.T) {
	t.Parallel()
	result := formatDurationChange(0, 0)
	if result != "" {
		t.Errorf("Expected empty string when both zero, got %q", result)
	}
}

func TestFormatDurationChange_NegativeValues(t *testing.T) {
	t.Parallel()
	result := formatDurationChange(-10, 100)
	if result != "" {
		t.Errorf("Expected empty string for negative before, got %q", result)
	}
}

// ============================================
// diffNetwork: New Endpoints
// ============================================

func TestDiffNetwork_NewEndpoints(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200},
		},
	}
	snapB := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200},
			{Method: "GET", URL: "/api/orders", Status: 200},
			{Method: "POST", URL: "/api/payments", Status: 201},
		},
	}

	diff := Network(snapA, snapB)

	if len(diff.NewEndpoints) != 2 {
		t.Fatalf("Expected 2 new endpoints, got %d", len(diff.NewEndpoints))
	}

	newURLs := make(map[string]bool)
	for _, ep := range diff.NewEndpoints {
		newURLs[ep.URL] = true
	}
	if !newURLs["/api/orders"] || !newURLs["/api/payments"] {
		t.Errorf("Expected /api/orders and /api/payments in new endpoints, got %v", newURLs)
	}
}

// ============================================
// diffNetwork: Missing Endpoints
// ============================================

func TestDiffNetwork_MissingEndpoints(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200},
			{Method: "GET", URL: "/api/legacy", Status: 200},
			{Method: "DELETE", URL: "/api/cleanup", Status: 204},
		},
	}
	snapB := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200},
		},
	}

	diff := Network(snapA, snapB)

	if len(diff.MissingEndpoints) != 2 {
		t.Fatalf("Expected 2 missing endpoints, got %d", len(diff.MissingEndpoints))
	}

	missingURLs := make(map[string]bool)
	for _, ep := range diff.MissingEndpoints {
		missingURLs[ep.URL] = true
	}
	if !missingURLs["/api/legacy"] || !missingURLs["/api/cleanup"] {
		t.Errorf("Expected /api/legacy and /api/cleanup in missing, got %v", missingURLs)
	}
}

// ============================================
// diffNetwork: Status Changes
// ============================================

func TestDiffNetwork_StatusChanges(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/data", Status: 200, Duration: 50},
		},
	}
	snapB := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/data", Status: 500, Duration: 200},
		},
	}

	diff := Network(snapA, snapB)

	if len(diff.StatusChanges) != 1 {
		t.Fatalf("Expected 1 status change, got %d", len(diff.StatusChanges))
	}

	sc := diff.StatusChanges[0]
	if sc.Method != "GET" {
		t.Errorf("Expected method 'GET', got %q", sc.Method)
	}
	if sc.URL != "/api/data" {
		t.Errorf("Expected URL '/api/data', got %q", sc.URL)
	}
	if sc.BeforeStatus != 200 {
		t.Errorf("Expected before_status=200, got %d", sc.BeforeStatus)
	}
	if sc.AfterStatus != 500 {
		t.Errorf("Expected after_status=500, got %d", sc.AfterStatus)
	}
	if sc.DurationChange != "+150ms" {
		t.Errorf("Expected duration_change='+150ms', got %q", sc.DurationChange)
	}
}

func TestDiffNetwork_StatusChangeOKToError_AddsToNewErrors(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "POST", URL: "/api/submit", Status: 200},
		},
	}
	snapB := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "POST", URL: "/api/submit", Status: 503},
		},
	}

	diff := Network(snapA, snapB)

	// Status 200 -> 503 should appear in NewErrors
	if len(diff.NewErrors) != 1 {
		t.Fatalf("Expected 1 new error (status change 200->503), got %d", len(diff.NewErrors))
	}
	if diff.NewErrors[0].Status != 503 {
		t.Errorf("Expected new error status=503, got %d", diff.NewErrors[0].Status)
	}
}

func TestDiffNetwork_StatusChangeErrorToError_NotNewError(t *testing.T) {
	t.Parallel()
	// 500 -> 502: error changed but was already an error
	snapA := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/health", Status: 500},
		},
	}
	snapB := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/health", Status: 502},
		},
	}

	diff := Network(snapA, snapB)

	if len(diff.StatusChanges) != 1 {
		t.Fatalf("Expected 1 status change, got %d", len(diff.StatusChanges))
	}
	// 500 -> 502: beforeStatus >= 400, so NOT a new error (was already an error)
	if len(diff.NewErrors) != 0 {
		t.Errorf("Expected 0 new errors (was already erroring), got %d", len(diff.NewErrors))
	}
}

// ============================================
// diffNetwork: New Error Endpoints
// ============================================

func TestDiffNetwork_NewEndpointWithErrorStatus(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{},
	}
	snapB := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/broken", Status: 404},
			{Method: "GET", URL: "/api/ok", Status: 200},
		},
	}

	diff := Network(snapA, snapB)

	if len(diff.NewEndpoints) != 2 {
		t.Fatalf("Expected 2 new endpoints, got %d", len(diff.NewEndpoints))
	}
	// Only the 404 should be in NewErrors
	if len(diff.NewErrors) != 1 {
		t.Fatalf("Expected 1 new error, got %d", len(diff.NewErrors))
	}
	if diff.NewErrors[0].Status != 404 {
		t.Errorf("Expected new error status=404, got %d", diff.NewErrors[0].Status)
	}
}

// ============================================
// diffNetwork: No Changes
// ============================================

func TestDiffNetwork_NoChanges(t *testing.T) {
	t.Parallel()
	requests := []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
		{Method: "POST", URL: "/api/login", Status: 200},
	}

	snapA := &NamedSnapshot{NetworkRequests: requests}
	snapB := &NamedSnapshot{NetworkRequests: requests}

	diff := Network(snapA, snapB)
	assertNetworkDiffEmpty(t, diff)
}

// ============================================
// diffNetwork: Empty Snapshots
// ============================================

func TestDiffNetwork_BothEmpty(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{NetworkRequests: []SnapshotNetworkRequest{}}
	snapB := &NamedSnapshot{NetworkRequests: []SnapshotNetworkRequest{}}

	diff := Network(snapA, snapB)
	assertNetworkDiffEmpty(t, diff)
}

func TestDiffNetwork_NilNetworkRequests(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{NetworkRequests: nil}
	snapB := &NamedSnapshot{NetworkRequests: nil}

	diff := Network(snapA, snapB)

	if len(diff.NewEndpoints) != 0 {
		t.Errorf("Expected 0 new endpoints for nil, got %d", len(diff.NewEndpoints))
	}
}

// ============================================
// diffNetwork: Duration Change in Status Changes
// ============================================

func TestDiffNetwork_DurationChangeNoDuration(t *testing.T) {
	t.Parallel()
	// Status change with zero duration (missing data)
	snapA := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/data", Status: 200, Duration: 0},
		},
	}
	snapB := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/data", Status: 500, Duration: 0},
		},
	}

	diff := Network(snapA, snapB)

	if len(diff.StatusChanges) != 1 {
		t.Fatalf("Expected 1 status change, got %d", len(diff.StatusChanges))
	}
	if diff.StatusChanges[0].DurationChange != "" {
		t.Errorf("Expected empty duration_change for zero durations, got %q", diff.StatusChanges[0].DurationChange)
	}
}

// ============================================
// diffNetwork: Query Param Matching
// ============================================

func TestDiffNetwork_QueryParamsIgnoredForMatching(t *testing.T) {
	t.Parallel()
	snapA := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "https://api.example.com/users?page=1&limit=10", Status: 200},
		},
	}
	snapB := &NamedSnapshot{
		NetworkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "https://api.example.com/users?page=2&limit=20", Status: 200},
		},
	}

	diff := Network(snapA, snapB)

	// Should match on path, not new/missing
	if len(diff.NewEndpoints) != 0 {
		t.Errorf("Expected 0 new (same path), got %d", len(diff.NewEndpoints))
	}
	if len(diff.MissingEndpoints) != 0 {
		t.Errorf("Expected 0 missing (same path), got %d", len(diff.MissingEndpoints))
	}
}
