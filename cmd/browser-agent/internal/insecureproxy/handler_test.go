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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

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

func TestInsecureProxyRejectsDeclaredOversizedResponseBeforeReadingOrCommittingSuccess(t *testing.T) {
	t.Parallel()
	read := false
	cap := capture.NewCapture()
	cap.Extension().SetSecurityMode("insecure_proxy", nil)
	handler := New(cap, testRespond)
	handler.responseLimit = 4
	handler.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Content-Length": []string{"5"}, "X-Upstream": []string{"present"}},
			ContentLength: 5,
			Body:          &readTrackingBody{Reader: strings.NewReader("abcde"), read: &read},
		}, nil
	})}

	req := httptest.NewRequest(http.MethodGet, "/insecure-proxy?target=https://example.test/large", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rr.Code)
	}
	if read {
		t.Fatal("declared oversized body must be rejected before reading")
	}
	if rr.Header().Get("X-Upstream") != "" {
		t.Fatal("upstream headers must not be committed on rejection")
	}
}

func TestInsecureProxyRejectsChunkedOversizedResponseBeforeCommittingSuccess(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.Extension().SetSecurityMode("insecure_proxy", nil)
	handler := New(cap, testRespond)
	handler.responseLimit = 4
	handler.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"X-Upstream": []string{"present"}},
			ContentLength: -1,
			Body:          io.NopCloser(strings.NewReader("abcde")),
		}, nil
	})}

	req := httptest.NewRequest(http.MethodGet, "/insecure-proxy?target=https://example.test/chunked", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rr.Code)
	}
	if rr.Header().Get("X-Upstream") != "" {
		t.Fatal("upstream headers must not be committed on streamed rejection")
	}
	if strings.Contains(rr.Body.String(), "abcd") {
		t.Fatal("partial upstream body must never be returned")
	}
}

type readTrackingBody struct {
	*strings.Reader
	read *bool
}

func (body *readTrackingBody) Read(p []byte) (int, error) {
	*body.read = true
	return body.Reader.Read(p)
}

func (*readTrackingBody) Close() error { return nil }

func TestBuildUpstreamRequestCopiesOptionalHeaders(t *testing.T) {
	t.Parallel()
	target := &url.URL{Scheme: "https", Host: "example.test", Path: "/script.js"}

	bare := httptest.NewRequest(http.MethodGet, "/insecure-proxy", nil)
	upstream, err := buildUpstreamRequest(bare, target)
	if err != nil {
		t.Fatal(err)
	}
	if got := upstream.Header.Get("Accept"); got != "" {
		t.Fatalf("absent Accept must stay unset, got %q", got)
	}
	if got := upstream.Header.Get("User-Agent"); got != "" {
		t.Fatalf("absent User-Agent must stay unset, got %q", got)
	}

	enriched := httptest.NewRequest(http.MethodGet, "/insecure-proxy", nil)
	enriched.Header.Set("Accept", "text/css")
	enriched.Header.Set("User-Agent", "kaboom-test/1.0")
	upstream, err = buildUpstreamRequest(enriched, target)
	if err != nil {
		t.Fatal(err)
	}
	if got := upstream.Header.Get("Accept"); got != "text/css" {
		t.Fatalf("Accept = %q, want forwarded value", got)
	}
	if got := upstream.Header.Get("User-Agent"); got != "kaboom-test/1.0" {
		t.Fatalf("User-Agent = %q, want forwarded value", got)
	}
	if upstream.URL.String() != "https://example.test/script.js" {
		t.Fatalf("upstream URL = %q", upstream.URL.String())
	}
}

func TestBuildUpstreamRequestRejectsUnparseableTarget(t *testing.T) {
	t.Parallel()
	// A Host containing a space produces "http://a b", which url.Parse rejects;
	// this mirrors a caller bypassing parseTargetURL with a hand-built URL.
	invalid := &url.URL{Scheme: "http", Host: "in valid host"}
	req := httptest.NewRequest(http.MethodGet, "/insecure-proxy", nil)
	if _, err := buildUpstreamRequest(req, invalid); err == nil {
		t.Fatal("unparseable target must return an error, not a request")
	}
}
