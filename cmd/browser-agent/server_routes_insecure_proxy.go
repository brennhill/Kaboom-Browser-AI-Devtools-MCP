// Purpose: Serves an opt-in insecure proxy endpoint at /proxy for CSP-bypass debugging of third-party resources.
// Why: Enables debugging CSP-restricted pages by proxying requests through the local server in explicit security_mode.
// Docs: docs/features/feature/csp-safe-execution/index.md

package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

const (
	// insecureProxyTimeout is the HTTP client timeout for outbound requests
	// made through the insecure proxy endpoint.
	insecureProxyTimeout = 20 * time.Second

	// insecureProxyMaxResponseBytes caps the proxied response body size
	// to prevent memory exhaustion from unexpectedly large upstream responses.
	insecureProxyMaxResponseBytes = 50 * 1024 * 1024 // 50MB
)

// ssrfCheckEnabled controls whether the SSRF denylist is enforced.
// Disabled in tests that need to reach localhost mock servers.
var ssrfCheckEnabled = true

var (
	insecureProxyClient     *http.Client
	insecureProxyClientOnce sync.Once
)

func getInsecureProxyClient() *http.Client {
	insecureProxyClientOnce.Do(func() {
		insecureProxyClient = &http.Client{
			Timeout:   insecureProxyTimeout,
			Transport: uploadsec.NewSSRFSafeTransport(func() bool { return !ssrfCheckEnabled }),
		}
	})
	return insecureProxyClient
}

var insecureProxyStripHeaders = map[string]bool{
	"content-security-policy":             true,
	"content-security-policy-report-only": true,
	"x-content-security-policy":           true,
	"x-webkit-csp":                        true,
}

func (s *Server) handleInsecureProxy(w http.ResponseWriter, r *http.Request, cap *capture.Store) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	if cap == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Capture unavailable"})
		return
	}

	mode, productionParity, rewrites := cap.GetSecurityMode()
	if mode != capture.SecurityModeInsecureProxy {
		jsonResponse(w, http.StatusForbidden, map[string]string{
			"error": "insecure proxy is disabled; enable configure(what='security_mode', mode='insecure_proxy', confirm=true)",
		})
		return
	}

	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing target query parameter"})
		return
	}
	targetURL, err := url.Parse(target)
	if err != nil || targetURL.Host == "" || (targetURL.Scheme != "http" && targetURL.Scheme != "https") {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid target URL"})
		return
	}

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid target URL"})
		return
	}
	if accept := r.Header.Get("Accept"); accept != "" {
		upstreamReq.Header.Set("Accept", accept)
	}
	if ua := r.Header.Get("User-Agent"); ua != "" {
		upstreamReq.Header.Set("User-Agent", ua)
	}

	// Use pooled SSRF-safe client that pins DNS resolution at the dial layer,
	// preventing redirect-based SSRF bypasses and TOCTOU DNS rebinding.
	// Reuses the comprehensive denylist from internal/upload/ssrf.go.
	client := getInsecureProxyClient()
	upstreamResp, err := client.Do(upstreamReq)
	if err != nil {
		if strings.Contains(err.Error(), "ssrf_blocked") {
			jsonResponse(w, http.StatusForbidden, map[string]string{"error": "Target URL resolves to private/internal network address"})
			return
		}
		jsonResponse(w, http.StatusBadGateway, map[string]string{"error": "Failed to fetch target URL"})
		return
	}
	defer upstreamResp.Body.Close() //nolint:errcheck

	for key, values := range upstreamResp.Header {
		if insecureProxyStripHeaders[strings.ToLower(key)] {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("X-Kaboom-Proxy-Mode", mode)
	w.Header().Set("X-Kaboom-Production-Parity", "false")
	if len(rewrites) > 0 {
		w.Header().Set("X-Kaboom-Insecure-Rewrites", strings.Join(rewrites, ","))
	}
	if productionParity {
		w.Header().Set("X-Kaboom-Production-Parity", "true")
	}
	w.WriteHeader(upstreamResp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(upstreamResp.Body, insecureProxyMaxResponseBytes))
}
