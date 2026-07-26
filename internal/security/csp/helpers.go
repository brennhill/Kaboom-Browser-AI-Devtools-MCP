// helpers.go — Directive shaping, confidence scoring and dev-pollution filtering.
// Purpose: Provides CSP helper logic for directive shaping, filtering, and policy assembly.
// Why: Keeps CSP generation behavior consistent and testable across strict/moderate/reporting modes.
// Docs: docs/features/feature/security-hardening/index.md

package csp

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// computeConfidence determines the confidence level for an origin entry.
// High: 3+ observations AND 2+ pages
// Medium: 2+ observations OR 2+ pages
// Low: exactly 1 observation on 1 page
// Exception: connect-src has relaxed threshold (1 observation = medium)
func (g *Generator) computeConfidence(entry *OriginEntry) string {
	pageCount := len(entry.Pages)
	obsCount := entry.Count

	if isHighConfidence(obsCount, pageCount) {
		return "high"
	}
	if resourceTypeToDirective[entry.ResourceType] == "connect-src" {
		return "medium"
	}
	if obsCount >= 2 || pageCount >= 2 {
		return "medium"
	}
	return "low"
}

func isHighConfidence(obsCount, pageCount int) bool {
	return obsCount >= 3 && pageCount >= 2
}

// extensionPrefixes maps browser extension URL prefixes to their filter reasons.
var extensionPrefixes = []struct {
	prefix string
	reason string
}{
	{"chrome-extension://", "Browser extension origin (auto-filtered)"},
	{"moz-extension://", "Firefox extension origin (auto-filtered)"},
}

// isDevPollution checks if an origin is a known development-only pattern.
// Returns the reason string if filtered, empty string if not filtered.
func (g *Generator) isDevPollution(origin string, pageOrigins map[string]bool) string {
	if reason := matchExtensionPrefix(origin); reason != "" {
		return reason
	}
	return g.checkLocalhostDevServer(origin, pageOrigins)
}

// matchExtensionPrefix returns a filter reason if the origin matches a known browser extension prefix.
func matchExtensionPrefix(origin string) string {
	for _, ext := range extensionPrefixes {
		if strings.HasPrefix(origin, ext.prefix) {
			return ext.reason
		}
	}
	return ""
}

// checkLocalhostDevServer filters localhost origins that differ from observed page origins.
func (g *Generator) checkLocalhostDevServer(origin string, pageOrigins map[string]bool) string {
	parsed, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	if host != "localhost" && host != "127.0.0.1" {
		return ""
	}
	if pageOrigins[origin] {
		return ""
	}
	return "Development server (auto-filtered, different port from app)"
}

// extractPageOrigins returns the set of origins from observed page URLs.
func (g *Generator) extractPageOrigins() map[string]bool {
	origins := make(map[string]bool)
	for pageURL := range g.pages {
		parsed, err := url.Parse(pageURL)
		if err != nil {
			continue
		}
		// Reconstruct origin: scheme://host[:port]
		origin := parsed.Scheme + "://" + parsed.Host
		origins[origin] = true
	}
	return origins
}

// directiveOrder defines the canonical order for CSP directives.
var directiveOrder = []string{
	"default-src",
	"script-src",
	"style-src",
	"img-src",
	"font-src",
	"connect-src",
	"frame-src",
	"media-src",
	"worker-src",
	"base-uri",
	"form-action",
	"frame-ancestors",
}

// directiveOrderSet is built from directiveOrder for O(1) membership checks.
var directiveOrderSet = buildDirectiveOrderSet()

func buildDirectiveOrderSet() map[string]bool {
	s := make(map[string]bool, len(directiveOrder))
	for _, d := range directiveOrder {
		s[d] = true
	}
	return s
}

// buildCSPHeader builds the CSP header string from sorted directives.
func (g *Generator) buildCSPHeader(directives map[string][]string) string {
	parts := make([]string, 0, len(directives))

	for _, dir := range directiveOrder {
		if sources, ok := directives[dir]; ok && len(sources) > 0 {
			parts = append(parts, dir+" "+strings.Join(sources, " "))
		}
	}

	for dir, sources := range directives {
		if !directiveOrderSet[dir] && len(sources) > 0 {
			parts = append(parts, dir+" "+strings.Join(sources, " "))
		}
	}

	return strings.Join(parts, "; ")
}

// directiveForResourceType returns the CSP directive name for a resource type,
// falling back to "default-src" for unknown types.
func directiveForResourceType(resourceType string) string {
	if d := resourceTypeToDirective[resourceType]; d != "" {
		return d
	}
	return "default-src"
}

// addDirectiveSource adds an origin to the given directive's source set.
func addDirectiveSource(directives map[string]map[string]bool, directive, origin string) {
	if directives[directive] == nil {
		directives[directive] = make(map[string]bool)
	}
	directives[directive][origin] = true
}

// sortDirectiveSources converts directive sets to sorted string slices.
func sortDirectiveSources(directives map[string]map[string]bool) map[string][]string {
	sorted := make(map[string][]string, len(directives))
	for dir, sources := range directives {
		list := make([]string, 0, len(sources))
		for src := range sources {
			list = append(list, src)
		}
		sort.Strings(list)
		sorted[dir] = list
	}
	return sorted
}

// formatMetaTag wraps a CSP header string in an HTML meta tag.
func formatMetaTag(cspHeader string) string {
	return fmt.Sprintf(`<meta http-equiv="Content-Security-Policy" content="%s">`, cspHeader)
}

// entryPages returns a sorted list of page URLs for an origin entry (up to 10).
func (g *Generator) entryPages(entry *OriginEntry) []string {
	var pages []string
	for p := range entry.Pages {
		pages = append(pages, p)
		if len(pages) >= 10 {
			break
		}
	}
	sort.Strings(pages)
	return pages
}

// countLowConfidenceExclusions returns the number of origin details excluded due to low confidence.
func countLowConfidenceExclusions(details []OriginDetail) int {
	count := 0
	for _, d := range details {
		if d.Confidence == "low" && !d.Included {
			count++
		}
	}
	return count
}

// exactContentTypeMap maps exact Content-Type values to CSP resource categories.
var exactContentTypeMap = map[string]string{
	"text/css":                "style",
	"application/x-font-ttf":  "font",
	"application/x-font-woff": "font",
}

// contentTypeToResourceType maps HTTP Content-Type to CSP resource category.
func contentTypeToResourceType(ct string) string {
	ct = strings.ToLower(ct)
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(ct)

	if res, ok := exactContentTypeMap[ct]; ok {
		return res
	}
	return contentTypePrefixMatch(ct)
}

// contentTypePrefixMatch classifies content types by prefix or substring patterns.
func contentTypePrefixMatch(ct string) string {
	if strings.Contains(ct, "javascript") {
		return "script"
	}
	if strings.HasPrefix(ct, "font/") || strings.Contains(ct, "application/font") {
		return "font"
	}
	if strings.HasPrefix(ct, "image/") {
		return "img"
	}
	if strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") {
		return "media"
	}
	return "connect"
}
