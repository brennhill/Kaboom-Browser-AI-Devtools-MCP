// handler_test.go — Tests media-ingest rate-limit ownership and cleanup.

package mediaapi

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestScreenshotRateLimiterCapacityAndCleanup(t *testing.T) {
	now := time.Now()
	handler := New(nil, nil, nil)
	for i := 0; i < 10000; i++ {
		handler.rateByClient["client-"+strconv.Itoa(i)] = now
	}

	if status, _ := handler.checkScreenshotRateLimit("new-client"); status != http.StatusServiceUnavailable {
		t.Fatalf("capacity status = %d, want %d", status, http.StatusServiceUnavailable)
	}

	handler.CleanupRateLimits(now.Add(2*time.Minute), time.Minute)
	if status, message := handler.checkScreenshotRateLimit("new-client"); status != 0 {
		t.Fatalf("status after cleanup = %d (%s), want allowed", status, message)
	}
}
