---
status: proposed
scope: feature/budget-thresholds/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-budget-thresholds
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Budget Thresholds

> QA plan for the Budget Thresholds feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Budget thresholds read from a project config file and emit alerts about performance metrics. The primary risks are that config file paths leak project structure, that budget violation alerts leak application URLs or architecture, and that the file-watching mechanism could be abused.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Config file path revealed in responses | The `config_file` field in `check_budgets` response shows the path to `.kaboom.json` -- verify this is a relative path, not an absolute path that reveals directory structure | medium |
| DL-2 | Application URLs exposed in violation alerts | Violation alerts include the URL that exceeded the budget (e.g., `/dashboard`, `/login`) -- verify these are path-only, not full URLs with query strings or tokens | high |
| DL-3 | Route patterns reveal application architecture | The config file's `routes` map reveals the application's URL structure -- since the file is project-local and Kaboom is localhost-only, verify this is acceptable | low |
| DL-4 | Budget values reveal business expectations | Budget thresholds (e.g., 1s load for `/login`) reveal what performance the team considers acceptable -- verify this is acceptable for a development tool | low |
| DL-5 | Config file watch reads arbitrary files | The 30-second file watch checks `.kaboom.json` modification time -- verify it ONLY reads this specific file and cannot be redirected via symlinks | high |
| DL-6 | Invalid config error messages leak file content | If `.kaboom.json` contains malformed JSON, verify the error message does not echo raw file content in MCP responses | medium |
| DL-7 | Performance snapshot data in violation alerts | Violation alerts include `actual` metric values (e.g., `actual: 620` for transfer KB) -- verify this does not reveal sensitive data about the application's network traffic | medium |
| DL-8 | `recommendation` field in alerts | Recommendations like "Reduce bundle size or increase budget in .kaboom.json" are generic -- verify they never include application-specific data | low |
| DL-9 | Preset names reveal Kaboom-internal knowledge | Preset names like `web-vitals-good` are public knowledge -- verify no internal-only presets leak proprietary thresholds | low |

### Negative Tests (must NOT leak)
- [ ] Config file path in responses is relative (`.kaboom.json`), not absolute (`/Users/dev/project/.kaboom.json`)
- [ ] Violation alert URLs are path-only (e.g., `/dashboard`), not full URLs with query parameters or auth tokens
- [ ] Invalid config error messages say "Invalid JSON in budget configuration" without echoing the file content
- [ ] Config file watch uses only the specified path, not following symlinks to arbitrary locations
- [ ] Performance snapshot metric values in alerts are aggregate numbers (load_ms, transfer_kb), not raw network body content
- [ ] The `check_budgets` tool does not expose the raw contents of `.kaboom.json` -- only parsed budget definitions

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Budget vs. regression distinction | AI understands `budget_exceeded` (absolute threshold) vs. `regression_detected` (relative change) are different alert types | [ ] |
| CL-2 | `over_by` format is unambiguous | Value like "120KB (24%)" clearly shows both absolute delta and percentage | [ ] |
| CL-3 | Metric names are self-documenting | `load_ms`, `fcp_ms`, `lcp_ms`, `cls`, `inp_ms`, `ttfb_ms`, `total_transfer_kb`, `script_transfer_kb`, `image_transfer_kb` are unambiguous | [ ] |
| CL-4 | `budget` vs. `actual` fields | AI understands `budget` is the threshold and `actual` is the measured value | [ ] |
| CL-5 | Route resolution is transparent | Response indicates which route matched (default vs. specific route) | [ ] |
| CL-6 | "No budget configuration found" is clear | AI understands this means the `.kaboom.json` file is missing, not that budgets are set to zero | [ ] |
| CL-7 | Preset semantics | AI understands presets provide fallback thresholds that config values override | [ ] |
| CL-8 | `show_passing` parameter behavior | AI understands passing=false (default) shows only violations, passing=true shows all metrics | [ ] |
| CL-9 | Budget of 0 means "no budget" | AI understands 0 is not a threshold, not "metric must be zero" | [ ] |
| CL-10 | Summary field is human-readable | The `summary` field in responses provides a concise natural-language summary | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might confuse budget thresholds (absolute) with regression detection (relative) -- verify alert types are clearly labeled
- [ ] AI might think budget_exceeded means the application is broken -- verify the summary explains it is a threshold violation, not a crash
- [ ] AI might not know how to fix a budget violation -- verify `recommendation` field provides actionable next steps
- [ ] AI might think budget=0 means "must be zero" instead of "no budget" -- verify documentation and error handling clarify
- [ ] AI might try to set budgets via MCP tool call instead of editing `.kaboom.json` -- verify response mentions the config file approach
- [ ] AI might not understand longest-prefix-match for routes -- verify route resolution is explained when a route-specific budget applies

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low-Medium (configuration is file-based, checking is single-tool)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Set up budget thresholds | 1 step: create/edit `.kaboom.json` with budget object | No -- file-based config is standard practice |
| Check all budgets | 1 step: `check_budgets()` | No -- already minimal |
| Check budget for specific URL | 1 step: `check_budgets(url: "/dashboard")` | No -- already minimal |
| See passing and failing | 1 step: `check_budgets(show_passing: true)` | No -- already minimal |
| Use preset thresholds | 1 step: add `"presets": ["web-vitals-good"]` to config | No -- already minimal |
| Get budget alerts from observe | 0 steps: alerts appear automatically in `get_changes_since` response | N/A -- fully automatic |
| Update budget config | 1 step: edit `.kaboom.json`, changes detected within 30 seconds | No -- file watch is automatic |

