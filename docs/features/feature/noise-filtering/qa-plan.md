---
status: proposed
scope: feature/noise-filtering/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-noise-filtering
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Noise Filtering

> QA plan for the Noise Filtering feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Noise filtering suppresses entries from MCP tool responses -- the primary risk is that security-relevant entries get hidden, or that filtered entries remain accessible through alternate paths.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Auth failures hidden by noise rules | A noise rule matching a domain (e.g., `analytics.example.com`) must NOT suppress 401/403 responses from that same domain | critical |
| DL-2 | Application errors suppressed by auto-detect | Auto-detected rules must never suppress console errors from application sources (localhost, app domain) | critical |
| DL-3 | Dismissed patterns hiding security events | A `dismiss_noise` call with a broad regex (e.g., `.*error.*`) could suppress security-relevant console errors | critical |
| DL-4 | Noise statistics leaking internal URLs | The `list` action returns rule patterns and statistics -- verify no sensitive internal URLs are captured in auto-detected rule patterns | high |
| DL-5 | Built-in rules hiding real failures | Built-in rules matching `favicon.ico` or source maps should not suppress 5xx errors on those paths | high |
| DL-6 | Filtered data still in raw buffers | Noise filtering is read-time only -- raw buffer data still exists in memory. Verify no MCP tool exposes raw unfiltered buffers to the agent | high |
| DL-7 | Auto-detect confidence threshold too low | Auto-applied rules at 0.9 confidence could suppress legitimate repeating entries (e.g., polling API responses) | medium |
| DL-8 | Custom regex capturing too broadly | User-added regex rules could inadvertently match application entries beyond the intended noise pattern | medium |
| DL-9 | Noise rule reason field leaking sensitive info | The `reason` field on dismissed patterns is free-text -- verify it does not appear in contexts where it could leak | low |

### Negative Tests (must NOT leak)
- [ ] 401/403 network responses are NEVER filtered regardless of any noise rule matching the URL
- [ ] Console errors from application sources (localhost, 127.0.0.1, app domain) are never auto-detected as noise
- [ ] 5xx server errors on any URL are never classified as noise by built-in rules
- [ ] Raw unfiltered buffer contents are not exposed through any MCP tool response
- [ ] Auto-detected rules never suppress entries with fewer than 10 occurrences
- [ ] The noise rule `list` response does not include the actual text content of filtered entries

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Rule ID prefix semantics | AI can distinguish `builtin_`, `user_`, `auto_`, `dismiss_` prefixes and understand their immutability implications | [ ] |
| CL-2 | Classification labels are unambiguous | Labels "extension", "framework", "cosmetic", "analytics", "infrastructure", "repetitive", "dismissed" are clearly distinct | [ ] |
| CL-3 | Auto-detect confidence values | AI understands that confidence >= 0.9 means auto-applied, < 0.9 means suggestion only | [ ] |
| CL-4 | Filtered vs. excluded terminology | Tool responses use consistent terminology (always "filtered" not sometimes "excluded" or "hidden") | [ ] |
| CL-5 | Statistics include both filtered and total counts | AI can compute the noise ratio (filtered / total) from the response without ambiguity | [ ] |
| CL-6 | Rule category maps clearly to buffer type | "console" = console logs, "network" = network requests, "websocket" = WebSocket events -- no overlap or confusion | [ ] |
| CL-7 | Error messages on failed operations | Error messages clearly state what went wrong (e.g., "Cannot remove built-in rule: builtin_favicon") | [ ] |
| CL-8 | Auto-detect proposals distinguish applied vs. suggested | Response clearly separates rules that were auto-applied (>= 0.9) from suggestions (< 0.9) | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might interpret "noise" as "all non-error entries" -- verify descriptions clarify noise means browser/tooling overhead, not application info-level logs
- [ ] AI might try to remove built-in rules and not understand why it fails -- verify error message suggests using `dismiss_noise` or custom rules instead
- [ ] AI might misinterpret the confidence score as a probability of the entry being noise vs. a probability the rule is correct -- verify the auto-detect response explains the confidence semantics
- [ ] AI might think `reset` removes ALL rules including built-ins -- verify response confirms built-in rules are preserved
- [ ] AI might confuse `configure_noise(action: "remove")` (removes a rule) with `dismiss_noise` (adds a suppress rule) -- verify tool descriptions make the difference clear

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Add a custom noise rule | 1 step: `configure_noise(action: "add", rules: [...])` | No -- already minimal |
| Quick dismiss a pattern | 1 step: `dismiss_noise(pattern: "...", reason: "...")` | No -- already minimal |
| Run auto-detection | 1 step: `configure_noise(action: "auto_detect")` | No -- already minimal |
| List all rules | 1 step: `configure_noise(action: "list")` | No -- already minimal |
| Remove a custom rule | 2 steps: list rules to find ID, then `configure_noise(action: "remove", rule_id: "...")` | Could accept pattern instead of ID, but ID is more precise |
| Reset to defaults | 1 step: `configure_noise(action: "reset")` | No -- already minimal |
| Full noise setup for a session | 2 steps: `auto_detect` then review/add remaining rules | No -- appropriately scoped |

