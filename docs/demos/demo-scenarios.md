---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Kaboom Demo Scenarios

Real-world scenarios where Kaboom dramatically accelerates product development.

---

## Scenario 1: "The Mystery 500 Error"

**Context:** User reports checkout is broken. Developer can't reproduce locally.

**Without Kaboom:** Open DevTools, try to reproduce, check network tab manually, grep server logs, add console.logs, redeploy, repeat. 30-60 minutes.

**With Kaboom:**
1. AI navigates to checkout, fills the form, submits
2. `observe(error_bundles, window_seconds: 5)` → shows the 500 error + the POST body that caused it + the console warning about a null field
3. AI reads the API response body, identifies the missing validation
4. Fixes the server code, re-tests via interact, confirms the 200
5. **Time: 2 minutes**

**Key features:** error_bundles, interact, network_bodies

---

## Scenario 2: "Ship Accessible or Get Sued"

**Context:** Compliance deadline approaching, dozens of WCAG violations across the app.

**Without Kaboom:** Hire an a11y consultant, run axe manually page-by-page, create tickets, assign to developers, wait for fixes, re-audit. Weeks.

**With Kaboom:**
1. `analyze(accessibility, tags: ["wcag2aa"])` → structured violations with selectors, rule IDs, impact levels
2. AI iterates: reads violation → fixes code → re-scans → confirms fix
3. Each cycle is ~30 seconds
4. **47 violations resolved in minutes, not weeks**

**Key features:** accessibility audit, structured findings, iterative fix/verify loop

---

## Scenario 3: "What Broke After the Deploy?"

**Context:** New release deployed, users reporting issues but nothing obvious in logs.

**Without Kaboom:** Compare git diffs, check monitoring dashboards, try to reproduce user reports, deploy hotfix blindly. Hours.

**With Kaboom:**
1. Record the happy path before deploy
2. Deploy the change
3. Replay the flow, `observe(log_diff_report)` shows: 2 new errors, 1 missing API call, 3 changed response shapes
4. AI diagnoses each diff, generates reproduction scripts
5. **Time: 5 minutes to full diagnosis**

**Key features:** recording, playback, log_diff_report, reproduction generation

---

## Scenario 4: "Security Audit Before Launch"

**Context:** Pre-launch security review needed. No dedicated security team.

**Without Kaboom:** Hire a pentester or manually check OWASP checklist. Days to weeks. Expensive.

**With Kaboom:**
1. `analyze(security_audit)` → missing HSTS, insecure cookies, no CSP
2. `analyze(third_party_audit)` → 6 unknown third-party domains loading scripts
3. `generate(csp, mode: "strict")` → ready-to-deploy CSP header
4. `generate(sri)` → integrity hashes for all external scripts
5. AI applies all fixes, re-audits → clean
6. **Time: 10 minutes from audit to hardened**

**Key features:** security_audit, third_party_audit, CSP generation, SRI generation

---

## Scenario 5: "Generate a Test Suite from Nothing"

**Context:** Greenfield project or legacy app with zero test coverage. Management wants tests.

**Without Kaboom:** Manually write test boilerplate for each page/flow. Days of tedious work. Tests often miss edge cases because they're written from spec, not behavior.

**With Kaboom:**
1. AI navigates every user flow (signup → login → dashboard → settings → checkout)
2. At each step: interacts, observes real responses, captures real error messages
3. Generates Playwright tests grounded in actual behavior
4. Navigates edge cases (empty inputs, invalid data, out-of-stock items)
5. **Output: 15+ tests in 10 minutes, covering real paths with real assertions**

**Key features:** interact (full browser control), observe (network + errors), test generation

---

## Scenario 6: "Why Is It So Slow?"

**Context:** Users complaining about performance. PageSpeed score is 42.

**Without Kaboom:** Open Lighthouse in DevTools, read the wall of suggestions, guess which matters most, try fixes one at a time, re-run Lighthouse each time. Hours.

**With Kaboom:**
1. `observe(vitals)` → LCP: 4.2s (should be <2.5s), CLS: 0.31 (should be <0.1)
2. `observe(network_waterfall)` → 3MB uncompressed JS, render-blocking script in `<head>`
3. `observe(timeline)` → image carousel loading 12 full-res images on mount, font swap at 1.2s causing layout shift
4. AI identifies the bottlenecks, applies targeted fixes
5. `interact(refresh, analyze: true)` → perf_diff confirms improvement
6. **LCP: 4.2s → 1.6s. CLS: 0.31 → 0.04. Time: 5 minutes.**