### Default Behavior Verification
- [ ] Feature works with zero configuration -- without `.kaboom.json`, budget enforcement is simply disabled (not errored)
- [ ] No MCP tool call is needed to enable budget checking; creating the config file is sufficient
- [ ] Budget violations appear automatically in `get_changes_since` alerts without opt-in
- [ ] Built-in presets are available without any setup
- [ ] Unknown metric names in config are silently ignored (forward-compatible)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Metric within budget | load_ms=1500, budget=2000 | No violation generated | must |
| UT-2 | Metric exceeds budget | load_ms=2500, budget=2000 | Violation: actual=2500, budget=2000, over_by="500ms (25%)" | must |
| UT-3 | Route-specific budget overrides default | URL="/dashboard", route budget=3000, default=2000 | Budget=3000 used for /dashboard | must |
| UT-4 | Longest prefix match | URL="/dashboard/charts/revenue", routes: {"/dashboard": 3000, "/dashboard/charts": 3500} | `/dashboard/charts` matched (longer prefix) | must |
| UT-5 | Fallback to default when no route match | URL="/settings", no route for "/settings" | Default budget used | must |
| UT-6 | Preset "web-vitals-good" thresholds | Apply preset | FCP=1800, LCP=2500, CLS=0.1, INP=200 | must |
| UT-7 | Config value overrides preset | Config: lcp_ms=3000, preset: web-vitals-good (lcp=2500) | LCP budget = 3000 (config wins) | must |
| UT-8 | No config file | Config load with missing file | Budget enforcement disabled, no error | must |
| UT-9 | Invalid JSON in config | Config file with malformed JSON | Warning logged, no budgets active | must |
| UT-10 | Budget of 0 for a metric | `load_ms: 0` in config | No budget for load_ms (0 = no budget) | must |
| UT-11 | Unknown metric name in config | `unknown_metric: 100` in config | Silently ignored, other metrics processed | must |
| UT-12 | Multiple violations on same URL | load_ms=3000 (budget=2000), lcp_ms=4000 (budget=2500) | Both violations listed in alert | must |
| UT-13 | First load exceeds budget (no baseline) | No prior snapshots, first snapshot exceeds budget | Alert generated immediately | must |
| UT-14 | Violation cleared on improvement | Metric improves below budget | Previous violation cleared | must |
| UT-15 | Preset "web-vitals-needs-improvement" | Apply preset | FCP=3000, LCP=4000, CLS=0.25, INP=500 | should |
| UT-16 | Preset "performance-budget-default" | Apply preset | Load=3000, TTFB=600, transfer=1MB, scripts=500KB | should |
| UT-17 | Multiple presets merged in order | Presets: ["web-vitals-good", "performance-budget-default"] | Both sets of thresholds present, later overrides earlier for same metric | should |
| UT-18 | CLS threshold (float comparison) | CLS actual=0.12, budget=0.1 | Violation detected (0.12 > 0.1) | must |
| UT-19 | CLS within budget (float edge) | CLS actual=0.1, budget=0.1 | No violation (exactly at budget) | must |
| UT-20 | Transfer size budget in KB | total_transfer_kb actual=620, budget=500 | Violation: over by 120KB (24%) | must |
| UT-21 | `check_budgets` with no URL (all URLs) | No URL parameter | Checks all URLs with recent snapshots | should |
| UT-22 | `check_budgets` with specific URL | URL="/login" | Only /login checked | should |
| UT-23 | `show_passing` includes within-budget metrics | Metric within budget, show_passing=true | Metric appears in `passing` array | should |
| UT-24 | Config file change detection | Modify file mtime, wait 30s | New config loaded, budgets re-evaluated | should |
| UT-25 | Config appears mid-session | No config at startup, create file later | Picked up on next 30-second check | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Budget violations in `get_changes_since` | Budget evaluator + observe/diff handler | `performance_alerts` section includes `budget_exceeded` alerts | must |
| IT-2 | `check_budgets` MCP tool call | MCP dispatcher + budget evaluator | Returns violations and summary | must |
| IT-3 | Config file loaded on server startup | main.go + budget config loader | Server reads `.kaboom.json` on startup | must |
| IT-4 | Config file watch detects changes | File watcher (30s interval) + budget evaluator | Modified config re-read and budgets re-evaluated | must |
| IT-5 | No config file graceful handling | Server startup without `.kaboom.json` | Server starts normally, check_budgets returns "No budget configuration found" | must |
| IT-6 | Performance snapshot triggers budget check | Snapshot ingest + budget evaluator | Snapshot arrival triggers evaluation against applicable budgets | must |
| IT-7 | Route-specific budget applied correctly | Budget config with routes + performance snapshot | Correct route budget matched and evaluated | must |
| IT-8 | Multiple presets combined | Config with `presets: ["web-vitals-good", "performance-budget-default"]` | Both presets' thresholds active | should |
| IT-9 | Budget violation persistence across snapshots | First snapshot violates, second still violates | Violation persists, not duplicated | should |
| IT-10 | Invalid config mid-session | Replace valid config with invalid JSON | Warning logged, budgets disabled (previous config NOT retained) | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Config file read and parse | Latency | < 5ms | must |
| PT-2 | Budget evaluation per snapshot | Latency | < 0.1ms | must |
| PT-3 | Config file watch (stat check) | Overhead per 30s cycle | < 1ms | must |
| PT-4 | Memory for config with 20 routes | Memory | < 10KB | should |
| PT-5 | Budget evaluation with 10 metrics per snapshot | Latency | < 1ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Empty budgets object in config | `{"budgets": {}}` | No budgets enforced, no error | must |
| EC-2 | Route with trailing slash | Config: `/dashboard/`, URL: `/dashboard` | Match behavior documented (prefix match includes both forms) | should |
| EC-3 | Root route `/` | Config: `/` with budget | Matches all URLs (everything starts with /) | should |
| EC-4 | Very large config file | Config with 100+ routes | Loaded within performance budget, no crash | should |
| EC-5 | Negative budget value | `load_ms: -100` | Treated as invalid or no budget (not a valid threshold) | must |
| EC-6 | Extremely large budget value | `load_ms: 999999999` | Accepted, effectively no constraint | should |
| EC-7 | Config file deleted mid-session | Remove `.kaboom.json` while server is running | Next 30s check detects missing file, budgets disabled | must |
| EC-8 | Config file with extra fields | Config has unknown top-level keys besides `budgets` | Unknown keys silently ignored | should |
| EC-9 | Concurrent budget check and config reload | Config changes while check_budgets is running | Mutex ensures consistent read | must |
| EC-10 | Snapshot with no matching budget metrics | Snapshot has load_ms but no load_ms budget defined | No violation for that metric (no budget = no check) | must |
| EC-11 | Float precision in CLS comparison | CLS=0.10000001, budget=0.1 | Violation detected (actual > budget) | should |
| EC-12 | Config file is symlink | `.kaboom.json` is a symlink to another file | Follow symlink for read, but only within project directory | should |
| EC-13 | Budget violation on first page load ever | No baselines, first snapshot exceeds budget | Alert generated immediately (budgets are absolute) | must |
| EC-14 | Multiple config file locations | Both `.kaboom.json` and `.kaboom/budgets.json` exist | One takes precedence (document which) | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application running locally with measurable performance metrics
- [ ] Write access to project root for creating `.kaboom.json`

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "configure", "arguments": {"action": "health"}}` or AI calls `check_budgets` | None | Response indicates "No budget configuration found" (no `.kaboom.json` exists yet) | [ ] |
| UAT-2 | Human creates `.kaboom.json` at project root with: `{"budgets": {"default": {"load_ms": 2000, "total_transfer_kb": 500}, "presets": ["web-vitals-good"]}}` | File exists in project root | Configuration file saved | [ ] |
| UAT-3 | Wait 30 seconds for config watch to detect the file, then: `{"tool": "observe", "arguments": {"action": "check_budgets"}}` | None | Response shows `config_file: ".kaboom.json"` and budget definitions loaded | [ ] |
| UAT-4 | Navigate to the application in the browser to generate a performance snapshot | Page loads in browser | Performance snapshot captured by extension | [ ] |
| UAT-5 | `{"tool": "observe", "arguments": {"action": "check_budgets"}}` | Review violations | If page load > 2000ms or transfer > 500KB, violations appear with metric, budget, actual, over_by | [ ] |
| UAT-6 | `{"tool": "observe", "arguments": {"action": "check_budgets", "show_passing": true}}` | Review all metrics | Response includes both violations (if any) and passing metrics | [ ] |
| UAT-7 | `{"tool": "observe", "arguments": {"action": "check_budgets", "url": "/specific-page"}}` | None | Only the specified URL is checked against its applicable budget | [ ] |
| UAT-8 | `{"tool": "observe", "arguments": {"action": "get_changes_since", "since": "<recent_timestamp>"}}` | Check for performance_alerts section | If any budget was exceeded, `performance_alerts` includes `budget_exceeded` entries alongside any regression alerts | [ ] |
| UAT-9 | Human updates `.kaboom.json` to add a route-specific budget: `"routes": {"/login": {"load_ms": 1000}}` | File saved | Route-specific budget added | [ ] |
| UAT-10 | Wait 30 seconds, navigate to /login route, then: `{"tool": "observe", "arguments": {"action": "check_budgets", "url": "/login"}}` | None | Budget for /login uses route-specific 1000ms load threshold, not default 2000ms | [ ] |
| UAT-11 | Human deletes `.kaboom.json` | File removed | Budget configuration removed | [ ] |
| UAT-12 | Wait 30 seconds, then: `{"tool": "observe", "arguments": {"action": "check_budgets"}}` | None | Response: "No budget configuration found" | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Config path is relative | Inspect `config_file` field in check_budgets response | Shows `.kaboom.json` not absolute path | [ ] |
| DL-UAT-2 | URLs are path-only | Inspect violation alerts | URLs show path only (e.g., `/dashboard`), not full URLs with query strings | [ ] |
| DL-UAT-3 | Invalid config error is safe | Place invalid JSON in `.kaboom.json`, wait 30s, call check_budgets | Error message does not include raw file content | [ ] |
| DL-UAT-4 | Recommendations are generic | Check `recommendation` field in violation alerts | Recommendations mention `.kaboom.json` and general strategies, not application-specific details | [ ] |
| DL-UAT-5 | Raw config not exposed | Call check_budgets | Response shows parsed budget values, not the raw JSON content of the config file | [ ] |

### Regression Checks
- [ ] Existing regression detection (`get_changes_since` performance alerts) still works alongside budget checking
- [ ] Performance snapshot capture is unaffected by budget feature
- [ ] Server startup is not delayed by config file loading (< 5ms)
- [ ] Without `.kaboom.json`, all existing functionality works identically
- [ ] Config file watch does not interfere with other Kaboom file operations (`.kaboom/` directory)

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
