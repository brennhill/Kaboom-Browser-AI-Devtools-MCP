---
feature: dynamic-exposure
doc_type: qa-plan
feature_id: feature-dynamic-exposure
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Dynamic Exposure

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Feature flag registry
- [ ] Test IsEnabled returns true for enabled flag
- [ ] Test IsEnabled returns false for disabled flag
- [ ] Test IsEnabled defaults to true for undefined flag
- [ ] Test SetFlag updates registry
- [ ] Test LoadFromYAML parses config correctly
- [ ] Test concurrent access (RWMutex safety)

**Integration tests:** Full feature flag workflow
- [ ] Test start with config, flags loaded correctly
- [ ] Test disabled feature returns error
- [ ] Test enabled feature works normally
- [ ] Test hot-reload: update config, new flags take effect

**Edge case tests:** Error handling
- [ ] Test config file not found (fail startup)
- [ ] Test invalid YAML (log error, keep existing flags)
- [ ] Test config file deleted mid-run (log error, keep flags)

### Security/Compliance Testing

#### Audit tests:
- [ ] Test all flag changes logged
- [ ] Test CLI override logged

---

## Human UAT Walkthrough

### Scenario 1: Disabled Feature (Error Path)
1. Setup:
   - Create features.yaml: `{generate_har: false}`
   - Start: `kaboom --feature-flags=features.yaml`
2. Steps:
   - [ ] Check flags: `observe({what: "feature_flags"})` — verify generate_har: false
   - [ ] Attempt HAR generation: `generate({format: "har"})`
   - [ ] Verify error: {error: "feature_disabled", feature_flag: "generate_har"}
   - [ ] Attempt other generate types (reproduction): succeeds
3. Expected Result: Disabled feature fails, others work
4. Verification: Clear error message with flag name

### Scenario 2: Hot-Reload (Happy Path)
1. Setup:
   - Start with generate_har: false
2. Steps:
   - [ ] Attempt HAR generation: fails
   - [ ] Edit features.yaml: change generate_har: false → true
   - [ ] Wait 10 seconds (hot-reload delay)
   - [ ] Check server logs: "Feature flags reloaded, generate_har enabled"
   - [ ] Retry HAR generation: succeeds
3. Expected Result: Feature enabled without restart
4. Verification: Hot-reload works, no downtime

### Scenario 3: Emergency Disable via CLI
1. Setup:
   - Features.yaml has interact_execute_js: true
   - Start: `kaboom --feature-flags=features.yaml --disable-feature=interact_execute_js`
2. Steps:
   - [ ] Check flags: verify interact_execute_js: false (CLI override)
   - [ ] Attempt execute_js: fails
   - [ ] Edit features.yaml: change to true (attempt to re-enable)
   - [ ] Hot-reload occurs
   - [ ] Retry execute_js: still fails (CLI override persists)
3. Expected Result: CLI override cannot be bypassed by config
4. Verification: Emergency disable effective

### Scenario 4: Default Enabled (Backwards Compatibility)
1. Setup:
   - No feature flags config (or empty file)
   - Start: `kaboom`
2. Steps:
   - [ ] Check flags: observe({what: "feature_flags"}) returns empty or all true
   - [ ] Attempt all features: all work normally
3. Expected Result: All features enabled by default
4. Verification: Backwards compatible

### Scenario 5: Invalid Config (Error Handling)
1. Setup:
   - Create bad features.yaml: malformed YAML
   - Start: `kaboom --feature-flags=bad.yaml`
2. Steps:
   - [ ] Verify server fails to start with clear error
   - [ ] Or (if more graceful): server starts, logs error, uses defaults
3. Expected Result: Either fail-fast or graceful fallback
4. Verification: No undefined behavior

---

## Regression Testing

- Test default behavior (no flags) still works
- Test features not gated by flags work normally
- Test feature flags compatible with allowlisting (both can restrict)

---

## Performance/Load Testing

- Test flag check overhead (<0.01ms per request)
- Test hot-reload with 1000 concurrent requests (no race conditions)
- Test 100 flags in config (still fast lookup)
