---
feature: accessibility-audit-runtime-error
doc_type: qa-plan
feature_id: bug-accessibility-audit-runtime-error
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Accessibility Audit Runtime Error (Bug Fix)

> How to test the accessibility audit bug fix. Includes code-level testing and human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Function definition and import
- [ ] `runAxeAuditWithTimeout` is defined in the correct module
- [ ] Function is exported correctly from a11y module
- [ ] inject.js imports the function successfully
- [ ] `window.axe` is defined before function is called
- [ ] Function accepts expected parameters (options, timeoutMs)
- [ ] Function returns a Promise that resolves with audit results
- [ ] Timeout mechanism works (reject after 10s)

**Integration tests:** End-to-end audit execution
- [ ] MCP tool `generate({action: "query_accessibility"})` returns real audit results
- [ ] Audit results include violations array
- [ ] Audit results include passes count
- [ ] Audit results include incomplete and inapplicable counts
- [ ] Audit results include url and pageTitle
- [ ] Audit results include testEngine with axe-core version
- [ ] Each violation includes: id, impact, description, help, helpUrl, nodes

**Edge case tests:** Error scenarios
- [ ] Audit when no tab tracked returns error: "No tab is currently tracked"
- [ ] Audit on tab without content script returns error: "Content script not loaded"
- [ ] Audit timeout after 10 seconds returns timeout error
- [ ] Audit when axe-core not loaded returns clear error
- [ ] Concurrent audits queue correctly and return individual results
- [ ] Audit on page with complex DOM (10,000+ elements) completes or times out gracefully

### Security/Compliance Testing

**Data leak tests:** Verify no sensitive data exposed
- [ ] HTML snippets in violation nodes sanitized (no executable code)
- [ ] Audit results do not include sensitive form input values
- [ ] No axe-core internal state or debug data leaked in results

#### Library verification tests:
- [ ] Axe-core is bundled locally in extension/lib/axe.min.js
- [ ] No external rule loading or CDN requests during audit
- [ ] Bundled axe-core version is from official Deque repository

---

## Human UAT Walkthrough

### Scenario 1: Basic Audit Works
1. Setup:
   - Start Kaboom server: `./dist/kaboom`
   - Load Chrome with extension
   - Navigate to <https://example.com>
   - Start tracking the tab via interact tool
2. Steps:
   - [ ] Call MCP tool: `generate({action: "query_accessibility"})`
   - [ ] Observe response
3. Expected Result: Response contains:
   ```json
   {
     "url": "https://example.com/",
     "pageTitle": "Example Domain",
     "timestamp": "2026-01-28T...",
     "testEngine": {
       "name": "axe-core",
       "version": "4.10.2"
     },
     "violations": [...],
     "passes": 12,
     "incomplete": 1,
     "inapplicable": 8
   }
   ```
4. Verification:
   - [ ] No runtime error about "runAxeAuditWithTimeout is not defined"
   - [ ] violations, passes, incomplete, inapplicable fields are present
   - [ ] url and pageTitle are populated
   - [ ] testEngine shows axe-core

### Scenario 2: Audit Finds Real Violations
1. Setup:
   - Create test page with accessibility issues:
     - Missing alt text on image
     - Low color contrast text
     - Missing form labels
   - Track the tab
2. Steps:
   - [ ] Call MCP tool: `generate({action: "query_accessibility"})`
   - [ ] Observe response
3. Expected Result:
   - [ ] violations array has 3+ entries
   - [ ] One violation has id: "image-alt"
   - [ ] One violation has id: "color-contrast"
   - [ ] One violation has id: "label"
   - [ ] Each violation includes help text and helpUrl
4. Verification: Audit identifies real accessibility issues

### Scenario 3: No Runtime Error on Complex Page
1. Setup: Navigate to complex page (e.g., GitHub.com with 5,000+ DOM elements)
2. Steps:
   - [ ] Call MCP tool: `generate({action: "query_accessibility"})`
   - [ ] Wait for response (may take 5-10 seconds)
3. Expected Result:
   - [ ] Either complete audit results OR timeout error
   - [ ] NO "runAxeAuditWithTimeout is not defined" error
4. Verification: Function is properly defined even on complex pages

### Scenario 4: No Tracked Tab Error
1. Setup:
   - Start Kaboom server
   - DO NOT track any tab
2. Steps:
   - [ ] Call MCP tool: `generate({action: "query_accessibility"})`
   - [ ] Observe response
3. Expected Result: Error response with clear message
4. Verification: Error says "No tab is currently tracked" (not undefined function)

### Scenario 5: Audit Timeout Handling
1. Setup:
   - Navigate to extremely complex page or freeze page via DevTools
   - Track the tab
2. Steps:
   - [ ] Call MCP tool: `generate({action: "query_accessibility"})`
   - [ ] Wait 10+ seconds
3. Expected Result: Timeout error after 10 seconds
4. Verification: Audit doesn't hang indefinitely

### Scenario 6: Accessible Page Returns Passes
1. Setup: Navigate to well-designed accessible page (e.g., gov.uk)
2. Steps:
   - [ ] Call MCP tool: `generate({action: "query_accessibility"})`
   - [ ] Observe response
3. Expected Result:
   - [ ] violations array is empty or has low count
   - [ ] passes count is high (20+)
   - [ ] incomplete and inapplicable counts present
4. Verification: Audit correctly identifies accessible pages

---

## Regression Testing

### Must Not Break

- [ ] DOM queries still work (`analyze({what: "dom"})`)
- [ ] Page info queries still work (`observe({what: "page"})`)
- [ ] Other extension message types still handled correctly
- [ ] Tab tracking still works
- [ ] Content script injection on new tabs still works

### Regression Test Steps

1. Run existing extension test suite: `node --test tests/extension/*.test.js`
2. Verify all accessibility audit tests pass
3. Test accessibility audit, then DOM query in sequence (no interference)
4. Test with multiple page navigations (content script re-injection)
5. Verify axe-core loads correctly after extension restarts

---

## Performance/Load Testing

### Audit execution time:
- [ ] Simple page (< 100 elements): < 1 second
- [ ] Medium page (100-1000 elements): < 3 seconds
- [ ] Complex page (1000-5000 elements): < 7 seconds
- [ ] Very complex page (5000+ elements): < 10 seconds or timeout

### Message passing overhead:
- [ ] Background → Content → Inject: < 10ms total

### No memory leaks:
- [ ] Run 50 consecutive audits
- [ ] Check extension memory usage (should not grow unbounded)
- [ ] Verify no message listeners accumulate
- [ ] Verify axe-core doesn't accumulate state

### Axe-core library loading:
- [ ] Verify axe.min.js loads within 50ms of extension injection
- [ ] Verify `window.axe` is defined before first audit

---

## Axe-Core Verification

### Library bundling:
- [ ] axe.min.js exists in extension/lib/ directory
- [ ] File size is ~500KB (compressed)
- [ ] No external CDN requests during audit
- [ ] manifest.json includes axe.min.js in content_scripts or web_accessible_resources

### Version verification:
- [ ] Audit response includes testEngine.version matching bundled axe-core
- [ ] Version is stable (4.x series)
- [ ] No version mismatch warnings in console

### Rule set verification:
- [ ] Audit runs standard WCAG 2.1 Level A and AA rules
- [ ] No custom rule loading from external sources
- [ ] All rule IDs in violations are from axe-core standard rules
