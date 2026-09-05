---
title: "Security Auditing & Hardening"
description: "Use KaBOOM's security tools to audit credentials, PII, headers, cookies, third-party scripts, and transport security. Generate CSP headers and SRI hashes. Track security regressions."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'security', 'auditing']
---

KaBOOM turns your AI assistant into a security auditor. Six check categories scan live browser traffic for vulnerabilities, two generators produce security artifacts, and recording comparison catches regressions before they ship.

## Full Security Audit

Run all checks at once:

```js
analyze({what: "security_audit"})
```

Or target specific categories:

```js
analyze({what: "security_audit", checks: ["credentials", "pii"]})
analyze({what: "security_audit", checks: ["headers", "cookies", "transport"]})
analyze({what: "security_audit", checks: ["auth"]})
```

Filter by minimum severity to focus on critical issues:

```js
analyze({what: "security_audit", severity_min: "high"})
```

Severity levels: `critical` > `high` > `medium` > `low` > `info`

---

## Check Categories

### credentials — Exposed Secrets

Scans network requests, responses, console output, and URL parameters for:

| Pattern | Example | Detection Method |
|---------|---------|-----------------|
| AWS Access Keys | `AKIA1234567890ABCDEF` | Regex + prefix validation |
| GitHub PATs | `ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx` | Regex + length check |
| Stripe Keys | `sk_live_xxxx...xxxx` | Regex + prefix validation |
| JWT Tokens | `eyJhbGciOi...` | Structure validation (3-part base64) |
| Bearer Tokens | `Bearer eyJ...` | Regex + minimum length |
| Private Keys | `-----BEGIN RSA PRIVATE KEY-----` | Header detection |
| API Keys in URLs | `?api_key=abc123...` | Query parameter scanning |

Common findings:
- API responses that include server-side tokens the frontend doesn't need
- Debug endpoints that expose configuration with secrets
- Console.log statements that dump auth state
- JWTs in URL query parameters (visible in browser history and server logs)

### pii — Personal Data Exposure

| Data Type | Detection | Validation |
|-----------|-----------|-----------|
| Social Security Numbers | `XXX-XX-XXXX` pattern | Format check |
| Credit Card Numbers | 13-19 digit sequences | Luhn algorithm (eliminates false positives) |
| Email Addresses | Standard email regex | Domain validation |
| Phone Numbers | US format patterns | 10+ digit validation |

The Luhn validation is important — it means a random 16-digit number in a timestamp or ID won't trigger a false positive. Only numbers that pass the credit card checksum are flagged.

**Why this matters**: If your `/api/users` endpoint returns full SSNs when the UI only displays the last four digits, that's a data minimization violation (GDPR Article 5, CCPA).

### headers — Security Response Headers

Checks every HTTP response for six critical security headers:

| Header | Purpose | Missing = |
|--------|---------|-----------|
| `Strict-Transport-Security` | Forces HTTPS, prevents downgrade attacks | Browsers may connect via HTTP |
| `X-Content-Type-Options: nosniff` | Prevents MIME-type sniffing | Scripts disguised as images can execute |
| `X-Frame-Options` | Blocks clickjacking via iframes | Your pages can be embedded in attacker sites |
| `Content-Security-Policy` | Whitelist for script/style sources | XSS attacks can inject arbitrary scripts |
| `Referrer-Policy` | Controls referrer header leakage | Full URLs (with tokens) sent to third parties |
| `Permissions-Policy` | Restricts browser API access | Third-party scripts can access camera, location |

Findings are per-origin — if your API server has headers but your CDN doesn't, both are reported separately.

### cookies — Cookie Security Flags

For every cookie, especially session cookies (`session`, `token`, `auth`, `jwt`, `sid` patterns):

| Flag | What It Prevents | Severity If Missing |
|------|-----------------|-------------------|
| `HttpOnly` | JavaScript access (XSS can steal cookies) | Warning (high for session cookies) |
| `Secure` | Transmission over HTTP (interception) | Warning on HTTPS sites |
| `SameSite` | Cross-site request forgery (CSRF) | Warning |

A session cookie missing all three flags gets escalated severity.

### transport — Encryption

| Finding | Severity | Description |
|---------|----------|-------------|
| HTTP usage | High | Unencrypted traffic (non-localhost) |
| Mixed content | Medium | HTTPS page loading HTTP resource |
| HTTPS downgrade | Critical | Resource previously loaded via HTTPS now via HTTP |

Localhost is excluded — `http://localhost:3000` is fine for development.

### auth — Unauthenticated PII Endpoints

Identifies API endpoints that:
1. Return response bodies containing PII patterns
2. Were called without an `Authorization` header

This catches the classic "forgot to add the auth middleware" bug on new endpoints.

---

## Third-Party Script Audit

Separate from `security_audit`, the third-party audit analyzes all external origins your page communicates with:

```js
analyze({what: "third_party_audit"})
```

### Risk Classification

Every third-party origin gets a risk rating:

| Risk Level | Criteria | Examples |
|------------|----------|---------|
| **Critical** | Scripts from suspicious domains, data exfiltration patterns | Unknown domain executing JS + receiving POST data |
| **High** | Scripts from unknown origins, high-entropy domain names (DGA) | `xk4m2.example.xyz` serving JavaScript |
| **Medium** | Suspicious TLDs, non-essential services | Analytics from `.click` domains |
| **Low** | Known CDNs, passive resources (fonts, images) | Google Fonts, jsDelivr, Cloudflare CDN |

### Suspicious TLD Detection

