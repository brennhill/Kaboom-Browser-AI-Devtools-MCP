// audits.go — Accessibility, performance, and security audit playbook content.
// Why: These evidence-gathering workflows share the same quick/full audit lifecycle.

package resources

func playbookSetAccessibility() map[string]string {
	return map[string]string{
		"accessibility/quick": `# Playbook: Accessibility Audit (Quick)

Use when you need a fast WCAG issue snapshot.
For a single element check, call analyze(what:"dom", selector:"...") directly.

## Steps

1. {"tool":"configure","arguments":{"what":"health"}}
2. {"tool":"analyze","arguments":{"what":"accessibility"}}
3. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from analyze>"}}

## Output Format

- Top accessibility blockers
- WCAG tags impacted
- Quick remediation suggestions
`,
		"accessibility/full": `# Playbook: Accessibility Audit (Full)

Use for triage plus implementation-ready fixes.

## Preconditions

- Extension connected
- Correct tracked tab

## Steps

1. {"tool":"configure","arguments":{"what":"health"}}
2. {"tool":"observe","arguments":{"what":"page"}}
3. {"tool":"analyze","arguments":{"what":"accessibility"}}
4. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from analyze>"}}
5. {"tool":"analyze","arguments":{"what":"dom","selector":"main, [role='main'], form, nav"}}
6. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from dom analyze>"}}

## Failure Modes

- extension_disconnected: reconnect and retry
- timeout: retry once, then narrow DOM scope

## Output Format

- Findings by severity
- Affected selectors/components
- Concrete code-level fix guidance
- Validation checklist
`,
	}
}

func playbookSetPerformance() map[string]string {
	return map[string]string{
		"performance/quick": `# Playbook: Performance Analysis (Quick)

Use when a page feels slow or performance regressed.
If you only need a single metric (e.g. LCP), call observe(what:"vitals") directly.

## Preconditions

- Extension connected and tracked tab confirmed.
- Target URL known.

## Steps

1. {"tool":"configure","arguments":{"what":"health"}}
2. {"tool":"interact","arguments":{"what":"navigate","url":"<target-url>"}}
3. {"tool":"observe","arguments":{"what":"vitals"}}
4. {"tool":"observe","arguments":{"what":"network_waterfall","status_min":400}}
5. {"tool":"observe","arguments":{"what":"actions","last_n":30}}

## Output Format

- Top 3 bottlenecks
- Evidence (metric/request/action references)
- Lowest-risk first fixes
`,
		"performance/full": `# Playbook: Performance Analysis (Full)

Use for deep profiling and remediation planning.

## When To Use

- Perf regression after a change
- Slow initial load or interaction lag
- Need actionable fix plan with evidence

## Preconditions

- Extension connected
- Correct tracked tab
- Reproducible URL/workflow

## Steps

1. Baseline health:
   {"tool":"configure","arguments":{"what":"health"}}
   {"tool":"observe","arguments":{"what":"page"}}
2. Capture navigation perf diff:
   {"tool":"interact","arguments":{"what":"navigate","url":"<target-url>","analyze":true}}
3. Collect web vitals:
   {"tool":"observe","arguments":{"what":"vitals"}}
4. Collect network hotspots:
   {"tool":"observe","arguments":{"what":"network_waterfall","limit":200}}
5. Collect runtime signals:
   {"tool":"observe","arguments":{"what":"actions","last_n":100}}
   {"tool":"observe","arguments":{"what":"logs","min_level":"warn","last_n":200}}
6. Optional active analysis:
   {"tool":"analyze","arguments":{"what":"performance"}}
   {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from analyze>"}}

## Failure Modes

- extension_disconnected: reconnect/track tab, rerun
- no_perf_diff: ensure navigate/refresh or interact with analyze=true
- sparse_data: increase observe limits and repeat flow

## Output Format

- Summary: regression/no-regression with confidence
- Bottlenecks: ranked with concrete evidence
- Fixes: prioritized quick wins then deeper refactors
- Validation plan: exact checks to verify improvement
`,
	}
}

func playbookSetSecurity() map[string]string {
	return map[string]string{
		"security/quick": `# Playbook: Security Audit (Quick)

Use for fast risk screening.
For a single header/cookie check, call analyze(what:"security_audit") directly.

## Steps

1. {"tool":"configure","arguments":{"what":"health"}}
2. {"tool":"analyze","arguments":{"what":"security_audit"}}
3. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from analyze>"}}

## Output Format

- High-risk findings first
- Evidence location (header/URL/request/etc.)
- Immediate mitigations
`,
		"security/full": `# Playbook: Security Audit (Full)

Use for comprehensive browser-surface security review.

## Preconditions

- Extension connected
- Representative app flow loaded

## Steps

1. {"tool":"configure","arguments":{"what":"health"}}
2. {"tool":"observe","arguments":{"what":"network_waterfall","limit":200}}
3. {"tool":"analyze","arguments":{"what":"security_audit","severity_min":"medium"}}
4. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from security audit>"}}
5. {"tool":"analyze","arguments":{"what":"third_party_audit","first_party_origins":["<origin>"]}}
6. {"tool":"observe","arguments":{"what":"command_result","correlation_id":"<from third_party_audit>"}}

## Failure Modes

- missing baseline traffic: exercise critical user flow and rerun
- noisy false positives: tighten first_party_origins

## Output Format

- Risks ranked by severity and exploitability
- Evidence and affected endpoints/origins
- Prioritized fix plan
- Verification steps
`,
	}
}
