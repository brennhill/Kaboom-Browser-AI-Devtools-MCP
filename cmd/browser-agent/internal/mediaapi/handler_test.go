// handler_test.go — Tests media-ingest rate-limit ownership and cleanup.

package mediaapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestHandleScreenshotValidatesPersistsResolvesAndThrottles(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	captured := capture.NewCapture()
	t.Cleanup(captured.Close)
	handler := New(captured, nil, nil)

	request := func(method, clientID, body string) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/screenshots", strings.NewReader(body))
		req.Header.Set("X-Kaboom-Client", clientID)
		handler.HandleScreenshot(recorder, req)
		return recorder
	}
	for _, test := range []struct {
		name   string
		method string
		body   string
		status int
	}{
		{"wrong method", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"invalid JSON", http.MethodPost, "{", http.StatusBadRequest},
		{"missing data URL", http.MethodPost, `{"url":"https://example.test"}`, http.StatusBadRequest},
		{"invalid data URL", http.MethodPost, `{"data_url":"not-a-data-url"}`, http.StatusBadRequest},
		{"invalid base64", http.MethodPost, `{"data_url":"data:image/jpeg;base64,%%%INVALID%%%"}`, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			if recorder := request(test.method, "client-"+test.name, test.body); recorder.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.status, recorder.Body.String())
			}
		})
	}

	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("abc123"))
	validBody := `{"data_url":"` + dataURL + `","url":"https://example.test/page","correlation_id":"corr-1","query_id":"query-1"}`
	valid := request(http.MethodPost, "query-client", validBody)
	if valid.Code != http.StatusOK {
		t.Fatalf("valid status = %d; body=%s", valid.Code, valid.Body.String())
	}
	var response map[string]string
	if err := json.NewDecoder(bytes.NewReader(valid.Body.Bytes())).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(response["path"]); err != nil || !strings.Contains(response["filename"], "example.test") {
		t.Fatalf("persisted response = %#v, stat error=%v", response, err)
	}
	if result, ok := captured.Queries().TakeQueryResult("query-1"); !ok || !bytes.Contains(result, []byte(dataURL)) {
		t.Fatalf("query result = %s, %t", result, ok)
	}

	unsolicited := `{"data_url":"` + dataURL + `","url":"https://example.test/page"}`
	if first := request(http.MethodPost, "rate-client", unsolicited); first.Code != http.StatusOK {
		t.Fatalf("first unsolicited status = %d", first.Code)
	}
	if second := request(http.MethodPost, "rate-client", unsolicited); second.Code != http.StatusTooManyRequests {
		t.Fatalf("second unsolicited status = %d, want 429", second.Code)
	}
}

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
