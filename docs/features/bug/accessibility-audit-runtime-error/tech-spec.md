---
feature: accessibility-audit-runtime-error
status: in-progress
doc_type: tech-spec
feature_id: bug-accessibility-audit-runtime-error
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Accessibility Audit Runtime Error (Bug Fix)

> Plain language only. No code. Describes HOW to fix the runtime error for accessibility audits.

## Architecture Overview

Kaboom's accessibility audit uses the same request/response pattern as DOM queries:
1. MCP tool creates a pending query in the server
2. Extension polls for pending queries
3. Background service worker receives accessibility query
4. Background worker forwards query to content script on tracked tab
5. Content script forwards to inject script
6. Inject script calls `runAxeAuditWithTimeout()` which runs axe-core
7. Results flow back: inject → content → background → server
8. MCP tool retrieves completed audit results

**The Bug:** Step 6 fails with "runAxeAuditWithTimeout is not defined". The function exists in the codebase but is not accessible at runtime when called.

## Key Components

- **axe.min.js:** Third-party accessibility testing library (bundled locally in extension/lib/)
- **a11y-queries.js (or similar):** Module containing `runAxeAuditWithTimeout` function
- **inject.js:** Must have access to `runAxeAuditWithTimeout` function when audit is requested
- **content.js:** Forwards accessibility audit messages from background to inject
- **background.js:** Handles accessibility query polling and forwarding (this part likely works)

## Data Flows

### Current (Broken) Flow
```
MCP Tool → Server creates pending query → Extension polls → background.js receives query
→ background.js forwards to content.js → content.js forwards to inject.js
→ inject.js attempts to call runAxeAuditWithTimeout()
→ ReferenceError: runAxeAuditWithTimeout is not defined
```

### Fixed Flow
```
MCP Tool → Server creates pending query → Extension polls → background.js receives query
→ background.js forwards to content.js → content.js forwards to inject.js
→ inject.js calls runAxeAuditWithTimeout() (function is defined and accessible)
→ runAxeAuditWithTimeout() runs axe.run() from axe.min.js
→ Results: inject.js → content.js → background.js → server → MCP Tool
```

## Implementation Strategy

### Step 1: Locate the Function Definition
Find where `runAxeAuditWithTimeout` is defined:
- Search for the function definition in `extension/lib/` directory
- Check if it's in a separate module (a11y-queries.js or similar)
- Verify the function signature and dependencies

### Step 2: Verify Import Chain
Trace how the function should be imported:
- Check if a11y-queries.js (or equivalent) is imported by inject.js
- Verify the import statement syntax matches the module export
- Ensure the import occurs BEFORE the function is called
- Check if the import is conditional or always loaded

### Step 3: Check Axe-Core Loading
Verify axe-core library loads before audit runs:
- Confirm axe.min.js is included in manifest.json content_scripts or inject.js imports
- Verify `window.axe` is defined before `runAxeAuditWithTimeout` attempts to use it
- Check if axe-core needs to be loaded asynchronously and awaited

### Step 4: Fix the Scope Issue
Resolve the undefined function error:
- If the function is not imported, add the import to inject.js
- If the function is in wrong scope, move it to the correct module
- If timing issue, ensure function is defined before message handler is registered
- If axe-core is not loaded, add proper loading logic with error handling

### Step 5: Add Runtime Checks
Prevent similar errors in the future:
- Before calling `runAxeAuditWithTimeout`, check if it's defined
- Before running axe audit, check if `window.axe` is defined
- If either is undefined, return structured error explaining what's missing
- Add console debug logging for troubleshooting

### Step 6: Verify Result Format
Ensure returned data matches the schema:
- Audit results must include: url, pageTitle, timestamp, testEngine, violations, passes, incomplete, inapplicable
- Violations must include: id, impact, description, help, helpUrl, nodes array
- Nodes must include: html, target (selector array), failureSummary

## Edge Cases & Assumptions

### Edge Case 1: Axe-Core Not Loaded
**Handling:** Check if `window.axe` is defined before calling `axe.run()`. If undefined, return error: `{success: false, error: 'Accessibility audit library (axe-core) not loaded'}`

