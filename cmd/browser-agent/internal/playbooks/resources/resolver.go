// resolver.go — Resolves playbook/demo resource URIs to canonical URIs and markdown content.
// Why: Isolates strict URI parsing from large static documentation payloads.

package resources

import "strings"

// CanonicalPlaybookCapability validates a canonical capability name.
func CanonicalPlaybookCapability(capability string) string {
	switch strings.ToLower(strings.TrimSpace(capability)) {
	case "performance":
		return "performance"
	case "accessibility":
		return "accessibility"
	case "security":
		return "security"
	case "automation":
		return "automation"
	default:
		return ""
	}
}

// ResolvePlaybookKey resolves "{capability}/{level}" and bare "{capability}" to canonical keys.
func ResolvePlaybookKey(raw string) string {
	trimmed := strings.Trim(strings.ToLower(strings.TrimSpace(raw)), "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	switch len(parts) {
	case 1:
		capability := CanonicalPlaybookCapability(parts[0])
		if capability == "" {
			return ""
		}
		return capability + "/quick"
	case 2:
		capability := CanonicalPlaybookCapability(parts[0])
		level := strings.TrimSpace(parts[1])
		if capability == "" || level == "" {
			return ""
		}
		return capability + "/" + level
	default:
		return ""
	}
}

// ResolveResourceContent resolves a kaboom resource URI into canonical URI + markdown.
func ResolveResourceContent(uri string) (string, string, bool) {
	switch {
	case uri == "kaboom://capabilities":
		return uri, CapabilityIndex, true
	case uri == "kaboom://guide":
		return uri, GuideContent, true
	case uri == "kaboom://quickstart":
		return uri, QuickstartContent, true
	case strings.HasPrefix(uri, "kaboom://playbook/"):
		key := ResolvePlaybookKey(strings.TrimPrefix(uri, "kaboom://playbook/"))
		text, ok := playbooks()[key]
		if !ok {
			return "", "", false
		}
		return "kaboom://playbook/" + key, text, true
	case strings.HasPrefix(uri, "kaboom://demo/"):
		name := strings.TrimPrefix(uri, "kaboom://demo/")
		text, ok := demoScripts()[name]
		if !ok {
			return "", "", false
		}
		return uri, text, true
	default:
		return "", "", false
	}
}
