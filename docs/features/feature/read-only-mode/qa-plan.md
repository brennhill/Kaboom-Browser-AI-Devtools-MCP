---
feature: read-only-mode
doc_type: qa-plan
feature_id: feature-read-only-mode
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Read-Only Mode

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Read-only flag enforcement
- [ ] Test server starts with --read-only flag, config.ReadOnlyMode = true
- [ ] Test KABOOM_READ_ONLY=true env var sets read-only mode
- [ ] Test interact tool returns error when read-only enabled
- [ ] Test observe tool works normally when read-only enabled
- [ ] Test generate tool works normally when read-only enabled
- [ ] Test analyze({what: "dom"}) works when read-only enabled
- [ ] Test configure clear returns error when read-only enabled

**Integration tests:** Full read-only workflow
- [ ] Test start server in read-only, connect agent, verify mutation blocked
- [ ] Test start server normally, verify mutations allowed
- [ ] Test configure health returns correct read_only_mode status

**Edge case tests:** Error handling
- [ ] Test CLI flag overrides env var (--read-only=false with KABOOM_READ_ONLY=true)
- [ ] Test all interact actions blocked in read-only mode
- [ ] Test all mutation configure actions blocked
- [ ] Test error message clarity (includes actionable guidance)

### Security/Compliance Testing

**Bypass tests:** Verify cannot circumvent
- [ ] Test cannot toggle read-only via MCP request
- [ ] Test extension HTTP endpoints respect read-only (no pending queries created)
- [ ] Test concurrent requests all blocked in read-only

#### Audit tests:
- [ ] Test all mutation attempts logged with error
- [ ] Test read-only status logged at startup

---

## Human UAT Walkthrough

### Scenario 1: Production Observation (Happy Path)
1. Setup:
   - Start Kaboom in read-only mode: `kaboom --read-only`
   - Open production web app in browser with extension
2. Steps:
   - [ ] Connect agent (Claude Code or Cursor)
- [ ] Check status: `configure({action: "health"})` — verify read_only_mode: true
   - [ ] Observe errors: `observe({what: "errors"})` — succeeds, returns errors
   - [ ] Observe network: `observe({what: "network_waterfall"})` — succeeds
   - [ ] Query DOM: `analyze({what: "dom", selector: ".error"})` — succeeds
- [ ] Generate reproduction: `generate({format: "reproduction"})` — succeeds
3. Expected Result: All observation and analysis operations work normally
4. Verification: Agent can analyze production issue without any mutations

### Scenario 2: Mutation Blocked (Error Path)
1. Setup:
   - Server running in read-only mode
2. Steps:
   - [ ] Attempt execute_js: `interact({action: "execute_js", code: "alert('test')"})`
   - [ ] Verify error returned: {error: "read_only_mode_enabled", message: "..."}
   - [ ] Attempt navigate: `interact({action: "navigate", url: "https://example.com"})`
   - [ ] Verify error returned
   - [ ] Attempt clear logs: `configure({action: "clear"})`
   - [ ] Verify error returned
3. Expected Result: All mutation attempts fail with clear error message
4. Verification: Browser state unchanged, no pending queries created

### Scenario 3: Normal Mode (Mutations Allowed)
1. Setup:
   - Start Kaboom without --read-only flag: `kaboom`
2. Steps:
   - [ ] Check status: `configure({action: "health"})` — verify read_only_mode: false
   - [ ] Attempt execute_js: `interact({action: "execute_js", code: "console.log('test')"})`
   - [ ] Verify succeeds
   - [ ] Attempt navigate: succeeds
3. Expected Result: All mutations allowed
4. Verification: Mutations execute normally

### Scenario 4: CLI Flag vs Environment Variable
1. Setup:
   - Set env var: `export KABOOM_READ_ONLY=true`
   - Start with flag: `kaboom --read-only=false`
2. Steps:
   - [ ] Check health response: verify read_only_mode based on CLI flag (false)
   - [ ] Attempt mutation: succeeds (CLI flag overrides env var)
3. Expected Result: CLI flag takes precedence
4. Verification: Mutations allowed when CLI flag says false

### Scenario 5: Runtime Toggle Attempt (Immutability)
1. Setup:
   - Server running in read-only mode
2. Steps:
   - [ ] Check if any MCP endpoint exists to toggle read-only
   - [ ] Verify no such endpoint (or it returns error)
   - [ ] Attempt configure action to disable read-only
   - [ ] Verify fails or no-op
3. Expected Result: Read-only mode cannot be toggled at runtime
4. Verification: Only way to change is restart server

---

## Regression Testing

- Test normal server startup (without read-only) still works
- Test all tools function normally when read-only disabled
- Test extension HTTP endpoints unaffected in normal mode

---

## Performance/Load Testing

- Test read-only check overhead (should be <0.01ms per request)
- Test 1000 mutation attempts in read-only mode (all should fail fast)
- Verify no memory leaks from repeated error returns