| Risk | TLDs |
|------|------|
| High (phishing/malware) | `.loan`, `.download` |
| Medium (abuse-prone) | `.xyz`, `.top`, `.click`, `.stream`, `.review`, `.country` |

### Domain Generation Algorithm (DGA) Detection

High-entropy hostnames (>3.5 bits per character) are flagged as potential malware C&C communication. Legitimate domains like `cdn.jsdelivr.net` have low entropy. Domains like `xk4m2q8f.example.com` have high entropy.

### Custom Configuration

Specify what's first-party and what's allowed or blocked:

```js
analyze({what: "third_party_audit",
         first_party_origins: ["https://api.myapp.com", "https://cdn.myapp.com"],
         include_static: true,
         custom_lists: {
           allowed: ["https://trusted-vendor.com"],
           blocked: ["https://known-bad-tracker.com"],
           internal: ["https://internal-tools.mycompany.com"]
         }})
```

### What's Reported Per Origin

- Domain and origin URL
- Resource types loaded (scripts, styles, fonts, images, XHR)
- Whether scripts are loaded (highest risk)
- Whether data is sent via POST/PUT (outbound data flow)
- Whether PII fields appear in requests
- Whether cookies are set
- Transfer size (total bytes)
- Domain reputation classification
- Risk level with explanation

---

## Security Regression Detection

Use recordings to capture before/after security state and compare error logs:

### Record a Baseline

```js
configure({what: "event_recording_start"})
// Browse the application to capture security-relevant traffic
configure({what: "event_recording_stop", recording_id: "rec-baseline"})
```

### After Changes, Record and Compare

```js
configure({what: "event_recording_start"})
// Browse the same flows after the refactor/deploy
configure({what: "event_recording_stop", recording_id: "rec-after"})
configure({what: "log_diff", original_id: "rec-baseline", replay_id: "rec-after"})
```

The comparison reports new errors, resolved errors, and changes between the two sessions. Run `analyze({what: "security_audit"})` on each recording to compare security posture directly.

---

## Generate Security Artifacts

### Content Security Policy

Generate a CSP header from observed traffic:

```js
generate({what: "csp"})
generate({what: "csp", mode: "strict"})
generate({what: "csp", mode: "report_only"})
```

| Mode | Behavior |
|------|----------|
| `strict` | Only high-confidence origins (3+ observations, 2+ pages) |
| `moderate` | Balanced — includes medium-confidence origins |
| `report_only` | Generates `Content-Security-Policy-Report-Only` (no enforcement) |

**Smart filtering**:
- Browser extension origins auto-excluded (`chrome-extension://`, `moz-extension://`)
- Dev server origins filtered (different-port localhost)
- Low-confidence origins excluded (single observation = possible ad injection)
- Your app's own localhost preserved

**Output includes**:
- Ready-to-use CSP header string
- HTML `<meta>` tag equivalent
- Per-origin confidence details
- Filtered origins with explanations
- Coverage warnings (e.g., "visit more pages for broader coverage")

### Subresource Integrity Hashes

Generate SRI hashes for all third-party scripts and stylesheets:

```js
generate({what: "sri"})
generate({what: "sri", resource_types: ["script"], origins: ["https://cdn.example.com"]})
```

Output per resource:
- SHA-384 hash in browser-standard format
- Ready-to-paste `<script>` or `<link>` tag with `integrity` and `crossorigin` attributes
- File size
- Whether the resource already has SRI protection

---

## Workflow: Complete Security Review

Here's how to use these tools together for a thorough security review:

### 1. Baseline Audit

```
"Run a full security audit of this page — all check categories."
```

```js
analyze({what: "security_audit"})
```

### 2. Third-Party Analysis

```
"Show me all third-party scripts and classify their risk."
```

```js
analyze({what: "third_party_audit"})
```

### 3. Fix Critical Issues

Address findings by severity: critical first, then high, medium, low.

### 4. Generate Security Headers

```
"Generate a strict CSP from what you've observed, and SRI hashes for all external scripts."
```

```js
generate({what: "csp", mode: "strict"})
generate({what: "sri"})
```

### 5. Record Your Secure Baseline

```
"Start a recording to capture our secured state."
```

```js
configure({what: "event_recording_start"})
// Browse the application
configure({what: "event_recording_stop", recording_id: "rec-secured"})
```

### 6. After Future Changes, Compare

```
"Record a new session and compare it to the secured baseline."
```

```js
configure({what: "event_recording_start"})
// Browse the same flows
configure({what: "event_recording_stop", recording_id: "rec-post-change"})
configure({what: "log_diff", original_id: "rec-secured", replay_id: "rec-post-change"})
```

---

## Automatic Credential Redaction

Independent of auditing, KaBOOM automatically protects against accidental data exposure:

**Extension layer** (before data leaves the browser):
- Strips `Authorization`, `Cookie`, `Set-Cookie` headers
- Strips any header containing `token`, `secret`, `key`, or `password`
- Replaces password input values with `[redacted]`

**Server layer** (before data reaches the AI):
- Regex-based redaction of AWS keys, JWTs, credit cards, SSNs
- Luhn validation to avoid false positives on numbers
- Format: `[REDACTED:pattern-name]`

Two layers, always active, zero configuration.

---

## SARIF Export for CI/CD Integration

Export accessibility and security findings in SARIF format for GitHub Code Scanning:

```js
generate({what: "sarif", save_to: "/path/to/report.sarif"})
```

SARIF files integrate with:
- GitHub Code Scanning (upload via `github/codeql-action/upload-sarif`)
- VS Code SARIF Viewer extension
- Azure DevOps
- Any SARIF-compatible CI/CD pipeline
