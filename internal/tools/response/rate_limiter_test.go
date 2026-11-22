// rate_limiter_test.go — Tests sliding-window tool-call limiting.

package toolresp

import (
	"testing"
	"time"
)

func TestToolCallLimiterRejectsCallsPastLimit(t *testing.T) {
	t.Parallel()
	limiter := NewToolCallLimiter(2, time.Minute)
	if !limiter.Allow() || !limiter.Allow() {
		t.Fatal("first two calls should be allowed")
	}
	if limiter.Allow() {
		t.Fatal("third call should be rejected")
	}
}
