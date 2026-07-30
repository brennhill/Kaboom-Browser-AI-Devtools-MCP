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

	if status, _ := handler.checkScreenshotRateLimit("new-client", ""); status != http.StatusServiceUnavailable {
		t.Fatalf("capacity status = %d, want %d", status, http.StatusServiceUnavailable)
	}

	handler.CleanupRateLimits(now.Add(2*time.Minute), time.Minute)
	if status, message := handler.checkScreenshotRateLimit("new-client", ""); status != 0 {
		t.Fatalf("status after cleanup = %d (%s), want allowed", status, message)
	}
}

func TestScreenshotRateLimiterAllowsDaemonCorrelatedQueries(t *testing.T) {
	handler := New(nil, nil, nil)

	if status, message := handler.checkScreenshotRateLimit("extension-client", "query-1"); status != 0 {
		t.Fatalf("first correlated screenshot status = %d (%s), want allowed", status, message)
	}
	if status, message := handler.checkScreenshotRateLimit("extension-client", "query-2"); status != 0 {
		t.Fatalf("second correlated screenshot status = %d (%s), want allowed", status, message)
	}
	if len(handler.rateByClient) != 0 {
		t.Fatalf("correlated screenshots consumed unsolicited upload quota: %+v", handler.rateByClient)
	}
}
