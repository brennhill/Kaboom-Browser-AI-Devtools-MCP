---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Kaboom Demo Strategy

## Overview

Marketing demos for Kaboom, showcasing how AI + browser telemetry accelerates the product development lifecycle.

## Contents

- [demo-strategy.md](demo-strategy.md) — This file (overview + demo scripts)
- [feature-power-ranking.md](feature-power-ranking.md) — Most powerful features ranked
- [demo-scenarios.md](demo-scenarios.md) — Real-world scenarios where Kaboom shines
- [shopbroken-spec.md](shopbroken-spec.md) — Demo site specification

---

## The 12 Demos

### Category A: Debugging & Quality (Demos 1-6)
### Category B: Automation & Workflow (Demos 7-12)

### Demo 1: "Bug Detective" (3 min)
**Story:** User reports checkout is broken. AI investigates autonomously.

**Flow:**
1. AI navigates to `/checkout` via `interact(navigate)`
2. Fills form via `interact(type)` + `interact(click)`
3. Submits — 500 error appears
4. `observe(error_bundles, window_seconds: 5)` → full context: the POST body, the error, the console warning
5. AI identifies: the API expects `"quantity": number` but frontend sends `"quantity": "2"` (string)
6. AI fixes the frontend code (parseInt the quantity)
7. Re-tests via interact → 200 OK, order confirmed
8. `generate(test)` → Playwright test for checkout flow

**Features showcased:** error_bundles, interact (navigate/type/click), network_bodies, test generation

---

### Demo 2: "Accessibility Blitz" (3 min)
**Story:** Make the site WCAG AA compliant in one session.

**Flow:**
1. `analyze(accessibility, tags: ["wcag2aa"])` → 12 violations found
2. AI reads first violation: "Images must have alternate text" (8 instances)
3. AI fixes the code — adds alt attributes
4. Re-scans → 4 violations remaining
5. Fixes: form labels, color contrast, keyboard nav
6. Final scan → 0 violations
7. `generate(sarif)` → export findings for CI integration

**Features showcased:** accessibility audit, structured findings, iterative fix/verify loop, SARIF export

---

### Demo 3: "Security Lockdown" (3 min)
**Story:** Pre-launch security review, from audit to remediation.

**Flow:**
1. `analyze(security_audit, checks: ["headers", "cookies", "credentials", "pii"])` → 8 findings
2. `analyze(third_party_audit)` → 4 unknown third-party origins
3. AI reviews findings: missing HSTS, insecure cookies, no CSP, third-party risk
4. `generate(csp, mode: "strict")` → ready-to-use CSP header
5. `generate(sri, resource_types: ["script"])` → integrity hashes for external scripts
6. AI adds security headers to server config
7. Re-audit → 0 critical findings

**Features showcased:** security_audit, third_party_audit, CSP generation, SRI generation, before/after

---

### Demo 4: "Performance Rescue" (3 min)
**Story:** Site is slow, users leaving. AI diagnoses and fixes.

**Flow:**
1. `observe(vitals)` → LCP: 4.2s, CLS: 0.31, both failing
2. `observe(network_waterfall)` → 3MB hero image, render-blocking JS
3. `observe(timeline)` → font swap causing layout shift at 1.2s
4. AI fixes: compress image, add `loading="lazy"`, defer scripts, add `font-display: swap`
5. `interact(refresh, analyze: true)` → perf_diff shows improvement
6. `observe(vitals)` → LCP: 1.6s, CLS: 0.04 — both passing

**Features showcased:** Web Vitals, network waterfall, timeline correlation, perf_diff, measurable before/after

---

### Demo 5: "Test Suite from Zero" (4 min)
**Story:** No tests exist. AI creates a comprehensive test suite by using the app.

**Flow:**
1. AI navigates to home → products → product detail → add to cart → checkout
2. At each step: observes state, records assertions
3. `generate(test, test_name: "happy_path_checkout")` → full Playwright test
4. AI navigates edge cases: empty cart submission, invalid email, out-of-stock product
5. Generates tests for each error path
6. `observe(errors)` during edge cases captures real error messages for assertions
7. Output: 6 Playwright tests covering happy path + edge cases, grounded in real behavior

**Features showcased:** interact (full flow), observe (network + errors), test generation, reproduction scripts

---

### Demo 6: "The Full Monty — Ship Day" (5 min, combines everything)
**Story:** It's release day. AI does a complete pre-flight check.