### Edge Case 2: Audit Timeout
**Handling:** `runAxeAuditWithTimeout` must enforce a timeout (default 10 seconds). If axe.run() doesn't complete, reject the promise and return timeout error.

### Edge Case 3: No Tracked Tab
**Handling:** Background.js should check for tracked tab before forwarding audit query. If no tab is tracked, immediately post error result to server.

### Edge Case 4: Content Script Not Loaded
**Handling:** If `chrome.tabs.sendMessage()` fails because content script isn't injected, catch the error and post: `{success: false, error: 'Content script not loaded on tracked tab'}`

### Edge Case 5: Partial Audit Results
**Handling:** If axe.run() completes but returns partial results (e.g., some rules errored), still return the available results with a warning field indicating partial completion.

### Assumption 1: Axe-Core Is Bundled
We assume axe.min.js is already bundled in the extension at `extension/lib/axe.min.js` and declared in manifest.json. If not, it must be added.

### Assumption 2: Function Signature Matches
We assume the function signature is `runAxeAuditWithTimeout(options, timeoutMs)` or similar. Verify the expected parameters match what the caller provides.

### Assumption 3: Same Pattern as DOM Queries
We assume accessibility queries follow the exact same message forwarding pattern as DOM queries (which work correctly). Copy that pattern.

## Risks & Mitigations

### Risk 1: Axe-Core Version Incompatibility
**Mitigation:** Use a stable version of axe-core (e.g., 4.10.x). Document the required version in manifest.json and extension docs. Test with the bundled version.

### Risk 2: Axe-Core Performance on Large DOMs
**Mitigation:** The timeout mechanism (10s default) prevents indefinite hangs. Audits on pages with 10,000+ elements may timeout, which is acceptable behavior.

### Risk 3: Circular Import or Module Load Order
**Mitigation:** Use explicit imports in inject.js. Avoid circular dependencies between modules. Load a11y-queries module synchronously before registering message handlers.

### Risk 4: Breaking Existing Accessibility Queries
**Mitigation:** If accessibility queries currently work in some cases, ensure the fix doesn't break them. Test both before and after the fix on multiple pages.

## Dependencies

- **Existing:** axe-core library (axe.min.js) must be bundled locally
- **Existing:** Content script injection on tracked tabs (already working)
- **Existing:** Message forwarding from background to content (likely works)
- **New:** Import or define `runAxeAuditWithTimeout` in inject.js scope
- **New:** Ensure axe.min.js is loaded before audit function runs

## Performance Considerations

- Audit execution time: 1-10 seconds depending on page complexity
- Axe-core library size: ~500KB (already bundled, no runtime impact)
- Message passing overhead: < 10ms (same as DOM queries)
- Timeout enforcement: 10 seconds default (prevents indefinite hangs)
- No impact on main thread: audits run async and don't block browsing

## Security Considerations

- **Axe-Core Bundling:** Must be bundled locally (Chrome Web Store prohibits loading remotely hosted code). The bundled axe.min.js must be from a trusted source (official Deque repository).
- **Audit Scope:** Audits execute only on tracked tab, not all tabs (maintains single-tab isolation)
- **Result Sanitization:** HTML snippets in violation nodes are serialized to strings. No executable code or event handlers leak to server.
- **Privacy:** Audit results may contain page structure and text content (PII). This is already a known constraint of accessibility audits. Document in tool description.
- **No Remote Rule Loading:** Axe-core must use only bundled rules. No external rule fetching (could leak page data or introduce security risk).

## Test Plan Reference

See qa-plan.md for detailed testing strategy. Key test scenarios:
1. Audit returns real violations on page with accessibility issues
2. Audit returns passes when page is accessible
3. No runtime error for "runAxeAuditWithTimeout is not defined"
4. Audit timeout after 10 seconds on complex page
5. Clear error when axe-core is not loaded
6. Clear error when no tracked tab
7. Regression: DOM queries still work after fix
