// Purpose: Provides URL query parsing, header formatting, and file-save helpers for HAR export.
// Why: Separates utility functions from HAR conversion and filter logic.
package har

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// parseQueryString extracts query parameters from a URL as name/value pairs.
func parseQueryString(rawURL string) []harNameValue {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return make([]harNameValue, 0)
	}
	params := parsed.Query()
	if len(params) == 0 {
		return make([]harNameValue, 0)
	}
	result := make([]harNameValue, 0, len(params))
	for name, values := range params {
		for _, val := range values {
			result = append(result, harNameValue{Name: name, Value: val})
		}
	}
	return result
}

// httpStatusText returns the standard text for an HTTP status code.
func httpStatusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 304:
		return "Not Modified"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	default:
		return ""
	}
}

// isPathSafe rejects path traversal and absolute paths outside temp directories.
func isPathSafe(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	if filepath.IsAbs(path) {
		tmpDir := os.TempDir()
		return strings.HasPrefix(path, "/tmp/") ||
			strings.HasPrefix(path, "/private/tmp/") ||
			strings.HasPrefix(path, tmpDir+"/")
	}
	return true
}

func writeHARData(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}