### Default Behavior Verification
- [ ] Feature works with zero configuration -- built-in rules are active immediately on server start
- [ ] No MCP tool call needed to activate noise filtering -- it is always on
- [ ] Default built-in rules cover the most common browser noise without user intervention
- [ ] Framework-specific rules activate automatically upon framework detection

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | NewNoiseConfig creates all built-in rules | Call `NewNoiseConfig()` | Config has ~50 built-in rules across console, network, websocket categories | must |
| UT-2 | Chrome extension source matched as noise | Console entry with source `chrome-extension://abc/content.js` | `IsConsoleNoise()` returns true, classification = "extension" | must |
| UT-3 | Application error from localhost not noise | Console error from `http://localhost:3000/app.js` | `IsConsoleNoise()` returns false | must |
| UT-4 | Favicon 404 matched as noise | Network entry: GET `/favicon.ico` 404 | `IsNetworkNoise()` returns true | must |
| UT-5 | API endpoint 500 not noise | Network entry: GET `/api/users` 500 | `IsNetworkNoise()` returns false | must |
| UT-6 | Segment analytics URL matched | Network entry: POST `https://api.segment.io/v1/track` | `IsNetworkNoise()` returns true, classification = "analytics" | must |
| UT-7 | CORS preflight OPTIONS matched | Network entry: OPTIONS `/api/data` 204 | `IsNetworkNoise()` returns true | must |
| UT-8 | Vite HMR message matched | Console entry: `[vite] hot updated: /src/App.tsx` | `IsConsoleNoise()` returns true, classification = "framework" | must |
| UT-9 | React DevTools prompt matched | Console entry: `Download the React DevTools...` | `IsConsoleNoise()` returns true | must |
| UT-10 | Add custom rule increases rule count | `AddNoiseRules([]NoiseRule{...})` | Rule count increases by 1, rule has `user_` prefix ID | must |
| UT-11 | Remove custom rule decreases count | `RemoveNoiseRule("user_1")` | Rule count decreases by 1 | must |
| UT-12 | Cannot remove built-in rule | `RemoveNoiseRule("builtin_favicon")` | Error returned, rule still present | must |
| UT-13 | Max 100 rules enforced | Add rules until count reaches 100, then add one more | Extra rule silently dropped, count remains 100 | must |
| UT-14 | Reset removes only user/auto rules | Add user rules, call `ResetNoiseRules()` | Only built-in rules remain | must |
| UT-15 | Invalid regex skipped silently | Add rule with pattern `[invalid` | Rule has nil compiled regex, never matches, no panic | must |
| UT-16 | Auth 401 response never filtered | Network entry: 401 on `https://analytics.google.com/collect` | `IsNetworkNoise()` returns false despite URL matching analytics rule | must |
| UT-17 | Auth 403 response never filtered | Network entry: 403 on any noise-matching URL | `IsNetworkNoise()` returns false | must |
| UT-18 | dismiss_noise creates rule with correct defaults | `DismissNoise(pattern, "", "")` | Rule created with category="console", classification="dismissed", `dismiss_` prefix | should |
| UT-19 | dismiss_noise with network category | `DismissNoise(pattern, "network", reason)` | Rule created with category="network" | should |
| UT-20 | Auto-detect frequent messages | Buffer with 15 identical console messages | Proposed rule with confidence > 0.7 | should |
| UT-21 | Auto-detect skips existing patterns | Buffer already filtered by existing rule | No duplicate rule proposed | should |
| UT-22 | Auto-detect high confidence auto-applies | Pattern with confidence >= 0.9 | Rule automatically added to config | should |
| UT-23 | Statistics track per-rule filter counts | Filter 5 entries with rule X, 3 with rule Y | Stats show X: 5, Y: 3 | should |
| UT-24 | Source map 404 matched as noise | Network entry: GET `/app.js.map` 404 | `IsNetworkNoise()` returns true | should |
| UT-25 | HMR WebSocket URL matched | WebSocket URL containing `webpack-dev-server` | `IsWebSocketNoise()` returns true | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Noise filtering applied to observe tool responses | NoiseConfig + observe tool handler | `observe` returns only non-noise entries | must |
| IT-2 | configure_noise add/list round-trip | MCP tool dispatcher + NoiseConfig | Add a rule via MCP, list returns it | must |
| IT-3 | dismiss_noise creates and applies rule | MCP tool dispatcher + NoiseConfig | Dismissed pattern no longer appears in observe output | must |
| IT-4 | Auto-detect via MCP tool | MCP tool dispatcher + NoiseConfig + buffers | Returns proposals with confidence scores, high-confidence ones auto-applied | must |
| IT-5 | Noise rules survive session within same process | NoiseConfig + server lifecycle | Rules added early in session still active later | must |
| IT-6 | Framework detection activates framework rules | Network/console ingest + NoiseConfig | After detecting React, React-specific noise rules become active | should |
| IT-7 | Concurrent noise rule add and buffer read | NoiseConfig RWMutex | No race condition (go test -race passes) | must |
| IT-8 | Health/diagnostics include noise statistics | NoiseConfig + health endpoint | Stats show total filtered, per-rule counts, last signal/noise times | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Match single entry against all rules (100 rules) | Latency per entry | < 0.1ms | must |
| PT-2 | Recompile all patterns after rule change | Recompile time | < 5ms | must |
| PT-3 | Auto-detect full buffer scan (1000 entries) | Scan time | < 50ms | must |
| PT-4 | Memory usage for 100 compiled regex rules | Memory | < 100KB | should |
| PT-5 | Match 1000 entries against 50 rules (batch) | Total batch time | < 50ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Empty buffer with noise rules | Call auto-detect on empty buffers | Returns empty proposals, no crash | must |
| EC-2 | Rule with empty pattern | Add rule with pattern="" | Rule never matches (or rejected at add time) | must |
| EC-3 | Unicode in console message matching | Console message with unicode characters, regex with unicode | Correct regex matching behavior | should |
| EC-4 | Very long console message (10KB) | Console entry with 10KB message body | Matching completes within performance budget | should |
| EC-5 | Concurrent auto-detect and rule add | Two goroutines: one running auto-detect, one adding rules | No race condition, consistent rule state | must |
| EC-6 | Rule matching both console and network | Rule with category "console" tested against network entry | Does not match (category scoping enforced) | must |
| EC-7 | 401 on non-noise URL | GET `/api/secure-resource` 401 | Not filtered (auth responses always preserved) | must |
| EC-8 | Network entry with empty URL | Network entry with URL="" | No panic, entry not matched by URL-pattern rules | should |
| EC-9 | Auto-detect periodicity with jitter | Requests at 10s intervals with 8% jitter | Detected as periodic (within 10% tolerance) | should |
| EC-10 | Auto-detect periodicity with high jitter | Requests at irregular intervals (>10% jitter) | Not detected as periodic | should |
| EC-11 | Multiple framework detections | Page uses both React and Vite | Both framework rule sets activated | should |
| EC-12 | dismiss_noise with invalid regex | `dismiss_noise(pattern: "[invalid")` | Rule created but with nil regex, never matches, no panic | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application running locally (e.g., a React/Vite app on localhost:3000)
- [ ] Application generates both real errors and typical browser noise (extension warnings, favicon 404s, etc.)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"action": "noise_rule", "noise_action": "list"}}` | Review the rule list in response | Response contains ~50 built-in rules with `builtin_` prefixes across console, network, websocket categories | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"action": "get_console_logs"}}` | Open browser DevTools, compare console entries | Kaboom output excludes chrome-extension warnings, DevTools prompts, and HMR messages that appear in the browser console | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"action": "get_network"}}` | Check browser Network tab | Kaboom output excludes favicon.ico requests, analytics pings, and CORS preflights visible in browser | [ ] |
| UAT-4 | Trigger a real application error (e.g., navigate to a broken route) | See the error in browser console | The application error appears in Kaboom's observe output (NOT filtered) | [ ] |
| UAT-5 | Trigger a 401 response on an analytics-matching URL | Monitor network tab | The 401 response appears in Kaboom's output despite the URL matching an analytics noise rule | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"action": "noise_rule", "noise_action": "add", "rules": [{"category": "console", "match": {"message_pattern": "specific-app-warning"}, "classification": "cosmetic"}]}}` | None | Response confirms rule added with `user_` prefix ID | [ ] |
| UAT-7 | `{"tool": "observe", "arguments": {"action": "get_console_logs"}}` | Trigger the "specific-app-warning" message in browser | The warning no longer appears in Kaboom output | [ ] |
| UAT-8 | `{"tool": "configure", "arguments": {"action": "noise_rule", "noise_action": "remove", "rule_id": "<user_rule_id>"}}` | None | Rule removed successfully | [ ] |
| UAT-9 | `{"tool": "observe", "arguments": {"action": "get_console_logs"}}` | Trigger the same warning | The warning now appears again in Kaboom output | [ ] |
| UAT-10 | `{"tool": "configure", "arguments": {"action": "noise_rule", "noise_action": "remove", "rule_id": "builtin_favicon"}}` | None | Error returned: cannot remove built-in rule | [ ] |
| UAT-11 | `{"tool": "configure", "arguments": {"action": "dismiss", "pattern": "repetitive-log-message", "reason": "Noisy during testing"}}` | None | Rule created with `dismiss_` prefix, classification "dismissed" | [ ] |
| UAT-12 | `{"tool": "configure", "arguments": {"action": "noise_rule", "noise_action": "auto_detect"}}` | Generate repetitive console messages (>10 identical) before calling | Response includes proposals; high-confidence (>=0.9) ones marked as auto-applied | [ ] |
| UAT-13 | `{"tool": "configure", "arguments": {"action": "noise_rule", "noise_action": "reset"}}` | None | All user/auto/dismiss rules removed; only built-in rules remain | [ ] |
| UAT-14 | `{"tool": "configure", "arguments": {"action": "noise_rule", "noise_action": "list"}}` | Compare rule count to UAT-1 | Rule count matches original built-in count (no user/auto rules) | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | 401/403 never filtered | Make a request that returns 401 to a URL matching a noise rule pattern, then call observe | 401 response appears in output | [ ] |
| DL-UAT-2 | Application errors never auto-detected | Generate 20+ identical console errors from app code, run auto-detect | No rule proposed for the application error pattern | [ ] |
| DL-UAT-3 | 5xx responses not filtered | Trigger a 500 error on a URL that partially matches a noise pattern | 500 response appears in output | [ ] |
| DL-UAT-4 | Noise stats do not contain entry text | Call list action and inspect statistics | Stats contain counts and rule IDs only, not the actual text of filtered entries | [ ] |

### Regression Checks
- [ ] Existing observe tool functionality works identically when no custom noise rules are configured
- [ ] Adding and removing noise rules does not affect buffer sizes or memory usage
- [ ] Noise filtering does not add observable latency to observe tool responses (< 0.1ms per entry)
- [ ] Server startup time is not significantly affected by noise rule compilation

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
