// detectors.go — Individual TLD, port, IP-origin, typosquat and mixed-content detectors.
// Purpose: Detects suspicious TLDs, typosquatting, and anomalous network patterns.
// Why: Separates detection heuristics from static data and distance calculations.
package netflag

import (
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func checkSuspiciousTLD(origin string) *capture.SecurityFlag {
	if _, ok := knownLegitimateOrigins[origin]; ok {
		return nil
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return nil
	}

	hostname := parsed.Hostname()
	for tld, rep := range suspiciousTLDs {
		if strings.HasSuffix(hostname, tld) {
			if rep.Severity == "low" {
				return nil
			}
			return &capture.SecurityFlag{
				Type:      "suspicious_tld",
				Severity:  rep.Severity,
				Origin:    origin,
				Message:   rep.Reason,
				Timestamp: time.Now(),
			}
		}
	}

	return nil
}

func checkNonStandardPort(origin string) *capture.SecurityFlag {
	parsed, err := url.Parse(origin)
	if err != nil {
		return nil
	}

	port := parsed.Port()
	if port == "" {
		return nil
	}

	if _, ok := standardWebPorts[port]; ok {
		return nil
	}

	hostname := parsed.Hostname()
	if isLocalHost(hostname) {
		if _, ok := localhostDevPorts[port]; ok {
			return nil
		}
	}

	return &capture.SecurityFlag{
		Type:      "non_standard_port",
		Severity:  "medium",
		Origin:    origin,
		Message:   "Origin uses non-standard port " + port + " which may indicate compromised infrastructure",
		Timestamp: time.Now(),
	}
}

func checkMixedContent(entry capture.NetworkWaterfallEntry, pageURL string) *capture.SecurityFlag {
	pageParsed, err := url.Parse(pageURL)
	if err != nil || pageParsed.Scheme != "https" {
		return nil
	}

	entryParsed, err := url.Parse(entry.URL)
	if err != nil || entryParsed.Scheme != "http" {
		return nil
	}

	severity := "medium"
	if entry.InitiatorType == "script" || entry.InitiatorType == "stylesheet" {
		severity = "high"
	}

	return &capture.SecurityFlag{
		Type:      "mixed_content",
		Severity:  severity,
		Origin:    entryParsed.Scheme + "://" + entryParsed.Host,
		Message:   "HTTP resource loaded on HTTPS page (mixed content vulnerability)",
		Resource:  entry.URL,
		PageURL:   pageURL,
		Timestamp: time.Now(),
	}
}

func checkIPAddressOrigin(origin string) *capture.SecurityFlag {
	parsed, err := url.Parse(origin)
	if err != nil {
		return nil
	}

	hostname := parsed.Hostname()
	if isLocalHost(hostname) {
		return nil
	}
	if net.ParseIP(hostname) == nil {
		return nil
	}

	return &capture.SecurityFlag{
		Type:      "ip_address_origin",
		Severity:  "medium",
		Origin:    origin,
		Message:   "Origin uses IP address instead of domain name, which may indicate compromised or temporary infrastructure",
		Timestamp: time.Now(),
	}
}

func checkTyposquatting(origin string) *capture.SecurityFlag {
	parsed, err := url.Parse(origin)
	if err != nil {
		return nil
	}

	hostname := parsed.Hostname()
	for _, targetDomain := range typosquatTargetDomains {
		distance := levenshteinDistance(hostname, targetDomain)
		if distance > 0 && distance <= 2 {
			return &capture.SecurityFlag{
				Type:      "potential_typosquatting",
				Severity:  "high",
				Origin:    origin,
				Message:   "Domain is similar to " + targetDomain + " (possible typosquatting)",
				Timestamp: time.Now(),
			}
		}
	}

	return nil
}

func isLocalHost(hostname string) bool {
	return hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
}
