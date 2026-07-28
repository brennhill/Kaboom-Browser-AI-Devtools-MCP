// Purpose: Tests for insecure proxy request handling.
// Docs: docs/features/feature/csp-safe-execution/index.md

package insecureproxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func testRespond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func TestInsecureProxyEndpoint_SSRFDenylist(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	cap.Extension().SetSecurityMode("insecure_proxy", []string{"csp_headers"})
	handler := New(cap, testRespond)

	// Test various private/internal IP ranges.
	tests := []struct {
		name   string
		target string
	}{
		{"cloud_metadata", "http://169.254.169.254/latest/meta-data/"},
		{"loopback", "http://127.0.0.1:8080/secret"},
		{"private_10", "http://10.0.0.1/internal"},
		{"private_172", "http://172.16.0.1/internal"},
		{"private_192", "http://192.168.1.1/internal"},
		{"ipv6_loopback", "http://[::1]:8080/secret"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/insecure-proxy?target="+url.QueryEscape(tc.target), nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("GET /insecure-proxy to %s status = %d, want 403", tc.target, rr.Code)
			}
		})
	}
}

func TestInsecureProxyEndpoint_StripsCSPHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Content-Security-Policy-Report-Only", "default-src 'none'")
		w.Header().Set("X-Frame-Options", "DENY")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>fixture</body></html>"))
	}))
	defer upstream.Close()

	cap := capture.NewCapture()
	cap.Extension().SetSecurityMode("insecure_proxy", []string{"csp_headers"})
	handler := New(cap, testRespond)
	handler.client = upstream.Client()

	req := httptest.NewRequest(http.MethodGet, "/insecure-proxy?target="+url.QueryEscape(upstream.URL), nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /insecure-proxy status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("CSP header should be stripped, got %q", got)
	}
	if got := rr.Header().Get("Content-Security-Policy-Report-Only"); got != "" {
		t.Fatalf("CSP report-only header should be stripped, got %q", got)
	}
	if got := rr.Header().Get("X-Kaboom-Proxy-Mode"); got != "insecure_proxy" {
		t.Fatalf("X-Kaboom-Proxy-Mode = %q, want insecure_proxy", got)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "fixture") {
		t.Fatalf("proxy body should include upstream content, got: %s", string(body))
	}
}

func TestInsecureProxyEndpoint_RequiresInsecureMode(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.Extension().SetSecurityMode("normal", nil)
	handler := New(cap, testRespond)

	req := httptest.NewRequest(http.MethodGet, "/insecure-proxy?target="+url.QueryEscape("https://example.com"), nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("GET /insecure-proxy status = %d, want 403 when mode is normal", rr.Code)
	}
}
