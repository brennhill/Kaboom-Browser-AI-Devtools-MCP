---
feature: config-profiles
doc_type: qa-plan
feature_id: feature-config-profiles
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Configuration Profiles

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Profile loading and merging
- [ ] Test load paranoid profile, verify read_only=true
- [ ] Test load restricted profile, verify allowlist set
- [ ] Test CLI flag overrides profile setting
- [ ] Test invalid profile fails startup
- [ ] Test profile with unknown key warns, continues

**Integration tests:** Full profile workflow
- [ ] Test start with --profile=paranoid, verify all settings applied
- [ ] Test start with --profile=development, verify full access
- [ ] Test custom profile path: --profile=/custom/profile.yaml

**Edge case tests:** Error handling
- [ ] Test profile not found (fail with clear error)
- [ ] Test malformed YAML (fail with parse error)
- [ ] Test conflicting settings in profile (fail validation)

### Security/Compliance Testing

#### Profile security review:
- [ ] Review paranoid profile ensures max security
- [ ] Review restricted profile is production-safe
- [ ] Test no profile can bypass core security (e.g., localhost binding)

---

## Human UAT Walkthrough

### Scenario 1: Paranoid Profile (Max Security) (Happy Path)
1. Setup:
   - Start server: `kaboom --profile=paranoid`
2. Steps:
   - [ ] Verify server starts, logs "Profile 'paranoid' loaded"
   - [ ] Check config: `configure({action:"health"})` — verify read_only=true, aggressive redaction
   - [ ] Attempt mutation: `interact({action: "execute_js"})` — fails (read-only)
   - [ ] Observe logs: succeeds
   - [ ] Verify project expiration is short (15 minutes)
3. Expected Result: Max security settings applied
4. Verification: No mutations possible, aggressive data cleanup

### Scenario 2: Restricted Profile (Production-Safe)
1. Setup:
   - Start: `kaboom --profile=restricted`
2. Steps:
   - [ ] Check config: read_only=false, allowlist enabled
   - [ ] Observe: succeeds
   - [ ] Generate: succeeds
   - [ ] Navigate: succeeds (allowed in restricted allowlist)
   - [ ] Execute JS: fails (not in restricted allowlist)
3. Expected Result: Safe interactions allowed, dangerous ones blocked
4. Verification: Production-safe behavior

### Scenario 3: Development Profile (Full Access)
1. Setup:
   - Start: `kaboom --profile=development`
2. Steps:
   - [ ] Check config: no read-only, no allowlist
   - [ ] All tools work: observe, generate, interact (all actions)
   - [ ] Network body capture enabled
   - [ ] Long retention (1 week)
3. Expected Result: Full access, all features enabled
4. Verification: Development-friendly config

### Scenario 4: CLI Flag Override
1. Setup:
   - Start: `kaboom --profile=restricted --project-expiration-minutes=30`
2. Steps:
   - [ ] Check config: verify project expiration is 30 (not 60 from profile)
   - [ ] Verify other restricted settings still applied
3. Expected Result: CLI flag overrides profile setting
4. Verification: Fine-tuning works

### Scenario 5: Invalid Profile (Error Path)
1. Setup:
   - Create bad.yaml: `read_only: "not_a_boolean"`
2. Steps:
   - [ ] Start: `kaboom --profile=bad`
   - [ ] Verify server fails to start
   - [ ] Verify error message explains YAML validation failure
3. Expected Result: Server fails fast with clear error
4. Verification: No partial startup

---

## Regression Testing

- Test default behavior (no profile) still works
- Test all CLI flags still work without profile
- Test profiles compatible with all other features

---

## Performance/Load Testing

- Test profile loading time (<1ms)
- Verify no runtime overhead from profiles
