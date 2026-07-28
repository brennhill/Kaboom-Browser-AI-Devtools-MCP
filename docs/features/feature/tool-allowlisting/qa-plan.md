---
feature: tool-allowlisting
doc_type: qa-plan
feature_id: feature-tool-allowlisting
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Tool Allowlisting

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Allowlist parsing and matching
- [ ] Test parse YAML allowlist file
- [ ] Test wildcard matching ("observe.*" matches "observe.logs")
- [ ] Test exact matching ("interact.navigate" matches only that)
- [ ] Test global wildcard ("*" matches everything)
- [ ] Test empty allowlist (default to all allowed)
- [ ] Test invalid YAML (server fails to start)

**Integration tests:** Full allowlist enforcement
- [ ] Test production profile (observe-only) blocks interact
- [ ] Test staging profile allows navigate, blocks execute_js
- [ ] Test development profile allows all
- [ ] Test error response includes allowed_tools list

**Edge case tests:** Error handling
- [ ] Test malformed config file (server fails with clear error)
- [ ] Test conflicting patterns (last wins or most specific wins)
- [ ] Test non-allowed tool retry (same error every time)

### Security/Compliance Testing

**Bypass tests:** Verify cannot circumvent
- [ ] Test cannot modify allowlist via MCP request
- [ ] Test extension HTTP endpoints respect allowlist
- [ ] Test concurrent requests all subject to allowlist

#### Audit tests:
- [ ] Test all blocked attempts logged
- [ ] Test allowlist contents logged at startup

---

## Human UAT Walkthrough

### Scenario 1: Production Profile (Observation-Only) (Happy Path)
1. Setup:
   - Create allowlist-production.yaml: `allowed_tools: [observe.*, generate.*, analyze.dom]`
   - Start server: `kaboom --allowlist-config=allowlist-production.yaml`
2. Steps:
   - [ ] Check status: `configure({what:"health"})` — verify allowlist_enabled: true
   - [ ] Observe logs: `observe({what: "logs"})` — succeeds
   - [ ] Generate reproduction: `generate({what: "reproduction"})` — succeeds
   - [ ] Query DOM: `analyze({what: "dom", selector: ".error"})` — succeeds
   - [ ] Attempt execute_js: `interact({what: "execute_js"})` — fails with error
   - [ ] Verify error includes allowed_tools list
3. Expected Result: Observation works, mutations blocked
4. Verification: Agent can analyze but not interact

### Scenario 2: Staging Profile (Safe Interactions)
1. Setup:
   - Create allowlist-staging.yaml: `allowed_tools: [observe.*, generate.*, interact.navigate, interact.refresh]`
2. Steps:
   - [ ] Observe works
   - [ ] Navigate: `interact({what: "navigate", url: "https://example.com"})` — succeeds
   - [ ] Refresh: `interact({what: "refresh"})` — succeeds
   - [ ] Execute JS: `interact({what: "execute_js"})` — fails
   - [ ] Fill form: `interact({what: "fill_form"})` — fails
3. Expected Result: Navigation allowed, code execution blocked
4. Verification: Safe interactions work, dangerous ones blocked

### Scenario 3: Development Profile (Full Access)
1. Setup:
   - Create allowlist-development.yaml: `allowed_tools: ["*"]`
2. Steps:
   - [ ] Observe: succeeds
   - [ ] Navigate: succeeds
   - [ ] Execute JS: succeeds
   - [ ] Fill form: succeeds
3. Expected Result: All tools/actions allowed
4. Verification: Full access like default behavior

### Scenario 4: Invalid Config (Error Path)
1. Setup:
   - Create malformed YAML: `allowed_tools: [observe.*, missing_quote`
2. Steps:
   - [ ] Attempt server start: `kaboom --allowlist-config=bad.yaml`
   - [ ] Verify server fails to start
   - [ ] Verify error message explains YAML parse failure
3. Expected Result: Server fails fast with clear error
4. Verification: No partial startup, clear diagnostic

### Scenario 5: Hot-Reload (Optional)
1. Setup:
   - Start server with allowlist
   - Modify allowlist file (add new allowed action)
2. Steps:
   - [ ] Server detects file change
   - [ ] Server logs "Allowlist reloaded"
   - [ ] Attempt newly allowed action
   - [ ] Verify now succeeds
3. Expected Result: Allowlist updated without restart
4. Verification: New rules take effect immediately

---

## Regression Testing

- Test default behavior (no allowlist) still allows all
- Test allowlist doesn't affect extension HTTP endpoints
- Test allowlist compatible with read-only mode (both can be enabled)

---

## Performance/Load Testing

- Test allowlist matching overhead (<0.1ms per request)
- Test 1000 requests with complex allowlist (20 patterns)
- Verify no performance degradation vs no allowlist
