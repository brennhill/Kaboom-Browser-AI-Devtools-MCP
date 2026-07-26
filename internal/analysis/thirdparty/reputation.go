// Purpose: Implements hostname reputation classification heuristics for third-party audit scoring.
// Why: Provides consistent risk labeling for domains to prioritize review of suspicious origins.
// Docs: docs/features/feature/enterprise-audit/index.md

package thirdparty

import (
	"math"
	"net/url"
	"strings"
)

// knownCDNs is a hardcoded set of known CDN hostnames.
var knownCDNs = map[string]bool{
	"cdn.jsdelivr.net":           true,
	"cdnjs.cloudflare.com":       true,
	"unpkg.com":                  true,
	"fonts.googleapis.com":       true,
	"fonts.gstatic.com":          true,
	"ajax.googleapis.com":        true,
	"code.jquery.com":            true,
	"maxcdn.bootstrapcdn.com":    true,
	"stackpath.bootstrapcdn.com": true,
	"cdn.cloudflare.com":         true,
	"raw.githubusercontent.com":  true,
	"github.githubassets.com":    true,
	"cdn.tailwindcss.com":        true,
	"esm.sh":                     true,
	"deno.land":                  true,
	"cdn.skypack.dev":            true,
}

// abuseTLDs is a hardcoded set of TLDs commonly associated with abuse.
var abuseTLDs = map[string]bool{
	".xyz":     true,
	".top":     true,
	".click":   true,
	".loan":    true,
	".work":    true,
	".tk":      true,
	".ml":      true,
	".ga":      true,
	".cf":      true,
	".gq":      true,
	".buzz":    true,
	".monster": true,
}

// classifyReputation determines the reputation classification for a hostname.
func classifyReputation(hostname string, customLists *CustomLists) DomainReputation {
	if rep, ok := checkCustomLists(hostname, customLists); ok {
		return rep
	}
	if knownCDNs[hostname] {
		return DomainReputation{Classification: "known_cdn", Source: "bundled_list"}
	}
	return checkSuspicionHeuristics(hostname)
}

// checkCustomLists checks if hostname matches enterprise allowed/blocked lists.
func checkCustomLists(hostname string, customLists *CustomLists) (DomainReputation, bool) {
	if customLists == nil {
		return DomainReputation{}, false
	}
	if matchesDomainList(hostname, customLists.Allowed) {
		return DomainReputation{Classification: "enterprise_allowed", Source: "custom_list"}, true
	}
	if matchesDomainList(hostname, customLists.Blocked) {
		return DomainReputation{Classification: "enterprise_blocked", Source: "custom_list"}, true
	}
	return DomainReputation{}, false
}

// matchesDomainList returns true if hostname matches any domain in the list.
func matchesDomainList(hostname string, domains []string) bool {
	for _, d := range domains {
		if hostname == d || strings.HasSuffix(hostname, "."+d) {
			return true
		}
	}
	return false
}

// checkSuspicionHeuristics applies abuse-TLD and DGA heuristics.
func checkSuspicionHeuristics(hostname string) DomainReputation {
	var flags []string
	if abuseTLDs[extractTLD(hostname)] {
		flags = append(flags, "abuse_tld")
	}
	subdomain := extractSubdomain(hostname)
	if subdomain != "" && len(subdomain) > 8 && shannonEntropy(subdomain) > 3.5 {
		flags = append(flags, "possible_dga")
	}
	if len(flags) > 0 {
		return DomainReputation{Classification: "suspicious", SuspicionFlags: flags, Source: "heuristic"}
	}
	return DomainReputation{Classification: "unknown"}
}

// shannonEntropy computes Shannon entropy in bits per character.
func shannonEntropy(s string) float64 {
	freq := make(map[rune]float64)
	for _, c := range s {
		freq[c]++
	}
	length := float64(len([]rune(s)))
	entropy := 0.0
	for _, count := range freq {
		p := count / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// extractHostname extracts the hostname from a URL or origin string.
func extractHostname(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// extractTLD extracts the TLD with dot prefix from a hostname.
func extractTLD(hostname string) string {
	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return ""
	}
	return "." + parts[len(parts)-1]
}

// extractSubdomain extracts the first subdomain label from a hostname.
func extractSubdomain(hostname string) string {
	parts := strings.Split(hostname, ".")
	if len(parts) <= 2 {
		return ""
	}
	return parts[0]
}
