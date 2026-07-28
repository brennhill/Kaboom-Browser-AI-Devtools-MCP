---
feature: accessibility-audit-runtime-error
status: in-progress
tool: generate
mode: accessibility
version: 0.7.12
doc_type: product-spec
feature_id: bug-accessibility-audit-runtime-error
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Accessibility Audit Runtime Error (Bug Fix)

## Problem Statement

Users attempting to run accessibility audits via `generate({what: "query_accessibility"})` encounter a runtime error: `runAxeAuditWithTimeout is not defined`. The function is implemented in the codebase but is undefined at runtime when called, causing all accessibility audits to fail.

### Current User Experience:
1. User calls `generate({what: "query_accessibility"})`
2. Extension attempts to execute the accessibility audit
3. Runtime error: `ReferenceError: runAxeAuditWithTimeout is not defined`
4. Audit fails with no results
5. User cannot diagnose accessibility issues in their application

**Root Cause:** The `runAxeAuditWithTimeout` function exists in the codebase but is not properly exported, imported, or scoped for runtime access. This is likely an import/export issue, timing problem (function not yet defined when called), or scope issue (function not in the correct context).

## Solution

Fix the import/export chain or scope issue so that `runAxeAuditWithTimeout` is defined and accessible when the accessibility audit executor calls it. Ensure the axe-core library is properly loaded and the audit function can access it.

### Fixed User Experience:
1. User calls `generate({what: "query_accessibility"})`
2. Query is forwarded to the extension
3. `runAxeAuditWithTimeout` executes successfully with axe-core
4. User receives real accessibility audit results with violations, passes, and incomplete checks

## Requirements

1. **Fix Function Binding:** Ensure `runAxeAuditWithTimeout` is defined in the correct scope when called
2. **Verify Import Chain:** Trace imports from background.js → content.js → inject.js → accessibility module
3. **Axe-Core Loading:** Verify axe-core library (axe.min.js) is loaded before audit function runs
4. **Error Handling:** If axe-core fails to load, return structured error (not undefined function error)
5. **Timeout Enforcement:** Audit must timeout after configured limit (10s default) to prevent hanging
6. **Schema Compliance:** Results must match the existing accessibility audit response schema
7. **Backward Compatibility:** Fix must not break existing audit result structure

## Out of Scope

- Adding new accessibility audit capabilities
- Changing the accessibility audit schema
- Performance optimizations beyond fixing the runtime error
- Supporting accessibility frameworks other than axe-core
- Custom accessibility rule configuration (already supported via axe-core config)

## Success Criteria

1. `generate({what: "query_accessibility"})` returns real audit results without runtime errors
2. Audit includes violations, passes, incomplete, and inapplicable checks
3. Each violation includes impact, description, help text, and affected elements
4. Audit completes within timeout period (10s default)
5. Error responses clearly indicate when audit can't run (e.g., axe-core not loaded)
6. All existing accessibility audit tests pass

## User Workflow

### Before Fix:
1. User calls `generate({what: "query_accessibility"})`
2. Receives runtime error: "runAxeAuditWithTimeout is not defined"
3. No audit results available
4. User cannot diagnose accessibility issues

### After Fix:
1. User calls `generate({what: "query_accessibility"})`
2. Audit executes in tracked tab's DOM
3. User receives comprehensive accessibility report
4. User can identify violations, review best practices, fix issues

## Examples

### Example 1: Successful Audit

#### Request:
```json
{
  "tool": "generate",
  "arguments": {
    "action": "query_accessibility"
  }
}
```

#### Before Fix Response:
```json
{
  "error": "Runtime error: runAxeAuditWithTimeout is not defined at line 1234"
}
```

#### After Fix Response:
```json
{
  "url": "https://example.com",
  "pageTitle": "Example Domain",
  "timestamp": "2026-01-28T10:00:00Z",
  "testEngine": {
    "name": "axe-core",
    "version": "4.10.2"
  },
  "violations": [
    {
      "id": "color-contrast",
      "impact": "serious",
      "description": "Ensures the contrast between foreground and background colors meets WCAG 2 AA contrast ratio thresholds",
      "help": "Elements must have sufficient color contrast",
      "helpUrl": "https://dequeuniversity.com/rules/axe/4.10/color-contrast",
      "nodes": [
        {
          "html": "<a href=\"/about\">About</a>",
          "target": ["a[href='/about']"],
          "failureSummary": "Fix any of the following: Element has insufficient color contrast of 3.2:1 (required 4.5:1)"
        }
      ]
    }
  ],
  "passes": 12,
  "incomplete": 1,
  "inapplicable": 8
}
```

### Example 2: No Tracked Tab Error

#### Request:
```json
{
  "tool": "generate",
  "arguments": {
    "action": "query_accessibility"
  }
}
```

#### Response (No Tab Tracked):
```json
{
  "error": "No tab is currently tracked. Use interact({what: 'track_tab'}) first."
}
```

### Example 3: Axe-Core Not Loaded Error

**Request:** Same as Example 1

#### Response (If axe-core library fails to load):
```json
{
  "error": "Accessibility audit library (axe-core) not loaded. Ensure axe.min.js is included in the extension."
}
```

---

## Notes

- Axe-core library must be bundled locally in `extension/lib/axe.min.js` (Chrome Web Store prohibits loading remotely hosted code)
- The function `runAxeAuditWithTimeout` is likely defined in an accessibility module or a11y-queries.js
- The fix should follow the same pattern as DOM queries (which work correctly)
- Related specs: See `docs/features/feature/query-dom/product-spec.md` for DOM query pattern
- Audit timeouts prevent infinite hangs on pages with complex DOMs or slow JavaScript
