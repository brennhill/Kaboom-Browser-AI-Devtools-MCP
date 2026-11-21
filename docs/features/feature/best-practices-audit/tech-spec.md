---
feature: best-practices-audit
status: proposed
doc_type: tech-spec
feature_id: feature-best-practices-audit
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Best Practices Audit

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Best Practices Audit would add `observe({what: "best_practices_audit"})` -- a new observation mode that checks for common web development best practice violations. Runs four audit categories: HTTPS usage, deprecated APIs, security headers, and console warnings. Planned to run in the extension page context using DOM inspection, Location API, and performance entries.

## Key Components

**Audit Categories**:

1. **HTTPS Enforcement**: Checks `window.location.protocol === 'https:'`. Flags mixed content (HTTPS page loading HTTP resources) by inspecting Resource Timing entries for `http://` URLs.

2. **Deprecated APIs**: Scans for usage of deprecated Web APIs in console warnings. Looks for warning messages containing "deprecated" keyword and known patterns (e.g., "document.write", "unload event", "WebSQL").

3. **Security Headers**: Inspects response headers from network waterfall for missing security headers: `Strict-Transport-Security`, `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy`. Only checks main document (not subresources).

4. **Third-party Scripts**: Identifies scripts loaded from external domains (CDNs, analytics, ads). Flags if total external script size > 500KB. Uses Resource Timing API with origin comparison.

**Finding Classification**: Each violation assigned: `critical` (security risk), `high` (impacts users), `medium` (maintenance concern), `low` (optimization).

**Recommendation Generator**: Produces actionable fix for each finding (e.g., "Upgrade all HTTP resources to HTTPS", "Remove deprecated document.write calls", "Add Content-Security-Policy header").

## Data Flows

```
AI calls observe({what: "best_practices_audit"})
  |
  v
Extension analyzes page
  -> Check protocol (HTTPS vs HTTP)
  -> Scan console for deprecated API warnings
  -> Read network waterfall for security headers
  -> Identify third-party scripts
  |
  v
Classify findings and generate recommendations
  |
  v
Return audit report to server
```

## Implementation Strategy

Not yet implemented as a standalone `observe` mode. Planned:
- Extension: a new best-practices audit module in the TypeScript extension (`src/`), invoked from the page-inspection path, holding audit logic and severity classification.
- Server: a new `observe` mode handler for `best_practices_audit` in the observe tool package.

Current state: `analyze({what: "audit"})` already reports a `best_practices` category (`cmd/browser-agent/internal/toolanalyze/combinedaudit/handler.go`), but it is backed by the third-party script audit (`securityaudit.HandleThirdPartyAudit`) and scored by `scoreBestPractices` in `cmd/browser-agent/internal/toolanalyze/audit_scoring.go`. HTTPS, deprecated-API, and security-header checks described above are not part of it.

**Trade-offs**:
- Extension-side analysis because requires DOM and network access
- Security headers only checked for main document (not all subresources) to limit scope

## Edge Cases & Assumptions

- **localhost/127.0.0.1**: HTTP acceptable (not flagged) for local development
- **Missing network waterfall**: Security header audit skipped if network data unavailable
- **Console logs cleared**: Deprecated API detection only sees warnings captured since extension loaded

## Risks & Mitigations

**Risk**: False positives for intentional HTTP usage (local dev).
**Mitigation**: Exclude localhost from HTTPS enforcement audit.

## Dependencies

- Location API
- Resource Timing API
- Network waterfall from Kaboom

## Performance Considerations

| Metric | Target |
|--------|--------|
| Audit execution time | < 100ms |
| Memory impact | < 500KB |

## Security Considerations

- Read-only inspection of page state
- No external network requests
- Header inspection uses data already captured by Kaboom
