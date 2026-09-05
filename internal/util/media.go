// media.go — Owns shared media, string, map, and URL normalization helpers.
package util

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// SanitizeForFilename replaces unsafe characters with underscores and truncates to 50 chars.
func SanitizeForFilename(s string) string {
	s = unsafeFilenameChars.ReplaceAllString(s, "_")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

// SplitDataURL separates a data URL into its base64 payload and MIME type without decoding it.
// Example: "data:image/png;base64,iVBORw0..." -> ("iVBORw0...", "image/png").
// Returns empty strings when the data URL is malformed, so callers can skip the image rather
// than emit a content block Claude will reject.
func SplitDataURL(dataURL string) (base64Data, mimeType string) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", ""
	}
	rest := dataURL[len("data:"):]
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return "", ""
	}
	mimeType = rest[:semicolonIdx]
	rest = rest[semicolonIdx+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return "", ""
	}
	return rest[len("base64,"):], mimeType
}

// DecodeDataURL extracts and base64-decodes the payload from a data URL.
func DecodeDataURL(dataURL string) ([]byte, error) {
	if dataURL == "" {
		return nil, errors.New("missing dataUrl")
	}
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid dataUrl format")
	}
	return base64.StdEncoding.DecodeString(parts[1])
}

// BuildScreenshotFilename creates a timestamped filename from a page URL and optional correlation ID.
func BuildScreenshotFilename(pageURL, correlationID string) string {
	hostname := "unknown"
	if pageURL != "" {
		if u, err := url.Parse(pageURL); err == nil && u.Host != "" {
			hostname = u.Host
		}
	}
	ts := time.Now().Format("20060102-150405")
	if correlationID != "" {
		return fmt.Sprintf("%s-%s-%s.jpg", SanitizeForFilename(hostname), ts, SanitizeForFilename(correlationID))
	}
	return fmt.Sprintf("%s-%s.jpg", SanitizeForFilename(hostname), ts)
}

// Truncate returns s unchanged if len(s) <= maxLen. Otherwise, it truncates
// and appends "..." so the total output length equals maxLen.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// SortedMapKeys returns the keys of a string-keyed map in sorted order.
func SortedMapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ExtractURLPath extracts the path portion from a URL string, stripping query parameters.
// Returns "/" if the URL has no path component.
// Returns the input unchanged if it cannot be parsed.
func ExtractURLPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	path := parsed.Path
	if path == "" {
		return "/"
	}
	return path
}

// ExtractOrigin extracts the origin (scheme://host[:port]) from a URL.
// Returns empty string for data: URLs, blob: URLs (after extracting nested origin),
// and malformed URLs.
func ExtractOrigin(rawURL string) string {
	// Handle data: URLs
	if strings.HasPrefix(rawURL, "data:") {
		return ""
	}

	// Handle blob: URLs - extract the nested origin
	// blob:https://example.com/uuid -> https://example.com
	rawURL = strings.TrimPrefix(rawURL, "blob:")

	// Parse URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// URL must have a scheme and host
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	// Reconstruct origin: scheme://host[:port]
	return parsed.Scheme + "://" + parsed.Host
}
