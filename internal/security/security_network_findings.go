// Purpose: Analyzes network waterfall entries for security issues (mixed content, insecure transport).
// Why: Separates network-level security analysis from credential scanning and header checks.
package security

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/netflag"
)

func (s *SecurityScanner) checkNetworkSecurity(entries []capture.NetworkWaterfallEntry, pageURLs []string) []SecurityFinding {
	var findings []SecurityFinding
	pageURL := ""
	if len(pageURLs) > 0 {
		pageURL = pageURLs[0]
	}

	for _, entry := range entries {
		flags := netflag.Analyze(entry, pageURL)
		for _, flag := range flags {
			findings = append(findings, SecurityFinding{
				Check:       "network",
				Severity:    flag.Severity,
				Title:       flag.Message,
				Description: networkFlagDescription(flag.Type),
				Location:    flag.Resource,
				Evidence:    flag.Origin,
				Remediation: networkFlagRemediation(flag.Type),
			})
		}
	}
	return findings
}

func networkFlagDescription(flagType string) string {
	switch flagType {
	case "suspicious_tld":
		return "Resource loaded from a TLD with elevated abuse rates. May indicate a supply chain attack or compromised dependency."
	case "non_standard_port":
		return "Resource loaded from a non-standard port, which may indicate compromised or temporary infrastructure."
	case "mixed_content":
		return "HTTP resource loaded on an HTTPS page. An attacker on the network can modify this resource."
	case "ip_address_origin":
		return "Resource loaded from an IP address instead of a domain name. May indicate compromised or ephemeral infrastructure."
	case "potential_typosquatting":
		return "Domain is suspiciously similar to a popular CDN or service. May be a typosquatting attack."
	default:
		return "Suspicious network origin detected."
	}
}

func networkFlagRemediation(flagType string) string {
	switch flagType {
	case "suspicious_tld":
		return "Verify the domain is legitimate. Consider using well-known CDNs for third-party resources."
	case "non_standard_port":
		return "Use standard ports (80/443) for production resources. Investigate why a non-standard port is in use."
	case "mixed_content":
		return "Upgrade all resource URLs to HTTPS. Use Content-Security-Policy: upgrade-insecure-requests."
	case "ip_address_origin":
		return "Use domain names with proper DNS. Investigate why a direct IP address is being used."
	case "potential_typosquatting":
		return "Verify the exact domain name. Check package.json / CDN references for typos."
	default:
		return "Investigate the flagged origin and verify it is legitimate."
	}
}