**Key features:** Web Vitals, network waterfall, timeline correlation, perf_diff

---

## Scenario 7: "Noise-Free Debugging"

**Context:** Console has 200+ errors. Most are from analytics scripts, browser extensions, and React dev warnings. Real bugs are buried.

**Without Kaboom:** Manually scroll through console, mentally filter noise, miss the real error among the spam.

**With Kaboom:**
1. `configure(noise_rule, noise_action: "auto_detect")` → Kaboom analyzes the buffer, suggests 8 noise rules with >90% confidence
2. AI applies the rules
3. `observe(errors)` → 3 real errors remain, all actionable
4. **Signal-to-noise ratio goes from 1.5% to 100%**

**Key features:** noise auto-detection, noise rule management, filtered error observation

---

## Scenario 8: "AI as QA Tester"

**Context:** No QA team. Developers ship and hope for the best.

**Without Kaboom:** Manual testing. Click around, check for obvious errors, miss edge cases. Ship with fingers crossed.

**With Kaboom:**
1. AI autonomously navigates every page via interact (click, type, navigate)
2. At each step, observes errors, network failures, and vitals
3. Tests edge cases: empty inputs, zero quantities, missing products, rapid clicks
4. Generates a full test suite from the session
5. **Result: Automated QA pass with generated regression tests, zero manual effort**

**Key features:** interact (full browser control), observe (errors + network), test generation

---

## Scenario 9: "Record the Demo, Not the Bug"

**Context:** Marketing needs a product demo video. PM wants a walkthrough for stakeholders.

**Without Kaboom:** Screen record manually, write a script, re-record when you mess up. Edit in post.

**With Kaboom:**
1. `interact(screen_recording_start, name: "product_demo", fps: 30)` → recording starts
2. AI navigates the happy path with subtitle narration at each step
3. `interact(highlight)` on key features as AI explains them via subtitle
4. `interact(screen_recording_stop)` → .webm video saved with all narration baked in
5. **Result: Professional demo video created entirely by AI. No human on camera.**

**Key features:** screen_recording_start/stop, subtitle narration, highlight, navigate

---

## Scenario 10: "Broken Links Before Launch"

**Context:** Marketing updated the site copy. Some internal links now point to pages that were renamed or removed.

**Without Kaboom:** Click every link manually. Miss the ones in the footer. Ship with broken links.

**With Kaboom:**
1. `analyze(link_health)` on every page → finds all 404s, redirect loops, timeout links
2. AI compiles a report: 3 broken links, 1 redirect loop, 2 slow external resources
3. AI fixes the broken hrefs in the source code
4. Re-scan confirms all links healthy
5. **Result: Zero broken links. 2-minute audit instead of 2-hour manual check.**

**Key features:** link_health, async command polling, automated fix verification

---

## Scenario 11: "Performance Budget Enforcement"

**Context:** Team agreed on performance budgets (LCP < 2.5s, CLS < 0.1) but nobody enforces them.

**Without Kaboom:** Run Lighthouse manually before each PR. Forget half the time. Budgets drift.

**With Kaboom:**
1. `observe(vitals)` → structured metrics with pass/fail against targets
2. `observe(network_waterfall)` → identifies the specific resources causing slowness
3. Test boundaries isolate each test scenario's performance data
4. AI can run before/after comparisons on every code change
5. **Result: Automated performance gate. Every change measured. Regressions caught before merge.**

**Key features:** vitals, test boundaries, perf_diff, network waterfall

---

## Scenario 12: "Annotated Code Review"

**Context:** Reviewing a PR that changes the checkout flow. Hard to verify visually from code alone.

**Without Kaboom:** Check out the branch, run the app, click through manually, try to remember what changed.

**With Kaboom:**
1. AI navigates the changed flow via interact
2. Uses highlight to visually mark the affected elements
3. Subtitle narrates what each change does as the AI tests it
4. Observes errors, vitals, and network to verify nothing broke
5. `generate(pr_summary)` → structured summary of what changed and what was verified
6. **Result: Visual, narrated code review. Reviewer sees exactly what the change looks like in the browser.**

**Key features:** highlight, subtitle, navigate, pr_summary generation
