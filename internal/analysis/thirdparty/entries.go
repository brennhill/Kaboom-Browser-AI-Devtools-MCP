// Purpose: Builds per-origin third-party entries including resource, outbound, and risk details.
// Why: Isolates origin-level classification heuristics from high-level audit orchestration.
// Docs: docs/features/feature/enterprise-audit/index.md

package thirdparty

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// buildThirdPartyEntry creates a ThirdPartyEntry for a single origin.
func buildThirdPartyEntry(origin string, bodies []capture.NetworkBody, urls []string, customLists *CustomLists) ThirdPartyEntry {
	entry := ThirdPartyEntry{
		Origin:       origin,
		RequestCount: len(bodies),
	}

	countResources(&entry, bodies)
	collectOutboundData(&entry, bodies)
	classifyRiskLevel(&entry)

	hostname := extractHostname(origin)
	entry.Reputation = classifyReputation(hostname, customLists)
	if entry.Reputation.Classification == "enterprise_blocked" {
		entry.RiskLevel = "critical"
		entry.RiskReason = "domain is on enterprise blocked list"
	}

	urlLimit := 10
	if len(urls) < urlLimit {
		urlLimit = len(urls)
	}
	entry.URLs = urls[:urlLimit]
	return entry
}

// countResources counts resource types and detects cookie-setting from response headers.
func countResources(entry *ThirdPartyEntry, bodies []capture.NetworkBody) {
	for _, body := range bodies {
		switch contentTypeToResourceType(body.ContentType) {
		case "script":
			entry.Resources.Scripts++
		case "style":
			entry.Resources.Styles++
		case "font":
			entry.Resources.Fonts++
		case "image":
			entry.Resources.Images++
		default:
			entry.Resources.Other++
		}
		if !entry.SetsCookies && hasSetCookieHeader(body.ResponseHeaders) {
			entry.SetsCookies = true
		}
	}
}

// hasSetCookieHeader checks if response headers contain a Set-Cookie header.
func hasSetCookieHeader(headers map[string]string) bool {
	if headers == nil {
		return false
	}
	for key := range headers {
		if strings.EqualFold(key, "set-cookie") {
			return true
		}
	}
	return false
}

// collectOutboundData detects outbound data methods, content types, and PII.
func collectOutboundData(entry *ThirdPartyEntry, bodies []capture.NetworkBody) {
	methodSet := make(map[string]bool)
	ctSet := make(map[string]bool)
	var outboundMethods, outboundContentTypes, allPIIFields []string

	for _, body := range bodies {
		method := strings.ToUpper(body.Method)
		if method != "POST" && method != "PUT" && method != "PATCH" {
			continue
		}
		entry.DataOutbound = true
		if !methodSet[method] {
			outboundMethods = append(outboundMethods, method)
			methodSet[method] = true
		}
		if body.ContentType != "" && !ctSet[body.ContentType] {
			outboundContentTypes = append(outboundContentTypes, body.ContentType)
			ctSet[body.ContentType] = true
		}
		if body.RequestBody != "" {
			for _, field := range detectPIIFields(body.RequestBody) {
				allPIIFields = appendUnique(allPIIFields, field)
			}
		}
	}

	if entry.DataOutbound {
		entry.OutboundDetails = &OutboundDetails{
			Methods: outboundMethods, ContentTypes: outboundContentTypes, PIIFields: allPIIFields,
		}
	}
}

// classifyRiskLevel determines the risk level and reason based on resources and data flow.
func classifyRiskLevel(entry *ThirdPartyEntry) {
	hasScripts := entry.Resources.Scripts > 0
	hasOutbound := entry.DataOutbound
	hasCookies := entry.SetsCookies

	switch {
	case hasScripts && hasOutbound:
		entry.RiskLevel = "critical"
		entry.RiskReason = "loads executable scripts AND sends data outbound"
	case hasScripts:
		entry.RiskLevel = "high"
		entry.RiskReason = "loads executable scripts (can execute code in page context)"
	case hasOutbound && hasCookies:
		entry.RiskLevel = "medium"
		entry.RiskReason = "receives outbound data and sets cookies"
	case hasOutbound:
		entry.RiskLevel = "medium"
		entry.RiskReason = "receives outbound data"
	case hasCookies:
		entry.RiskLevel = "medium"
		entry.RiskReason = "sets cookies for tracking"
	default:
		entry.RiskLevel = "low"
		entry.RiskReason = "static assets only (images, fonts, styles)"
	}
}

// appendUnique appends a value to a slice if not already present.
func appendUnique(slice []string, val string) []string {
	for _, existing := range slice {
		if existing == val {
			return slice
		}
	}
	return append(slice, val)
}

// loadCustomListsFile reads and parses a CustomLists JSON file.
func loadCustomListsFile(path string) *CustomLists {
	data, err := os.ReadFile(path) // #nosec G304 -- path is from trusted config location
	if err != nil {
		return nil
	}
	var lists CustomLists
	if err := json.Unmarshal(data, &lists); err != nil {
		return nil
	}
	return &lists
}

// contentTypeToResourceType maps content-type to a resource category.
func contentTypeToResourceType(ct string) string {
	ct = strings.ToLower(ct)
	ct = strings.Split(ct, ";")[0]
	ct = strings.TrimSpace(ct)

	switch {
	case strings.HasPrefix(ct, "application/javascript"),
		strings.HasPrefix(ct, "text/javascript"):
		return "script"
	case strings.HasPrefix(ct, "text/css"):
		return "style"
	case strings.HasPrefix(ct, "image/"):
		return "image"
	case strings.HasPrefix(ct, "font/"),
		strings.Contains(ct, "woff"),
		strings.Contains(ct, "opentype"):
		return "font"
	case strings.HasPrefix(ct, "text/html"):
		return "document"
	case strings.HasPrefix(ct, "application/json"):
		return "fetch"
	case strings.Contains(ct, "form"):
		return "fetch"
	default:
		return "other"
	}
}

// detectPIIFields scans a request/response body for common PII patterns.
// Returns a list of detected PII field names (email, phone, ssn).
func detectPIIFields(body string) []string {
	var fields []string
	if piiEmailPattern.MatchString(body) {
		fields = append(fields, "email")
	}
	if piiPhonePattern.MatchString(body) {
		fields = append(fields, "phone")
	}
	if piiSSNPattern.MatchString(body) {
		fields = append(fields, "ssn")
	}
	return fields
}

// PII detection patterns for detectPIIFields.
var (
	piiEmailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	piiPhonePattern = regexp.MustCompile(`\b(\+?1[-.]?)?\(?[0-9]{3}\)?[-. ]?[0-9]{3}[-. ]?[0-9]{4}\b`)
	piiSSNPattern   = regexp.MustCompile(`\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`)
)
