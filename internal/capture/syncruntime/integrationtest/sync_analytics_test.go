// sync_analytics_test.go — Tests for analytics fields in sync protocol:
// features_used callback, install_id in response.

package integrationtest

import (
	"encoding/json"
	. "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"net/http"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
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

	req := syncruntime.SyncRequest{
		ExtSessionID: "analytics_test",
		FeaturesUsed: &syncruntime.SyncFeaturesUsed{
			Screenshot:  true,
			Annotations: true,
			Video:       false,
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

	req := syncruntime.SyncRequest{
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

	req := syncruntime.SyncRequest{
		ExtSessionID: "analytics_test_no_cb",
		FeaturesUsed: &syncruntime.SyncFeaturesUsed{Screenshot: true},
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
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

	var req syncruntime.SyncRequest
	if err := json.Unmarshal([]byte(`{"ext_session_id":"allowlist_test","features_used":{"screenshot":true,"evil_key":true}}`), &req); err != nil {
		t.Fatalf("decode sync fixture: %v", err)
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

	req := syncruntime.SyncRequest{
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
