---
status: proposed
scope: feature/security-hardening/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-security-hardening
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Security Hardening

> QA plan for the Security Hardening feature suite. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification. This feature encompasses four sub-tools: CSP Generator (`generate_csp`), Third-Party Risk Audit (`audit_third_parties`), Security Regression Detection (`diff_security`), and SRI Hash Generator (`generate_sri`).

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. These tools analyze security posture and generate policies -- they must not inadvertently expose sensitive captured data in their responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Network body content in CSP response | `generate_csp` must only expose origin URLs and resource types, NOT request/response bodies | critical |
| DL-2 | Auth headers in third-party audit | `audit_third_parties` must not include `Authorization`, `Cookie`, or `X-API-Key` header values in its output | critical |
| DL-3 | PII field VALUES in audit outbound details | `outbound_details.contains_pii_fields` must list field NAMES only (e.g., `"email"`), NOT field values (e.g., `"user@example.com"`) | critical |
| DL-4 | Full URLs with query parameters in CSP | `generate_csp` must extract origins only (scheme://host:port), NOT full URLs with query strings that may contain tokens | high |
| DL-5 | Cookie values in third-party audit | `audit_third_parties` must report `sets_cookies: true/false`, NOT cookie values | high |
| DL-6 | Response body content in SRI tool | `generate_sri` must output hashes only, NOT the response body content used to compute them | high |
| DL-7 | Inline script content in CSP output | `inline_scripts.preview` field must be truncated (e.g., first 50 chars) and must NOT include full script content | high |
| DL-8 | Security snapshot contains credentials | `diff_security` snapshots must capture header names and presence, NOT header values for auth headers | critical |
| DL-9 | Enterprise custom list file paths exposed | Tool responses should not expose the full filesystem path of custom list files | medium |
| DL-10 | External enrichment leaks internal domains | When `enable_external_enrichment` is true, only third-party (non-first-party) domains are sent to external services | high |
| DL-11 | RDAP/CT results cached across sessions | External enrichment cache must be cleared on session reset, not persisted | medium |
| DL-12 | Reputation data exposes internal classification logic | Enterprise `blocked` reasons should not be sent to untrusted AI clients if they contain sensitive policy info | medium |

### Negative Tests (must NOT leak)
- [ ] `generate_csp` response contains no URL query parameters or fragments
- [ ] `audit_third_parties` response contains no `Authorization`, `Cookie`, or `Set-Cookie` header values
- [ ] `audit_third_parties` PII detection reports field names, never field values
- [ ] `generate_sri` response contains only hashes and URLs, no response body excerpts
- [ ] `diff_security` snapshot does not store auth header values (e.g., Bearer token strings)
- [ ] `inline_scripts.preview` is truncated to a safe length
- [ ] External enrichment queries use only registrable domain names, not full URLs
- [ ] Cookie analysis reports presence only (`sets_cookies: true`), not cookie content

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand security findings and act on them.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | CSP directive names are standard | Directives use exact W3C names: `script-src`, `style-src`, etc. | [ ] |
| CL-2 | Risk levels are self-documenting | `critical`, `high`, `medium`, `low`, `info` with `risk_reason` explanation | [ ] |
| CL-3 | Confidence scoring is explained | `high`, `medium`, `low` with `observation_count` and criteria in docs | [ ] |
| CL-4 | `included: false` has `exclusion_reason` | Low-confidence origins explain WHY they were excluded | [ ] |
| CL-5 | Filtered origins explain filtering | `filtered_origins[].reason` is human-readable: "Browser extension origin (auto-filtered)" | [ ] |
| CL-6 | Regression verdicts are unambiguous | `verdict` is one of: `regressed`, `improved`, `unchanged` -- not a numeric score | [ ] |
| CL-7 | SRI warnings explain limitations | `Vary: User-Agent` warning explains cross-browser hash mismatch risk | [ ] |
| CL-8 | Reputation classifications are clear | `known_cdn`, `known_tracker`, `suspicious`, `unknown` are self-explanatory | [ ] |
| CL-9 | Enterprise status vs bundled status | When enterprise lists override bundled classification, both are shown | [ ] |
| CL-10 | `recommendations` array is actionable | Each recommendation is a complete sentence with specific next step | [ ] |
| CL-11 | Diff security changes have `before`/`after` | Each regression shows exact previous and current values | [ ] |
| CL-12 | CSP meta tag vs header distinction | Response notes that `frame-ancestors` only works via header, not meta tag | [ ] |

### Common LLM Misinterpretation Risks
- [ ] Risk: LLM interprets `origin_details[].included: false` as "this origin is blocked" rather than "not included in generated CSP" -- verify `exclusion_reason` clarifies
- [ ] Risk: LLM confuses CSP `report_only` mode with actual enforcement -- verify `mode` field and `recommended_next_step` explain the difference
- [ ] Risk: LLM treats `risk_level: "high"` in third-party audit as "must remove immediately" rather than "loads executable code" -- verify `risk_reason` provides context
- [ ] Risk: LLM misinterprets `suspicious` reputation as `malicious` -- verify these are distinct classifications with different severities
- [ ] Risk: LLM assumes `diff_security` "unchanged" verdict means "fully secure" rather than "no regressions detected" -- verify `summary` wording is precise
- [ ] Risk: LLM deploys CSP generated from incomplete observation (few pages visited) -- verify `pages_visited` count and warning are prominent

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Generate CSP (basic) | 2 steps: browse app + call `generate_csp` | No -- observation requires browsing |
| Generate CSP (two-pass) | 5 steps: browse, generate report-only, deploy, browse again, generate enforcing | No -- two-pass is inherently multi-step |
| Audit third parties | 2 steps: browse app + call `audit_third_parties` | No -- already minimal |
| Audit with enterprise lists | 3 steps: create custom lists file + browse + call with `custom_lists_file` | Could embed lists in server config |
| Security regression check | 3 steps: snapshot + make changes + compare | No -- before/after is inherently 3 steps |
| Generate SRI hashes | 2 steps: browse app + call `generate_sri` | No -- already minimal |
| Full security audit | 4 steps: browse + CSP + audit + SRI | Could offer combined "security audit" action |

### Default Behavior Verification
- [ ] `generate_csp` works with zero parameters (defaults to `moderate` mode)
- [ ] `audit_third_parties` auto-detects first-party origin from page URLs
- [ ] `diff_security` defaults `compare_to` to current live state
- [ ] `generate_sri` defaults to both scripts and styles
- [ ] External enrichment is OFF by default (no network calls without opt-in)
- [ ] Development pollution filtering is automatic (no configuration needed)
- [ ] Low-confidence origins excluded by default (safe default)
- [ ] Enterprise custom lists are optional (tool works without them)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Origin extraction from URL | `https://cdn.example.com/path?token=secret` | `https://cdn.example.com` (no path/query) | must |
| UT-2 | Same-origin detection | Page URL and resource URL share origin | `'self'` in directive | must |
| UT-3 | Resource classification: JS | Response content-type `application/javascript` | Mapped to `script-src` | must |
| UT-4 | Resource classification: CSS | Response content-type `text/css` | Mapped to `style-src` | must |
| UT-5 | Resource classification: font | Response content-type `font/woff2` | Mapped to `font-src` | must |
| UT-6 | Resource classification: image | Response content-type `image/png` | Mapped to `img-src` | must |
| UT-7 | Resource classification: WebSocket | `wss://realtime.example.com` | Mapped to `connect-src` | must |
| UT-8 | Resource classification: XHR/fetch | `application/json` response | Mapped to `connect-src` | must |
| UT-9 | data: URI handling | `data:image/png;base64,...` | `data:` added to `img-src` | must |
| UT-10 | blob: URI handling | `blob:https://example.com/uuid` | `blob:` added to appropriate directive | should |
| UT-11 | Confidence scoring: high | 5+ observations across 2+ pages | `confidence: "high"`, included | must |
| UT-12 | Confidence scoring: medium | 2-4 observations or 1 page | `confidence: "medium"`, included with note | must |
| UT-13 | Confidence scoring: low | 1 observation | `confidence: "low"`, excluded by default | must |
| UT-14 | connect-src relaxation | API endpoint observed once | `confidence: "medium"` (not low) | must |
| UT-15 | Dev pollution filter: chrome-extension | `chrome-extension://abc123/script.js` | Filtered, listed in `filtered_origins` | must |
| UT-16 | Dev pollution filter: Vite | `/__vite_ping` | Filtered | must |
| UT-17 | Dev pollution filter: HMR | `main.hot-update.json` | Filtered | must |
| UT-18 | Dev pollution filter: source maps | `app.js.map` | Filtered | should |
| UT-19 | Risk classification: critical | Origin loads scripts AND receives POST data | `risk_level: "critical"` | must |
| UT-20 | Risk classification: high | Origin loads scripts only | `risk_level: "high"` | must |
| UT-21 | Risk classification: medium | Origin receives outbound POST data | `risk_level: "medium"` | must |
| UT-22 | Risk classification: low | Origin serves static images only | `risk_level: "low"` | must |
| UT-23 | PII field name detection | POST body with field named `email` | `contains_pii_fields: ["email"]` | must |
| UT-24 | SRI hash computation | Known content bytes | Correct SHA-384 hash in `sha384-...` format | must |
| UT-25 | SRI crossorigin attribute | Third-party resource | `crossorigin: "anonymous"` | must |
| UT-26 | Inline script hash (moderate mode) | Inline script content | SHA-256 hash in `'sha256-...'` format | must |
| UT-27 | Strict mode differences | `mode: "strict"` | No `'unsafe-inline'`, recommends nonces | must |
| UT-28 | Abuse TLD heuristic | Domain on `.xyz` TLD | `abuse_tld` flag set | must |
| UT-29 | DGA pattern heuristic | High-entropy subdomain | `dga_pattern` flag set | should |
| UT-30 | Excessive subdomain depth | `a.b.c.d.e.example.com` | `excessive_depth` flag set | should |
| UT-31 | Two heuristic flags trigger suspicious | Domain with abuse TLD + not in Tranco | `classification: "suspicious"` | must |
| UT-32 | Single heuristic flag is informational | Domain with only abuse TLD | Not classified as suspicious | must |
| UT-33 | Enterprise blocked overrides allowed | Same origin in both lists | Treated as blocked | must |
| UT-34 | Enterprise allowed overrides tracker | Known tracker in allowed list | Classified as `enterprise_allowed` | must |
| UT-35 | Wildcard matching in custom lists | `*.example.com` matches `cdn.example.com` | Match found | must |
| UT-36 | Wildcard does NOT match bare domain | `*.example.com` vs `example.com` | No match | must |
| UT-37 | Expired allowed entry ignored | Entry with past `expires` date | Falls back to bundled classification | must |
| UT-38 | Origin accumulator persistence | 2000 requests, early origins still in accumulator | All origins retained | must |
| UT-39 | Origin accumulator clear on session reset | Reset session | Accumulator empty | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | CSP generation from multi-origin session | Origin accumulator + CSP generator | Complete CSP with per-directive breakdown | must |
| IT-2 | CSP with inline script detection | DOM query + CSP generator | Hashes included in `script-src` for moderate mode | must |
| IT-3 | Third-party audit with PII detection | Network body buffer + audit tool | PII field names flagged in outbound details | must |
| IT-4 | Third-party audit with enterprise custom lists (inline) | Audit tool + inline `custom_lists` | Enterprise classifications applied | must |
| IT-5 | Third-party audit with custom lists file | Audit tool + file path | File loaded, classifications applied | must |
| IT-6 | Third-party audit with invalid custom lists file | Audit tool + bad JSON file | Clear error message returned | must |
| IT-7 | Security regression: header removed | Snapshot + remove header + compare | Regression detected with correct severity | must |
| IT-8 | Security regression: cookie flag lost | Snapshot + remove HttpOnly + compare | Regression detected as `warning` | must |
| IT-9 | Security regression: auth removed | Snapshot + remove auth + compare | Regression detected as `critical` | must |
| IT-10 | Security regression: no changes | Snapshot + compare immediately | `verdict: "unchanged"` | must |
| IT-11 | SRI generation from captured bodies | Network body buffer + SRI tool | Correct hashes for all third-party scripts | must |
| IT-12 | SRI with truncated body | Body > capture limit | Warning: "body too large for SRI computation" | must |
| IT-13 | SRI with Vary: User-Agent | Resource with Vary header | Warning about cross-browser hash mismatch | should |
| IT-14 | CSP + reputation integration | Reputation data modifies CSP confidence | `reputation_adjustments` field populated | must |
| IT-15 | Snapshot storage TTL (4-hour expiry) | Create snapshot, wait > 4h | Snapshot expired, compare returns error | should |
| IT-16 | Maximum 5 snapshots, oldest evicted | Create 6 snapshots | First snapshot evicted, newest stored | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | CSP generation with 50 origins | Latency | < 50ms | must |
| PT-2 | Origin accumulator memory with 200 entries | Memory | < 20KB | must |
| PT-3 | Third-party audit with bundled reputation lists | Startup memory | ~380KB for lists | must |
| PT-4 | Third-party audit with 20 third-party origins | Latency | < 100ms | must |
| PT-5 | External enrichment with 10 unknown origins | Latency | < 10s (bounded) | must |
| PT-6 | RDAP rate limiting | Query rate | <= 1 request/sec | must |
| PT-7 | Security snapshot memory | Per-snapshot size | < 50KB | should |
| PT-8 | SRI hash computation for 10 scripts | Latency | < 20ms | should |
| PT-9 | Total additional memory (all tools active) | Peak memory | < 660KB | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Empty session (no network traffic) | Call `generate_csp` immediately | Returns minimal `default-src 'self'` policy | must |
| EC-2 | App with 100+ unique origins | Large origin set | All origins processed, accumulator bounded | must |
| EC-3 | All resources from same origin | Single-origin app | CSP is just `default-src 'self'` | must |
| EC-4 | Page origin is localhost | Development environment | Warning: "suggest staging re-run for production CSP" | must |
| EC-5 | Custom lists file with wildcard port matching | Entry without port | Matches any port on that host | must |
| EC-6 | Custom lists file with specific port | `https://cdn.example.com:8443` | Only matches that port | must |
| EC-7 | RDAP server unreachable | External enrichment enabled, RDAP times out | Graceful fallback, other enrichments continue | must |
| EC-8 | Safe Browsing flags enterprise-allowed domain | Domain in both allowed list and Safe Browsing malware list | Safe Browsing overrides with warning | must |
| EC-9 | Resource loaded by Service Worker | SW-fetched resource | May be missed; documented limitation | should |
| EC-10 | Network body buffer wrapped around | Early resources evicted from buffer | Origin accumulator still has all origins | must |
| EC-11 | Snapshot compared to deleted snapshot | `compare_from` references expired/deleted snapshot | Clear error: "snapshot not found" | must |
| EC-12 | SRI for dynamic Google Fonts CSS | Google Fonts with Vary: User-Agent | Warning generated, hash still computed for current UA | should |
| EC-13 | Concurrent CSP generation and data ingestion | Generate CSP while extension is pushing data | No data race, consistent output | must |
| EC-14 | Public suffix extraction | `user.github.io` | Treated as registrable domain, not subdomain of `github.io` | must |
| EC-15 | Extension-injected inline scripts | Chrome extension adds inline `<script>` | Detected and excluded from CSP hash computation | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application with multiple third-party resources available for browsing (e.g., a React app with CDN scripts, Google Fonts, analytics)

### CSP Generator UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Human browses 5+ pages of the web app (home, dashboard, settings, profile, login) | Pages load normally | Extension captures all resource loads | [ ] |
| UAT-2 | AI calls: `{"tool": "generate", "params": {"type": "csp", "mode": "moderate"}}` | MCP response | Response contains `csp_header`, `meta_tag`, `directives`, `origin_details` | [ ] |
| UAT-3 | Verify CSP directives match observed resources | Compare CSP directives to DevTools Network tab | Each third-party origin appears in correct directive | [ ] |
| UAT-4 | Verify low-confidence origins excluded | Check `origin_details` for `included: false` entries | Origins seen only once are excluded with `exclusion_reason` | [ ] |
| UAT-5 | Verify dev pollution filtered | Check `filtered_origins` | Extension origins and dev server URLs are listed as filtered | [ ] |
| UAT-6 | Verify `observations` summary is accurate | Cross-check `total_resources`, `unique_origins`, `pages_visited` | Counts match what was observed during browsing | [ ] |
| UAT-7 | AI calls with `mode: "strict"`: `{"tool": "generate", "params": {"type": "csp", "mode": "strict"}}` | MCP response | No `'unsafe-inline'` in output, nonces recommended | [ ] |

### Third-Party Audit UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-8 | AI calls: `{"tool": "generate", "params": {"type": "third_party_audit"}}` | MCP response | Response contains `third_parties` array with risk levels | [ ] |
| UAT-9 | Verify risk classifications | Compare to DevTools Network tab | Scripts from CDNs are `high` risk, static images are `low` | [ ] |
| UAT-10 | Verify reputation data | Check `reputation` field on known CDN | `classification: "known_cdn"` with Tranco rank | [ ] |
| UAT-11 | Verify PII detection | If app sends POST to analytics | `outbound_details.contains_pii_fields` lists field names only (no values) | [ ] |
| UAT-12 | Verify recommendations are actionable | Read `recommendations` array | Each recommendation is a complete sentence with specific action | [ ] |
| UAT-13 | Verify `summary` counts | Check totals | `total_third_parties`, risk level counts match `third_parties` array | [ ] |

### Security Regression UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-14 | AI takes snapshot: `{"tool": "generate", "params": {"type": "security_diff", "action": "snapshot", "name": "baseline"}}` | MCP response | Snapshot created, confirmation returned | [ ] |
| UAT-15 | Human disables a security header in the app (e.g., remove X-Frame-Options middleware) | App change | Header no longer present in responses | [ ] |
| UAT-16 | Human browses the app again to capture new state | Pages load | New header state captured | [ ] |
| UAT-17 | AI compares: `{"tool": "generate", "params": {"type": "security_diff", "action": "compare", "compare_from": "baseline"}}` | MCP response | `verdict: "regressed"`, regression entry for missing X-Frame-Options | [ ] |
| UAT-18 | Verify regression details | Inspect `regressions` array | `category: "headers"`, `change: "header_removed"`, `severity: "warning"` | [ ] |

### SRI Generator UAT

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-19 | AI calls: `{"tool": "generate", "params": {"type": "sri"}}` | MCP response | Response contains `resources` array with hashes | [ ] |
| UAT-20 | Verify hash format | Inspect `hash` field | Format: `sha384-<base64>` | [ ] |
| UAT-21 | Verify `html` field is valid | Copy `html` field into an HTML file | Valid `<script>` tag with `integrity` and `crossorigin` attributes | [ ] |
| UAT-22 | Verify `already_has_sri` detection | If any resources have SRI | `already_has_sri: true` for those resources | [ ] |
| UAT-23 | Verify warnings for Vary: User-Agent | Check for Google Fonts or similar | Warning about cross-browser hash differences | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | CSP response has no query strings | Inspect all URLs in CSP response | Only `scheme://host:port` format | [ ] |
| DL-UAT-2 | Third-party audit has no auth header values | Inspect all fields | No `Authorization`, `Cookie` values | [ ] |
| DL-UAT-3 | PII detection shows names not values | Inspect `outbound_details` | Field names like `"email"`, not values like `"user@example.com"` | [ ] |
| DL-UAT-4 | SRI output has no body content | Inspect `resources` array | Only URLs, hashes, HTML tags -- no body excerpts | [ ] |
| DL-UAT-5 | Inline script preview is truncated | Inspect `inline_scripts` | `preview` field is short (< 100 chars), not full script | [ ] |
| DL-UAT-6 | Security snapshot has no auth values | Inspect diff output | Header presence/absence shown, not header values for auth | [ ] |

### Regression Checks
- [ ] Existing `observe` tool still works after security hardening tools are added
- [ ] Existing `generate` tool functionality (reproduction scripts, tests, etc.) unaffected
- [ ] Network body capture performance unchanged (< 0.5ms)
- [ ] Origin accumulator does not interfere with existing ring buffer behavior
- [ ] Bundled reputation lists do not cause server startup delay (< 100ms)
- [ ] Extension requires zero changes (all analysis is server-side)

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
