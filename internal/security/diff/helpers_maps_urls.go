// helpers_maps_urls.go — Key-set union helpers and URL/origin normalization.
// Purpose: Normalizes transport maps and compares URL security between diff snapshots.
// Why: Separates URL and transport diff logic from header, cookie, and summary helpers.
package diff

import (
	"net/url"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/httpsec"
)

func normalizeTransportByHost(transport map[string]string) map[string]string {
	byHost := make(map[string]string, len(transport))
	for origin, scheme := range transport {
		host := extractHostFromOrigin(origin)
		if host != "" {
			byHost[host] = scheme
		}
	}
	return byHost
}

func cookieSliceToMap(cookies []Cookie) map[string]Cookie {
	m := make(map[string]Cookie, len(cookies))
	for _, c := range cookies {
		m[c.Name] = c
	}
	return m
}

func collectMapKeys[V any](a, b map[string]map[string]V) map[string]bool {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

func collectCookieMapKeys(a, b map[string][]Cookie) map[string]bool {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

func collectBoolMapKeys(a, b map[string]bool) map[string]bool {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

func collectStringMapKeys(a, b map[string]string) map[string]bool {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

func extractSnapshotOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Scheme + "://" + parsed.Host
}

func extractScheme(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Scheme
}

func extractHostFromOrigin(origin string) string {
	parsed, err := url.Parse(origin)
	if err != nil {
		return origin
	}
	return parsed.Host
}

func parseSnapshotCookies(setCookieHeader string) []Cookie {
	var cookies []Cookie

	lines := strings.Split(setCookieHeader, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parsed := httpsec.ParseSingleCookie(line)
		cookies = append(cookies, Cookie(parsed))
	}

	return cookies
}
