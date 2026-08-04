// handler.go — Serves the opt-in CSP-bypass debugging proxy.
// Why: Enables debugging CSP-restricted pages by proxying requests through the local server in explicit security_mode.
// Docs: docs/features/feature/csp-safe-execution/index.md

package insecureproxy

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
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

type JSONResponder func(http.ResponseWriter, int, any)

type Handler struct {
	capture       *capture.Capture
	respond       JSONResponder
	client        *http.Client
	responseLimit int64
}

func New(cap *capture.Capture, respond JSONResponder) *Handler {
	return &Handler{
		capture: cap,
		respond: respond,
		client: &http.Client{
			Timeout:   insecureProxyTimeout,
			Transport: uploadsec.NewSSRFSafeTransport(func() bool { return false }),
		},
		responseLimit: insecureProxyMaxResponseBytes,
	}
}

var errResponseTooLarge = errors.New("upstream response exceeds insecure proxy limit")

func stageResponseBody(body io.Reader, limit int64) ([]byte, error) {
	var staged bytes.Buffer
	read, err := io.Copy(&staged, io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if read > limit {
		return nil, errResponseTooLarge
	}
	return staged.Bytes(), nil
}

var insecureProxyStripHeaders = map[string]bool{
	"content-security-policy":             true,
	"content-security-policy-report-only": true,
	"x-content-security-policy":           true,
	"x-webkit-csp":                        true,
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respond(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	if h.capture == nil {
		h.respond(w, http.StatusInternalServerError, map[string]string{"error": "Capture unavailable"})
		return
	}

	mode, productionParity, rewrites := h.capture.Extension().GetSecurityMode()
	if mode != capture.SecurityModeInsecureProxy {
		h.respond(w, http.StatusForbidden, map[string]string{
			"error": "insecure proxy is disabled; enable configure(what='security_mode', mode='insecure_proxy', confirm=true)",
		})
		return
	}

	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		h.respond(w, http.StatusBadRequest, map[string]string{"error": "Missing target query parameter"})
		return
	}
	targetURL, err := url.Parse(target)
	if err != nil || targetURL.Host == "" || (targetURL.Scheme != "http" && targetURL.Scheme != "https") {
		h.respond(w, http.StatusBadRequest, map[string]string{"error": "Invalid target URL"})
		return
	}

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		h.respond(w, http.StatusBadRequest, map[string]string{"error": "Invalid target URL"})
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
	upstreamResp, err := h.client.Do(upstreamReq)
	if err != nil {
		if strings.Contains(err.Error(), "ssrf_blocked") {
			h.respond(w, http.StatusForbidden, map[string]string{"error": "Target URL resolves to private/internal network address"})
			return
		}
		h.respond(w, http.StatusBadGateway, map[string]string{"error": "Failed to fetch target URL"})
		return
	}
	defer upstreamResp.Body.Close() //nolint:errcheck
	if upstreamResp.ContentLength > h.responseLimit {
		h.respond(w, http.StatusBadGateway, map[string]string{"error": "Target response exceeds proxy size limit"})
		return
	}
	responseBody, err := stageResponseBody(upstreamResp.Body, h.responseLimit)
	if err != nil {
		message := "Failed to read target response"
		if errors.Is(err, errResponseTooLarge) {
			message = "Target response exceeds proxy size limit"
		}
		h.respond(w, http.StatusBadGateway, map[string]string{"error": message})
		return
	}

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
	_, _ = w.Write(responseBody)
}
