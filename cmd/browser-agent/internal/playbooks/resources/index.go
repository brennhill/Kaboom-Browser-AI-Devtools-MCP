// index.go — Embeds the token-efficient capability index served as an MCP resource.
// Why: Provides token-efficient documentation that MCP clients can read without external network access.

package resources

// CapabilityIndex is the token-efficient capability discovery resource.
const CapabilityIndex = `# Kaboom Capability Index (Token-Efficient)

Use this index for discovery. Load detailed guidance only when task intent matches.

## Routing Rule

1. Identify intent from user request.
2. If intent matches a capability below, read that playbook URI.
3. Prefer quick playbook first; load full playbook only when needed.

## Capability Map

| Capability | When to Use | Playbook URI |
|---|---|---|
| performance | Regressions, slow pages, bottlenecks, Core Web Vitals | kaboom://playbook/performance/quick |
| accessibility | WCAG/axe issues, semantic/contrast/navigation checks | kaboom://playbook/accessibility/quick |
| security | Credential leaks, CSP/cookie/header risks, third-party origin risk | kaboom://playbook/security/quick |
| automation | Navigate sites, fill forms, click buttons, post content, any browser task | kaboom://playbook/automation/quick |
| api_validation | Contract mismatches, malformed responses, endpoint drift | See kaboom://guide |

## Available Playbook Variants

- kaboom://playbook/performance/quick
- kaboom://playbook/performance/full
- kaboom://playbook/accessibility/quick
- kaboom://playbook/accessibility/full
- kaboom://playbook/security/quick
- kaboom://playbook/security/full
- kaboom://playbook/automation/quick
- kaboom://playbook/automation/full

## Runtime Discovery

When unsure which params a mode accepts, use per-mode filtering:
- configure(what:"describe_capabilities", tool:"observe", mode:"errors") → returns only params relevant to observe/errors
- configure(what:"describe_capabilities", tool:"interact") → returns all interact modes with their per-mode params

## Notes

- Keep this index small; do not inline full workflows here.
- Add future playbooks under kaboom://playbook/{capability}/{quick|full}.
`

func playbooks() map[string]string {
	return mergePlaybookSets(
		playbookSetPerformance(),
		playbookSetAccessibility(),
		playbookSetSecurity(),
		playbookSetAutomation(),
	)
}

func mergePlaybookSets(sets ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, set := range sets {
		for key, value := range set {
			if _, exists := merged[key]; exists {
				panic("duplicate playbook key: " + key)
			}
			merged[key] = value
		}
	}
	return merged
}
