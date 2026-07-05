---
feature: cpu-network-emulation
doc_type: qa-plan
feature_id: feature-cpu-network-emulation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: CPU/Network Emulation

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** CDP command generation
- [ ] Test Slow 3G profile generates correct CDP params
- [ ] Test Fast 3G profile
- [ ] Test custom network profile conversion
- [ ] Test CPU throttling rate setting
- [ ] Test reset command

**Integration tests:** Full emulation flow
- [ ] Test apply Slow 3G, verify waterfall shows throttled requests
- [ ] Test apply CPU 4x, verify JS execution slowed (benchmark test)
- [ ] Test offline mode, verify requests fail
- [ ] Test reset, verify normal performance restored

**Edge case tests:** Error handling
- [ ] Test debugger attachment failure
- [ ] Test invalid custom values (negative bandwidth)
- [ ] Test tab closed during emulation

### Security/Compliance Testing

#### Permission tests:
- [ ] Test debugger permission required for emulation

---

## Human UAT Walkthrough

### Scenario 1: Slow 3G Network Test (Happy Path)
1. Setup:
   - Open large page (e.g., news site with many images)
   - Start Kaboom
2. Steps:
   - [ ] Observe baseline: `observe({what: "vitals"})` note LCP time
   - [ ] Apply throttle: `configure({action: "emulation", network: "Slow 3G"})`
   - [ ] Reload page: `interact({action: "refresh"})`
   - [ ] Observe throttled: `observe({what: "vitals"})` note increased LCP
   - [ ] Verify network waterfall shows slow requests (200ms+ per resource)
3. Expected Result: Page load significantly slower, LCP increases 5-10x
4. Verification: Compare vitals before/after, waterfall shows throttled timing

### Scenario 2: CPU Throttling Test
1. Setup:
   - Open page with heavy JS (e.g., data visualization)
2. Steps:
   - [ ] Run JS benchmark: `interact({action: "execute_js", code: "let start = Date.now(); for (let i=0; i<1e8; i++) {}; Date.now() - start"})`
   - [ ] Note baseline time (e.g., 100ms)
   - [ ] Apply CPU throttle: `configure({action: "emulation", cpu: 4})`
   - [ ] Rerun same benchmark
   - [ ] Note throttled time (should be ~400ms, 4x slower)
3. Expected Result: JS execution 4x slower
4. Verification: Benchmark time ratio matches throttle rate

### Scenario 3: Offline Mode
1. Setup:
   - Open any page
2. Steps:
   - [ ] Apply offline: `configure({action: "emulation", network: "offline"})`
   - [ ] Navigate to new URL: `interact({action: "navigate", url: "https://example.com"})`
   - [ ] Observe error: should see "No internet" page
   - [ ] Reset: `configure({action: "emulation", reset: true})`
   - [ ] Navigate again, verify success
3. Expected Result: Offline blocks navigation, reset restores
4. Verification: Observe logs/errors for offline indication

### Scenario 4: Custom Network Profile
1. Setup:
   - Open page
2. Steps:
   - [ ] Apply custom: `configure({action: "emulation", network: "custom", download_kbps: 500, latency_ms: 300})`
   - [ ] Reload page
   - [ ] Observe waterfall, verify latency ~300ms per request
3. Expected Result: Custom throttle applied
4. Verification: Waterfall timing matches custom profile

### Scenario 5: Reset All Throttling
1. Setup:
   - Apply both network and CPU throttling
2. Steps:
   - [ ] Reset: `configure({action: "emulation", reset: true})`
   - [ ] Verify network and CPU back to normal (run benchmark, check waterfall)
3. Expected Result: All throttling removed
4. Verification: Performance metrics return to baseline

---

## Regression Testing

- Test existing configure actions still work
- Test observe tool correctly reports emulation status
- Test interact tool unaffected by throttling (may be slower but functional)

---

## Performance/Load Testing

- Test CDP command execution time (<50ms)
- Test debugger attachment overhead (<50ms)
- Verify no performance impact when emulation disabled