**Flow:**
1. **Smoke test:** AI navigates the critical path, observes for errors → 2 bugs found → fixes both
2. **Accessibility:** `analyze(accessibility)` → 3 violations → fixes all
3. **Security:** `analyze(security_audit)` → generates and applies CSP
4. **Performance:** `observe(vitals)` → all green
5. **Link check:** `analyze(link_health)` → 1 broken link in footer → fixes href
6. **Test generation:** Generates tests for the critical path
7. **Noise cleanup:** `configure(noise_rule, noise_action: "auto_detect")` → suppresses 4 noise patterns
8. **Final sweep:** `observe(errors)` → clean. `observe(vitals)` → green. `analyze(accessibility)` → 0 violations.
9. Subtitle narration throughout: `"Checking accessibility..."`, `"Fixing security headers..."`, `"All clear — ship it."`

**Features showcased:** Everything. The full power of the tool in one narrative arc.

---

---

### Demo 7: "Browser Autopilot" (3 min)
**Story:** AI autonomously completes a multi-step user flow — form filling, navigation, validation — with no human touch.

**Flow:**
1. `interact(navigate, url: "http://localhost:3456/")` → AI loads the site
2. `interact(subtitle, text: "Navigating to products...")` → narration visible in browser
3. `interact(click, selector: "text=Products")` → navigates to products page
4. `interact(wait_for, selector: ".product-card")` → waits for products to load
5. `interact(click, selector: ".product-card")` → clicks first product
6. `interact(type, selector: "#product-quantity", text: "3", clear: true)` → sets quantity
7. `interact(click, selector: "text=Add to Cart")` → adds to cart
8. `interact(navigate, url: "/checkout")` → goes to checkout
9. `interact(type, selector: "#customer-name", text: "Jane Doe")` → fills form fields
10. `interact(type, selector: "#customer-email", text: "jane@example.com")`
11. `interact(click, selector: "text=Place Order")` → submits
12. `observe(errors)` → captures the 500 error
13. `observe(network_bodies)` → shows the malformed request payload
14. AI diagnoses and fixes the type mismatch bug

**Key point:** The entire flow is AI-driven. No human touches the browser. The AI is the QA tester.

**Features showcased:** Full interact suite (navigate, click, type, wait_for, subtitle), observe loop, autonomous debugging

---

### Demo 8: "Record & Replay" (3 min)
**Story:** Record a user session as video, capture all actions, replay for regression testing.

**Flow:**
1. `interact(screen_recording_start, name: "checkout_flow", fps: 30)` → starts video recording
2. AI performs the checkout flow (navigate → browse → add to cart → checkout)
3. `interact(screen_recording_stop)` → saves .webm video to disk
4. `observe(recordings)` → lists saved recordings with metadata (duration, size, fps)
5. `observe(recording_actions, recording_id: "checkout_flow")` → shows all captured actions
6. Later: replay the flow and compare results
7. `observe(log_diff_report)` → shows what changed between original and replay

**Key point:** This is regression testing via replay, not via written tests. Record once, detect drift forever.

**Features showcased:** screen_recording_start/stop, recordings, recording_actions, log_diff_report, video capture

---

### Demo 9: "Performance Lab" (4 min)
**Story:** Systematic performance testing — measure, change, measure again, prove improvement.

**Flow:**
1. `configure(test_boundary_start, test_id: "perf_baseline", label: "Before optimization")`
2. `interact(navigate, url: "/products")` → load the slow page
3. `observe(vitals)` → capture baseline: LCP 4.2s, FCP 2.1s, CLS 0.31
4. `observe(network_waterfall)` → identify: render-blocking analytics.js, 2s API delay, no lazy loading
5. `configure(test_boundary_end, test_id: "perf_baseline")`
6. AI applies fixes: defer scripts, add loading="lazy", set font-display: swap
7. `configure(test_boundary_start, test_id: "perf_after", label: "After optimization")`
8. `interact(refresh, analyze: true)` → reload with performance profiling
9. `observe(vitals)` → LCP: 1.4s, FCP: 0.8s, CLS: 0.02
10. `configure(test_boundary_end, test_id: "perf_after")`
11. Compare: measurable improvement with before/after boundaries

**Key point:** Test boundaries isolate performance data per experiment. This is a scientific performance lab, not just Lighthouse in a tab.

