// Purpose: Tests for capture HTTP debug logging with redaction.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
	"time"
)

func TestLogHTTPDebugEntry_RedactsSensitiveFields(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	const (
		bearer = "Bearer tokenValue1234567890abcdef"
		ghPAT  = "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef123456"
	)

	c.LogHTTPDebugEntry(types.HTTPDebugEntry{
		Timestamp: time.Now(),
		Endpoint:  "/mcp",
		Method:    "POST",
		Headers: map[string]string{
			"X-Auth-Token": bearer,
			"X-Custom":     "api_key=supersecret1234567890",
		},
		RequestBody:  `{"auth":"` + bearer + `","token":"` + ghPAT + `"}`,
		ResponseBody: `{"ok":true,"token":"` + ghPAT + `"}`,
		Error:        "request failed with " + bearer,
		DurationMs:   10,
	})

	entries := c.GetHTTPDebugLog()
	var found *types.HTTPDebugEntry
	for i := range entries {
		if entries[i].Method == "POST" && entries[i].Endpoint == "/mcp" {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected redacted HTTP debug entry")
	}

	if strings.Contains(found.RequestBody, bearer) || strings.Contains(found.ResponseBody, ghPAT) || strings.Contains(found.Error, bearer) {
		t.Fatalf("expected sensitive values to be redacted: %+v", *found)
	}
	if strings.Contains(found.Headers["X-Auth-Token"], bearer) || strings.Contains(found.Headers["X-Custom"], "supersecret1234567890") {
		t.Fatalf("expected sensitive headers to be redacted: %+v", found.Headers)
	}

	if !strings.Contains(found.RequestBody, "[REDACTED:bearer-token]") {
		t.Fatalf("expected bearer-token marker in request body, got %q", found.RequestBody)
	}
	if !strings.Contains(found.ResponseBody, "[REDACTED:github-pat]") {
		t.Fatalf("expected github-pat marker in response body, got %q", found.ResponseBody)
	}
}
