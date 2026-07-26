// Purpose: Tests for third-party script origin analysis.
// Docs: docs/features/feature/enterprise-audit/index.md

package thirdparty

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// NetworkBody keeps the test tables terse; the audit API takes capture.NetworkBody.
type NetworkBody = capture.NetworkBody

func TestThirdPartyBasicClassification(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		// Same-origin request — should NOT appear in results
		{URL: "https://myapp.com/api/data", ContentType: "application/json", Method: "GET"},
		// Third-party request — should appear
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	if len(result.ThirdParties) != 1 {
		t.Fatalf("expected 1 third party, got %d", len(result.ThirdParties))
	}
	if result.ThirdParties[0].Origin != "https://cdn.example.com" {
		t.Errorf("expected origin https://cdn.example.com, got %s", result.ThirdParties[0].Origin)
	}
	if result.FirstPartyOrigin != "https://myapp.com" {
		t.Errorf("expected first party origin https://myapp.com, got %s", result.FirstPartyOrigin)
	}
}

func TestThirdPartyRiskLevels(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		// Script from third party = high risk
		{URL: "https://cdn.evil.com/malware.js", ContentType: "application/javascript", Method: "GET"},
		// Data sent to third party (POST, no scripts) = medium risk
		{URL: "https://analytics.example.com/track", ContentType: "application/json", Method: "POST", RequestBody: `{"event":"click"}`},
		// Script AND data from same origin = critical risk
		{URL: "https://sketchy.net/widget.js", ContentType: "application/javascript", Method: "GET"},
		{URL: "https://sketchy.net/collect", ContentType: "application/json", Method: "POST", RequestBody: `{"user":"test"}`},
		// Static image only = low risk
		{URL: "https://images.example.com/logo.png", ContentType: "image/png", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	riskByOrigin := make(map[string]string)
	for _, entry := range result.ThirdParties {
		riskByOrigin[entry.Origin] = entry.RiskLevel
	}

	if riskByOrigin["https://cdn.evil.com"] != "high" {
		t.Errorf("expected cdn.evil.com risk=high, got %s", riskByOrigin["https://cdn.evil.com"])
	}
	if riskByOrigin["https://analytics.example.com"] != "medium" {
		t.Errorf("expected analytics.example.com risk=medium, got %s", riskByOrigin["https://analytics.example.com"])
	}
	if riskByOrigin["https://sketchy.net"] != "critical" {
		t.Errorf("expected sketchy.net risk=critical, got %s", riskByOrigin["https://sketchy.net"])
	}
	if riskByOrigin["https://images.example.com"] != "low" {
		t.Errorf("expected images.example.com risk=low, got %s", riskByOrigin["https://images.example.com"])
	}
}

func TestThirdPartyPIIDetection(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		{
			URL:         "https://tracker.example.com/submit",
			ContentType: "application/json",
			Method:      "POST",
			RequestBody: `{"email":"user@example.com","name":"John","phone":"555-123-4567"}`,
		},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	if len(result.ThirdParties) != 1 {
		t.Fatalf("expected 1 third party, got %d", len(result.ThirdParties))
	}
	entry := result.ThirdParties[0]
	if !entry.DataOutbound {
		t.Error("expected DataOutbound=true")
	}
	if entry.OutboundDetails == nil {
		t.Fatal("expected OutboundDetails to be set")
	}
	if len(entry.OutboundDetails.PIIFields) == 0 {
		t.Error("expected PII fields to be detected")
	}
	// Check that email is detected
	found := false
	for _, f := range entry.OutboundDetails.PIIFields {
		if f == "email" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'email' in PIIFields, got %v", entry.OutboundDetails.PIIFields)
	}
}

func TestThirdPartyCookieDetection(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		{
			URL:         "https://tracker.example.com/pixel.gif",
			ContentType: "image/gif",
			Method:      "GET",
			ResponseHeaders: map[string]string{
				"Set-Cookie": "tracking_id=abc123; Path=/; HttpOnly",
			},
		},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	if len(result.ThirdParties) != 1 {
		t.Fatalf("expected 1 third party, got %d", len(result.ThirdParties))
	}
	if !result.ThirdParties[0].SetsCookies {
		t.Error("expected SetsCookies=true")
	}
	// Image with cookie sets = medium risk (sets cookies)
	if result.ThirdParties[0].RiskLevel != "medium" {
		t.Errorf("expected risk=medium for cookie-setting image, got %s", result.ThirdParties[0].RiskLevel)
	}
}

func TestThirdPartyReputationKnownCDN(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		{URL: "https://cdn.jsdelivr.net/npm/vue@3/dist/vue.js", ContentType: "application/javascript", Method: "GET"},
		{URL: "https://fonts.googleapis.com/css2?family=Roboto", ContentType: "text/css", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	for _, entry := range result.ThirdParties {
		if entry.Reputation.Classification != "known_cdn" {
			t.Errorf("expected %s to be classified as known_cdn, got %s", entry.Origin, entry.Reputation.Classification)
		}
	}
}

func TestThirdPartyReputationAbuseTLD(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		{URL: "https://tracker.xyz/pixel.gif", ContentType: "image/gif", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	if len(result.ThirdParties) != 1 {
		t.Fatalf("expected 1 third party, got %d", len(result.ThirdParties))
	}
	entry := result.ThirdParties[0]
	if entry.Reputation.Classification != "suspicious" {
		t.Errorf("expected classification=suspicious for .xyz TLD, got %s", entry.Reputation.Classification)
	}
	foundFlag := false
	for _, flag := range entry.Reputation.SuspicionFlags {
		if flag == "abuse_tld" {
			foundFlag = true
			break
		}
	}
	if !foundFlag {
		t.Errorf("expected 'abuse_tld' in SuspicionFlags, got %v", entry.Reputation.SuspicionFlags)
	}
}

func TestThirdPartyReputationDGA(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	// Use a high-entropy subdomain that looks like DGA output (12+ unique chars, entropy > 3.5)
	bodies := []NetworkBody{
		{URL: "https://xk7q9mzp3fab.example.com/beacon.gif", ContentType: "image/gif", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	if len(result.ThirdParties) != 1 {
		t.Fatalf("expected 1 third party, got %d", len(result.ThirdParties))
	}
	entry := result.ThirdParties[0]
	if entry.Reputation.Classification != "suspicious" {
		t.Errorf("expected classification=suspicious for DGA domain, got %s", entry.Reputation.Classification)
	}
	foundFlag := false
	for _, flag := range entry.Reputation.SuspicionFlags {
		if flag == "possible_dga" {
			foundFlag = true
			break
		}
	}
	if !foundFlag {
		t.Errorf("expected 'possible_dga' in SuspicionFlags, got %v", entry.Reputation.SuspicionFlags)
	}
}

func TestThirdPartyCustomLists(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		// Allowed domain
		{URL: "https://trusted-cdn.corp.com/lib.js", ContentType: "application/javascript", Method: "GET"},
		// Blocked domain
		{URL: "https://evil-tracker.com/track.js", ContentType: "application/javascript", Method: "GET"},
		// Internal domain (should be treated as first-party and excluded)
		{URL: "https://internal.corp.com/api/data", ContentType: "application/json", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}
	customLists := &CustomLists{
		Allowed:  []string{"trusted-cdn.corp.com"},
		Blocked:  []string{"evil-tracker.com"},
		Internal: []string{"https://internal.corp.com"},
	}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{CustomLists: customLists})

	// Internal should be excluded as first-party
	for _, entry := range result.ThirdParties {
		if entry.Origin == "https://internal.corp.com" {
			t.Error("internal.corp.com should be treated as first-party and excluded")
		}
	}

	// Allowed domain should have enterprise_allowed reputation
	for _, entry := range result.ThirdParties {
		if entry.Origin == "https://trusted-cdn.corp.com" {
			if entry.Reputation.Classification != "enterprise_allowed" {
				t.Errorf("expected trusted-cdn.corp.com classification=enterprise_allowed, got %s", entry.Reputation.Classification)
			}
		}
	}

	// Blocked domain should have enterprise_blocked reputation and critical risk
	for _, entry := range result.ThirdParties {
		if entry.Origin == "https://evil-tracker.com" {
			if entry.Reputation.Classification != "enterprise_blocked" {
				t.Errorf("expected evil-tracker.com classification=enterprise_blocked, got %s", entry.Reputation.Classification)
			}
			if entry.RiskLevel != "critical" {
				t.Errorf("expected evil-tracker.com risk=critical (blocked), got %s", entry.RiskLevel)
			}
		}
	}
}

func TestThirdPartyRecommendations(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		// Suspicious origin with scripts = should generate critical recommendation
		{URL: "https://xk7q9mzp3fab.evil.xyz/widget.js", ContentType: "application/javascript", Method: "GET"},
		// Origin receiving data with PII
		{URL: "https://analytics.example.com/collect", ContentType: "application/json", Method: "POST", RequestBody: `{"email":"user@test.com"}`},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	if len(result.Recommendations) == 0 {
		t.Error("expected recommendations to be generated for critical findings")
	}

	// Should have recommendation about data receivers
	foundDataRec := false
	for _, rec := range result.Recommendations {
		if containsSubstring(rec, "receive") || containsSubstring(rec, "data") {
			foundDataRec = true
			break
		}
	}
	if !foundDataRec {
		t.Errorf("expected recommendation about data receivers, got %v", result.Recommendations)
	}
}

func TestThirdPartySummary(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		// Critical: scripts + outbound
		{URL: "https://sketchy.net/widget.js", ContentType: "application/javascript", Method: "GET"},
		{URL: "https://sketchy.net/collect", ContentType: "application/json", Method: "POST", RequestBody: `{"data":"test"}`},
		// High: scripts only
		{URL: "https://cdn.evil.com/tracker.js", ContentType: "application/javascript", Method: "GET"},
		// Medium: cookies
		{URL: "https://ad.example.com/pixel.gif", ContentType: "image/gif", Method: "GET", ResponseHeaders: map[string]string{"Set-Cookie": "id=123"}},
		// Low: static image
		{URL: "https://images.example.com/logo.png", ContentType: "image/png", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	if result.Summary.TotalThirdParties != 4 {
		t.Errorf("expected 4 total third parties, got %d", result.Summary.TotalThirdParties)
	}
	if result.Summary.CriticalRisk != 1 {
		t.Errorf("expected 1 critical risk, got %d", result.Summary.CriticalRisk)
	}
	if result.Summary.HighRisk != 1 {
		t.Errorf("expected 1 high risk, got %d", result.Summary.HighRisk)
	}
	if result.Summary.MediumRisk != 1 {
		t.Errorf("expected 1 medium risk, got %d", result.Summary.MediumRisk)
	}
	if result.Summary.LowRisk != 1 {
		t.Errorf("expected 1 low risk, got %d", result.Summary.LowRisk)
	}
	if result.Summary.OriginsReceivingData != 1 {
		t.Errorf("expected 1 origin receiving data, got %d", result.Summary.OriginsReceivingData)
	}
	if result.Summary.OriginsSettingCookies != 1 {
		t.Errorf("expected 1 origin setting cookies, got %d", result.Summary.OriginsSettingCookies)
	}
	if result.Summary.ScriptsFromThirdParty != 2 {
		t.Errorf("expected 2 origins with scripts, got %d", result.Summary.ScriptsFromThirdParty)
	}
}

func TestThirdPartyHandleMCP(t *testing.T) {
	t.Parallel()
	bodies := []NetworkBody{
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}

	params := ThirdPartyParams{}
	raw, _ := json.Marshal(params)

	result, err := HandleAuditThirdParties(json.RawMessage(raw), bodies, pageURLs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tpResult, ok := result.(ThirdPartyResult)
	if !ok {
		t.Fatal("expected ThirdPartyResult type")
	}
	if len(tpResult.ThirdParties) != 1 {
		t.Errorf("expected 1 third party, got %d", len(tpResult.ThirdParties))
	}
}

func TestThirdPartyFirstPartyOrigins(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		{URL: "https://api.myapp.com/data", ContentType: "application/json", Method: "GET"},
		{URL: "https://cdn.example.com/lib.js", ContentType: "application/javascript", Method: "GET"},
	}
	// Explicit first-party origins override page URL extraction
	params := ThirdPartyParams{
		FirstPartyOrigins: []string{"https://myapp.com", "https://api.myapp.com"},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, params)

	// api.myapp.com should be treated as first-party
	for _, entry := range result.ThirdParties {
		if entry.Origin == "https://api.myapp.com" {
			t.Error("api.myapp.com should be treated as first-party")
		}
	}
	if len(result.ThirdParties) != 1 {
		t.Errorf("expected 1 third party, got %d", len(result.ThirdParties))
	}
}

func TestThirdPartyResourceCounts(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", Method: "GET"},
		{URL: "https://cdn.example.com/style.css", ContentType: "text/css", Method: "GET"},
		{URL: "https://cdn.example.com/font.woff2", ContentType: "font/woff2", Method: "GET"},
		{URL: "https://cdn.example.com/logo.png", ContentType: "image/png", Method: "GET"},
		{URL: "https://cdn.example.com/data.json", ContentType: "application/json", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	if len(result.ThirdParties) != 1 {
		t.Fatalf("expected 1 third party, got %d", len(result.ThirdParties))
	}
	entry := result.ThirdParties[0]
	if entry.Resources.Scripts != 1 {
		t.Errorf("expected 1 script, got %d", entry.Resources.Scripts)
	}
	if entry.Resources.Styles != 1 {
		t.Errorf("expected 1 style, got %d", entry.Resources.Styles)
	}
	if entry.Resources.Fonts != 1 {
		t.Errorf("expected 1 font, got %d", entry.Resources.Fonts)
	}
	if entry.Resources.Images != 1 {
		t.Errorf("expected 1 image, got %d", entry.Resources.Images)
	}
	if entry.Resources.Other != 1 {
		t.Errorf("expected 1 other, got %d", entry.Resources.Other)
	}
	if entry.RequestCount != 5 {
		t.Errorf("expected 5 requests, got %d", entry.RequestCount)
	}
}

func TestThirdPartyURLLimit(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := make([]NetworkBody, 15)
	for i := 0; i < 15; i++ {
		bodies[i] = NetworkBody{
			URL:         "https://cdn.example.com/file" + string(rune('a'+i)) + ".js",
			ContentType: "application/javascript",
			Method:      "GET",
		}
	}
	pageURLs := []string{"https://myapp.com/"}
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{})

	if len(result.ThirdParties) != 1 {
		t.Fatalf("expected 1 third party, got %d", len(result.ThirdParties))
	}
	if len(result.ThirdParties[0].URLs) > 10 {
		t.Errorf("expected at most 10 URLs, got %d", len(result.ThirdParties[0].URLs))
	}
}

func TestThirdPartyShannonEntropy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		wantHigh bool // entropy > 3.5
	}{
		{"aaaa", false},         // Low entropy, repetitive
		{"abcdefghijklm", true}, // High entropy, 13 unique chars (log2(13)=3.7)
		{"xk7q9mzp3fab", true},  // DGA-like, 12 unique chars (log2(12)=3.58)
		{"www", false},          // Low entropy
		{"cdn", false},          // Short, low entropy
		{"abcdefghij", false},   // 10 unique chars, entropy=log2(10)=3.32, below 3.5
	}
	for _, tc := range tests {
		ent := shannonEntropy(tc.input)
		isHigh := ent > 3.5
		if isHigh != tc.wantHigh {
			t.Errorf("shannonEntropy(%q) = %.4f, high=%v, want high=%v", tc.input, ent, isHigh, tc.wantHigh)
		}
	}
}

func TestThirdPartyIncludeStaticFalse(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()
	bodies := []NetworkBody{
		{URL: "https://cdn.example.com/app.js", ContentType: "application/javascript", Method: "GET"},
		{URL: "https://images.example.com/logo.png", ContentType: "image/png", Method: "GET"},
	}
	pageURLs := []string{"https://myapp.com/"}
	includeStatic := false
	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{IncludeStatic: &includeStatic})

	// Only the script origin should be included when include_static is false
	if len(result.ThirdParties) != 1 {
		t.Fatalf("expected 1 third party with include_static=false, got %d", len(result.ThirdParties))
	}
	if result.ThirdParties[0].Origin != "https://cdn.example.com" {
		t.Errorf("expected cdn.example.com, got %s", result.ThirdParties[0].Origin)
	}
}

// containsSubstring checks if s contains substr (case-insensitive).
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsCI(s, substr))
}

