// Purpose: Builds Content-Security-Policy directive maps from captured network body origins.
// Docs: docs/features/feature/test-generation/index.md

package generate

import (
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// BuildCSPDirectives extracts unique origins from network bodies and groups them by CSP directive.
func BuildCSPDirectives(networkBodies []capture.NetworkBody) map[string][]string {
	originsByType := make(map[string]map[string]bool)
	for _, body := range networkBodies {
		origin := ExtractOrigin(body.URL)
		if origin == "" {
			continue
		}
		directive := resourceTypeToCSPDirective(body.ContentType)
		if originsByType[directive] == nil {
			originsByType[directive] = make(map[string]bool)
		}
		originsByType[directive][origin] = true
	}

	directives := map[string][]string{"default-src": {"'self'"}}
	for directive, origins := range originsByType {
		originList := make([]string, 0, len(origins))
		for origin := range origins {
			originList = append(originList, origin)
		}
		if len(originList) > 0 {
			// Sort: Go randomizes map iteration, so without this the origins
			// inside a directive come out in a different order every call for
			// identical input, making the generated policy undiffable.
			// 'self' is prepended after sorting so it stays pinned at the front.
			sort.Strings(originList)
			directives[directive] = append([]string{"'self'"}, originList...)
		}
	}
	return directives
}

// BuildCSPPolicyString serializes CSP directives into a semicolon-separated policy string.
//
// Directive order is stable: default-src first (it is the fallback every other
// directive narrows, so a reader should meet it first), then the rest
// alphabetically. Ranging the map directly produced a different order on every
// call for identical input, which made the output undiffable across runs and
// impossible to pin with a golden test.
func BuildCSPPolicyString(directives map[string][]string) string {
	names := make([]string, 0, len(directives))
	for directive := range directives {
		names = append(names, directive)
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == "default-src" != (names[j] == "default-src") {
			return names[i] == "default-src"
		}
		return names[i] < names[j]
	})

	policyParts := make([]string, 0, len(names))
	for _, directive := range names {
		policyParts = append(policyParts, directive+" "+strings.Join(directives[directive], " "))
	}
	return strings.Join(policyParts, "; ")
}

// ExtractOrigin extracts the origin (scheme://host:port) from a URL.
func ExtractOrigin(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	idx := 0
	if len(urlStr) > 8 && urlStr[:8] == "https://" {
		idx = 8
	} else if len(urlStr) > 7 && urlStr[:7] == "http://" {
		idx = 7
	} else {
		return ""
	}
	endIdx := idx
	for endIdx < len(urlStr) && urlStr[endIdx] != '/' && urlStr[endIdx] != '?' {
		endIdx++
	}
	return urlStr[:endIdx]
}

// resourceTypeToCSPDirective maps content-type to CSP directive.
func resourceTypeToCSPDirective(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "javascript"):
		return "script-src"
	case strings.Contains(ct, "css"):
		return "style-src"
	case strings.Contains(ct, "font"):
		return "font-src"
	case strings.Contains(ct, "image"):
		return "img-src"
	case strings.Contains(ct, "video"), strings.Contains(ct, "audio"):
		return "media-src"
	default:
		return "connect-src"
	}
}