**Features showcased:** test_boundary_start/end, vitals, network_waterfall, perf_diff, interact(refresh, analyze: true)

---

### Demo 10: "Usability Audit" (3 min)
**Story:** AI explores the site as a user would, finding UX problems that aren't bugs but are bad experience.

**Flow:**
1. `interact(list_interactive)` → gets all clickable/focusable elements on the page
2. AI checks: are all interactive elements keyboard-reachable?
3. `analyze(accessibility, tags: ["wcag2aa"])` → WCAG violations affecting usability
4. `analyze(dom, selector: "input:not([aria-label]):not([id])")` → finds inputs without labels
5. AI navigates the full user journey via keyboard only: Tab, Enter, Escape
6. `interact(key_press, text: "Tab")` → tabs through the page
7. `observe(actions)` → records the tab navigation sequence
8. AI identifies: product cards aren't keyboard focusable, form inputs have no labels, no skip-to-content link
9. `interact(highlight, selector: ".product-card")` → visually highlights the inaccessible elements
10. AI fixes: adds tabindex, ARIA labels, skip link, focus styles

**Key point:** This isn't just a11y compliance — it's UX quality. AI tests the site like a real user with keyboard, screen reader needs, and attention to interactive affordances.

**Features showcased:** list_interactive, accessibility, dom queries, key_press, highlight, actions observation

---

### Demo 11: "Link Patrol" (2 min)
**Story:** Find every broken link, redirect loop, and dead page on the site.

**Flow:**
1. `interact(navigate, url: "/")` → start at home page
2. `analyze(link_health)` → async scan of all links on page (returns correlation_id)
3. `observe(command_result, correlation_id: "...")` → poll for results
4. Results show:
   - `/about` → 404 (broken link in footer)
   - `/privacy` → 404 (broken link in footer)
   - `/old-products` → redirect loop detected (→ /legacy-products → /old-products)
   - `/products`, `/cart` → 200 OK
5. `interact(navigate, url: "/products")` → scan next page
6. `analyze(link_health)` → scan product page links
7. AI compiles full report: 3 broken links, 1 redirect loop
8. AI fixes the footer HTML and removes the redirect loop route

**Key point:** Automated crawl-style link checking driven by AI. No external tool needed.

**Features showcased:** link_health, async command polling, navigate, structured link analysis

---

### Demo 12: "Annotation Mode — Guided Walkthrough" (2 min)
**Story:** AI creates a visual, narrated tour of the site — perfect for demos, onboarding, or QA review.

**Flow:**
1. `interact(navigate, url: "/", subtitle: "Starting the ShopBroken tour...")` → load home with narration
2. `interact(highlight, selector: ".hero", subtitle: "This is the hero section — notice the oversized background")` → yellow highlight + narration
3. `interact(highlight, selector: ".newsletter-form input", subtitle: "This email input has no label — accessibility violation")` → highlight problem area
4. `interact(navigate, url: "/products", subtitle: "Moving to the product listing...")` → navigate with narration
5. `interact(highlight, selector: ".sale-badge", subtitle: "Sale badges have white-on-yellow text — fails WCAG contrast")` → highlight the contrast issue
6. `interact(highlight, selector: ".product-card", subtitle: "These cards are <div> elements — not keyboard accessible")` → highlight structural issue
7. `interact(navigate, url: "/checkout", subtitle: "Now let's look at the checkout form...")` → move to checkout
8. `interact(highlight, selector: "#customer-name", subtitle: "No <label> element — only placeholder text for context")` → highlight form issue
9. `interact(subtitle, text: "Tour complete. 6 issues identified across 3 pages.")` → final narration

**Key point:** This creates a visual, recordable walkthrough. Combined with `screen_recording_start`, this produces a professional demo video with captions.

**Can be combined with recording:**
- `interact(screen_recording_start, name: "site_tour", fps: 30, audio: "tab")` at the start
- `interact(screen_recording_stop)` at the end
- Output: .webm video with subtitle narration baked in

**Features showcased:** subtitle (composable narration), highlight, navigate, recording integration

---

## Implementation

Demo site lives at `~/dev/kaboom-demos/new/shopbroken/`.

```bash
cd ~/dev/kaboom-demos/new/shopbroken
npm install
npm start
# → http://localhost:3456
```

See [shopbroken-spec.md](shopbroken-spec.md) for full technical specification of intentional bugs.
