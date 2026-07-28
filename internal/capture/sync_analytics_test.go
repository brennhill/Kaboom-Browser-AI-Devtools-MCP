// sync_analytics_test.go — Tests for analytics fields in sync protocol:
// features_used callback, install_id in response.

package capture

import (
	"net/http"
	"sync"
	"testing"
)

func TestHandleSync_FeaturesUsedInvokesCallback(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	var mu sync.Mutex
	var callbackInvoked bool
	var receivedFeatures map[string]bool

	cap.FeatureUsage().SetCallback(func(features map[string]bool) {
		mu.Lock()
		callbackInvoked = true
		receivedFeatures = features
		mu.Unlock()
	})

	req := SyncRequest{
		ExtSessionID: "analytics_test",
		FeaturesUsed: map[string]bool{
			"screenshot":  true,
			"annotations": true,
			"video":       false,
		},
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if !callbackInvoked {
		t.Fatal("expected feature-usage observer to be notified")
	}
	if !receivedFeatures["screenshot"] {
		t.Error("Expected screenshot=true in callback")
	}
	if !receivedFeatures["annotations"] {
		t.Error("Expected annotations=true in callback")
	}
	if receivedFeatures["video"] {
		t.Error("Expected video=false in callback")
	}
}

func TestHandleSync_FeaturesUsedEmpty_NoCallback(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	var mu sync.Mutex
	var callbackInvoked bool
	cap.FeatureUsage().SetCallback(func(_ map[string]bool) {
		mu.Lock()
		callbackInvoked = true
		mu.Unlock()
	})

	req := SyncRequest{
		ExtSessionID: "analytics_test_empty",
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if callbackInvoked {
		t.Error("Callback should not be invoked when features_used is empty")
	}
}

func TestHandleSync_FeaturesUsedNoCallback_NoPanic(t *testing.T) {
	t.Parallel()
	cap := NewCapture()
	// No callback set — should not panic.

	req := SyncRequest{
		ExtSessionID: "analytics_test_no_cb",
		FeaturesUsed: map[string]bool{"screenshot": true},
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilterFeaturesUsed_AllowsKnownKeys(t *testing.T) {
	t.Parallel()
	raw := map[string]bool{
		"screenshot":       true,
		"annotations":      true,
		"video":            false,
		"dom_action":       true,
		"action_recording": true,
	}
	filtered := filterFeaturesUsed(raw)
	if len(filtered) != 5 {
		t.Fatalf("Expected 5 keys, got %d: %v", len(filtered), filtered)
	}
	// action_recording is wire-synced with the UIFeature union on the extension
	// side; if this drops, an extension release will silently lose the metric.
	if !filtered["action_recording"] {
		t.Error("Expected action_recording to be an allowed feature key")
	}
}

func TestFilterFeaturesUsed_RejectsUnknownKeys(t *testing.T) {
	t.Parallel()
	raw := map[string]bool{
		"screenshot":    true,
		"evil_key":      true,
		"another_bogus": true,
	}
	filtered := filterFeaturesUsed(raw)
	if len(filtered) != 1 {
		t.Fatalf("Expected 1 key (screenshot only), got %d: %v", len(filtered), filtered)
	}
	if !filtered["screenshot"] {
		t.Error("Expected screenshot=true in filtered output")
	}
	if _, ok := filtered["evil_key"]; ok {
		t.Error("evil_key should have been filtered out")
	}
}

func TestFilterFeaturesUsed_AllUnknown_ReturnsNil(t *testing.T) {
	t.Parallel()
	raw := map[string]bool{"bogus": true, "nonsense": true}
	filtered := filterFeaturesUsed(raw)
	if filtered != nil {
		t.Errorf("Expected nil for all-unknown keys, got %v", filtered)
	}
}

func TestFilterFeaturesUsed_Empty_ReturnsNil(t *testing.T) {
	t.Parallel()
	if filterFeaturesUsed(nil) != nil {
		t.Error("Expected nil for nil input")
	}
	if filterFeaturesUsed(map[string]bool{}) != nil {
		t.Error("Expected nil for empty input")
	}
}

func TestHandleSync_FeaturesUsedUnknownKeysFiltered(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	var mu sync.Mutex
	var receivedFeatures map[string]bool
	cap.FeatureUsage().SetCallback(func(features map[string]bool) {
		mu.Lock()
		receivedFeatures = features
		mu.Unlock()
	})

	req := SyncRequest{
		ExtSessionID: "allowlist_test",
		FeaturesUsed: map[string]bool{
			"screenshot": true,
			"evil_key":   true,
		},
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if _, ok := receivedFeatures["evil_key"]; ok {
		t.Error("Unknown key 'evil_key' should have been filtered before callback")
	}
	if !receivedFeatures["screenshot"] {
		t.Error("Known key 'screenshot' should have passed through")
	}
}

func TestHandleSync_ResponseContainsInstallID(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	req := SyncRequest{
		ExtSessionID: "install_id_test",
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	resp := decodeSyncResponse(t, w)
	if resp.InstallID == "" {
		t.Error("Expected install_id to be non-empty in sync response")
	}
}