func containsCI(s, substr string) bool {
	sLower := toLower(s)
	subLower := toLower(substr)
	for i := 0; i <= len(sLower)-len(subLower); i++ {
		if sLower[i:i+len(subLower)] == subLower {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func TestThirdPartyHelperEdgeCases(t *testing.T) {
	t.Parallel()
	// extractHostname with invalid URL
	got := extractHostname("://invalid")
	if got != "" {
		t.Errorf("expected empty for invalid URL, got %q", got)
	}

	// extractTLD with invalid URL
	got = extractTLD("://invalid")
	if got != "" {
		t.Errorf("expected empty for invalid URL, got %q", got)
	}

	// appendUnique with duplicates
	slice := []string{"a", "b"}
	slice = appendUnique(slice, "b")
	if len(slice) != 2 {
		t.Errorf("expected no duplicate, got length %d", len(slice))
	}
	slice = appendUnique(slice, "c")
	if len(slice) != 3 {
		t.Errorf("expected append, got length %d", len(slice))
	}

	// riskOrder for all levels
	levels := []string{"critical", "high", "medium", "low", "unknown"}
	for i, l := range levels {
		if riskOrder(l) != i {
			t.Errorf("expected riskOrder(%q) = %d, got %d", l, i, riskOrder(l))
		}
	}
}

func TestLoadCustomListsFile(t *testing.T) {
	t.Parallel()
	// Non-existent file returns nil
	result := loadCustomListsFile("/nonexistent/path.json")
	if result != nil {
		t.Error("expected nil for non-existent file")
	}

	// Invalid JSON returns nil
	tmpInvalid := t.TempDir() + "/invalid.json"
	os.WriteFile(tmpInvalid, []byte(`{not json}`), 0600)
	result = loadCustomListsFile(tmpInvalid)
	if result != nil {
		t.Error("expected nil for invalid JSON file")
	}

	// Valid JSON is parsed
	tmpValid := t.TempDir() + "/valid.json"
	os.WriteFile(tmpValid, []byte(`{"allowed":["cdn.example.com"],"blocked":["evil.xyz"]}`), 0600)
	result = loadCustomListsFile(tmpValid)
	if result == nil {
		t.Fatal("expected non-nil for valid JSON file")
	}
	if len(result.Allowed) != 1 || result.Allowed[0] != "cdn.example.com" {
		t.Errorf("unexpected allowed list: %v", result.Allowed)
	}
	if len(result.Blocked) != 1 || result.Blocked[0] != "evil.xyz" {
		t.Errorf("unexpected blocked list: %v", result.Blocked)
	}
}

func TestHandleAuditThirdPartiesHandler(t *testing.T) {
	t.Parallel()
	bodies := []NetworkBody{
		{URL: "https://myapp.com/page", ContentType: "text/html", Status: 200},
		{URL: "https://cdn.jsdelivr.net/lib.js", ContentType: "application/javascript", Status: 200},
	}
	pageURLs := []string{"https://myapp.com/page"}

	// Valid params
	params := []byte(`{"first_party_origins":["https://myapp.com"]}`)
	result, err := HandleAuditThirdParties(params, bodies, pageURLs)
	if err != nil {
		t.Fatalf("HandleAuditThirdParties failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Invalid params
	_, err = HandleAuditThirdParties([]byte(`{invalid}`), bodies, pageURLs)
	if err == nil {
		t.Error("expected error for invalid JSON params")
	}

	// Empty params (should use defaults)
	result, err = HandleAuditThirdParties(nil, bodies, pageURLs)
	if err != nil {
		t.Fatalf("HandleAuditThirdParties with nil params failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result with nil params")
	}
}

func TestThirdPartyCustomListsFromFile(t *testing.T) {
	t.Parallel()
	auditor := NewThirdPartyAuditor()

	tmpFile := t.TempDir() + "/lists.json"
	os.WriteFile(tmpFile, []byte(`{"allowed":["trusted-cdn.com"],"blocked":["blocked-site.com"]}`), 0600)

	bodies := []NetworkBody{
		{URL: "https://myapp.com/page", ContentType: "text/html", Status: 200},
		{URL: "https://trusted-cdn.com/lib.js", ContentType: "application/javascript", Status: 200},
		{URL: "https://blocked-site.com/track.js", ContentType: "application/javascript", Status: 200},
	}
	pageURLs := []string{"https://myapp.com/page"}

	result := auditor.Audit(bodies, pageURLs, ThirdPartyParams{
		FirstPartyOrigins: []string{"https://myapp.com"},
		CustomListsFile:   tmpFile,
	})

	resultJSON, _ := json.Marshal(result)
	resultStr := string(resultJSON)

	// Trusted CDN should have known_cdn or trusted reputation
	if !strings.Contains(resultStr, "trusted-cdn.com") {
		t.Error("expected trusted-cdn.com in results")
	}
}
