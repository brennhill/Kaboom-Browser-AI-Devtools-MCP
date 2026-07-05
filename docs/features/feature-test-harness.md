---
status: partially_implemented
scope: feature/test-harness/implementation
ai-priority: high
tags: [implementation, testing, architecture]
relates-to: []
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-test-harness
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Technical Spec: Deterministic Local Test Harness (Smoke Test V2)

## Purpose

Currently, Kaboom's smoke tests and integration tests rely heavily on external websites like `example.com`. While this proves the tool works on the open web, it introduces severe reliability issues: external sites change their DOM, experience downtime, or throttle requests, leading to flaky tests. Furthermore, we cannot reliably trigger edge cases like specific Web Vitals regressions, complex accessibility violations, or React-specific event traps on generic external sites.

The goal is to build a deterministic, self-hosted test harness served from `tests/pages/` that contains a suite of deliberately vulnerable, noisy, and complex web pages. This will serve as the definitive proving ground for every single tool and capability in the Kaboom MCP suite.

External sites (like Binance, X/Twitter, or LinkedIn) should be the exception, reserved strictly for testing advanced anti-bot evasion or cross-origin restrictions in real-world scenarios.

---

## Opportunity & Business Value

1. **Zero Flakiness:** Tests become 100% deterministic. If a test fails, it is guaranteed to be a regression in Kaboom, not a change in a third-party website.
2. **Offline Capable:** CI pipelines and local development won't require external network access for the core test suite.
3. **Edge Case Validation:** We can explicitly program pages to fail in specific ways (e.g., a button that only appears after 3 seconds, a script that throws an `UnhandledPromiseRejection` on click, a massive CLS shift).
4. **LLM Training Ground:** These pages will serve as perfect examples of "what bad looks like" for LLMs to practice diagnosing and fixing using the MCP tools.

---

## The Test Pages Suite

We will build a set of static/lightweight HTML/JS pages under `tests/pages/`. Each page will isolate and demonstrate specific failure modes and feature targets.

### 1. The Interaction Gauntlet (`/tests/pages/interact.html`)
**Focus:** Form filling, modals, and event traps.
* **React-style Input Traps:** An input field that tracks state internally and ignores direct `element.value = "foo"` assignments (requires proper `input` and `change` event dispatching).
* **Obfuscated Selectors:** Dynamic class names (e.g., `class="css-1a2b3c"`) to test semantic selector fallback (`text=Submit`).
* **Overlays & Modals:** A button hidden behind a `z-index: 9999` overlay, and a native `window.alert()` that fires on page load to test modal dismissal.
* **Anti-Bot Simulation:** An invisible honeypot button that overlaps the real submit button to test coordinate/CDP clicking.

### 2. The Performance Nightmare (`/tests/pages/performance.html`)
**Focus:** Web Vitals and Long Tasks.
* **Delayed LCP:** A massive hero image that is artificially delayed by 2 seconds using a service worker or backend delay.
* **Terrible CLS:** A DOM element that resizes itself 1.5 seconds after load, pushing content down to trigger a Layout Shift.
* **Main Thread Blocking:** A button that runs a heavy `while` loop for 800ms to trigger an explicitly measurable Long Task and poor Interaction to Next Paint (INP).

### 3. The Noise Factory (`/tests/pages/telemetry.html`)
**Focus:** Logs, Errors, and Network observation.
* **Console Spam:** Emits 100+ `console.log` messages mimicking React Strict Mode and Webpack HMR to test noise filtering rules.
* **Network Errors:** Attempts to `fetch()` a 404, a 500, and a CORS-blocked external resource to populate the `network_waterfall` and `network_bodies` buffers.
* **WebSocket Chaos:** Opens a WebSocket connection, sends binary data, and then abruptly closes it with an abnormal code to test connection state tracking.
* **Unhandled Exceptions:** A button that triggers a deep stack trace error (`TypeError: undefined is not a function`).

### 4. The Accessibility Disaster (`/tests/pages/a11y.html`)
**Focus:** Testing the `accessibility` and `sarif` generation tools.
* **Color Contrast:** Light grey text on a white background.
* **Missing ARIA:** Buttons without text or `aria-label` attributes.
* **Form Labels:** Inputs completely detached from their labels.
* **Structure:** Missing `<main>` landmark and out-of-order heading levels (`<h1>` followed directly by `<h4>`).

---

## Architecture & Infrastructure

### The Test Server
We will use a lightweight local server to host the `tests/pages/` directory during test runs. 
* **Implementation:** A simple Python `http.server`, a small Go file server, or `vite preview`.
* **Lifecycle:** Spun up at the start of `smoke-test.sh` and torn down automatically via `trap`.

Implemented as:
* `scripts/smoke-tests/harness-server.py` (ThreadingHTTPServer)
* Root: `tests/pages/`
* Deterministic API endpoints:
  * `/healthz`
  * `/api/status/404`
  * `/api/status/500`
  * `/api/slow`

### Refactoring `smoke-test.sh`
The existing smoke tests will be updated to point to `http://localhost:8080/interact.html` (or similar) instead of `https://example.com`. 

The test validations will be tightened. Instead of checking if `example.com` loaded, we will assert that Kaboom specifically caught the 800ms long task on `performance.html` or successfully navigated the React event trap on `interact.html`.

Implemented migration strategy:
* `scripts/smoke-tests/framework-smoke.sh` now starts the local harness automatically.
* Smoke requests are URL-rewritten at the framework layer so legacy `https://example.com` navigations are routed to local deterministic pages under `tests/pages/example.com/`.

### The "Real World" Exceptions
We will maintain a dedicated `external-evasion.sh` module for tests that *must* run against the open web to prove resilience against commercial anti-bot systems:
* **X/Twitter:** For testing CDP/Hardware clicks (`chrome.debugger`) against highly defended SPAs.
* **Binance/Stripe:** For testing network interception behind heavy WAFs (Cloudflare/Datadome).

---

## Execution Plan

Current phase status:
* Phase 1: implemented
* Phase 2: partially implemented (framework rewrite is in place; per-module assertion hardening remains)
* Phase 3: pending

1. **Phase 1: Create the Test Server & Pages**
   * Setup the local web server in the Makefile/test scripts.
   * Build the 4 core HTML/JS pages outlined above.
2. **Phase 2: Migrate Smoke Tests**
   * Rewrite `scripts/smoke-tests/*.sh` to target the local pages.
   * Enhance assertions to look for the specific, deterministic failures we programmed into the pages.
3. **Phase 3: CI Integration**
   * Ensure GitHub Actions spins up the test server before running the end-to-end tests.
   * Verify that the suite passes 100% reliably in a disconnected CI runner environment.
