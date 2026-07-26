// Purpose: Normalizes API endpoints into stable keys for contract tracking.
// Why: Ensures dynamic IDs do not fragment endpoint-level learning and validation.
// Docs: docs/features/feature/api-schema/index.md

package apicontract

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	contractUUIDPattern    = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	contractNumericPattern = regexp.MustCompile(`^\d+$`)
	contractHexPattern     = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
)

// normalizeEndpoint converts a METHOD + URL into a normalized endpoint key.
// Dynamic segments (numeric IDs, UUIDs, hex hashes) are replaced with {id}.
// Query parameters are stripped.
func normalizeEndpoint(method, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return method + " " + rawURL
	}

	path := parsed.Path
	if path == "" {
		path = "/"
	}

	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		if contractUUIDPattern.MatchString(seg) {
			segments[i] = "{id}"
			continue
		}
		if contractNumericPattern.MatchString(seg) {
			segments[i] = "{id}"
			continue
		}
		if contractHexPattern.MatchString(seg) {
			segments[i] = "{id}"
		}
	}

	normalizedPath := strings.Join(segments, "/")
	return method + " " + normalizedPath
}
