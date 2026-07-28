---
feature: dom-fingerprinting
status: proposed
tool: configure
mode: dom_fingerprint
version: 0.7.12
doc_type: product-spec
feature_id: feature-dom-fingerprinting
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: DOM Fingerprinting

## Problem Statement

End-to-end tests break when developers refactor UI components, even when the application works correctly. The problem: tests use fragile selectors that couple tightly to DOM structure.

Examples of fragile selectors:
- `.MuiButton-root.MuiButton-contained.MuiButton-containedPrimary.css-1hw9j7s` (CSS-in-JS class names change on every build)
- `div > div > div > button:nth-child(3)` (breaks when DOM structure changes)
- `#root > main > div > form > button` (breaks when wrapper divs added/removed)

When a developer renames a CSS class from `.submit-btn` to `.btn-submit`, the application works but the test fails with "element not found." The test is wrong, not the application. But current E2E test repair (Agentic E2E Repair feature) can only update the selector to the new exact class name -- it cannot make the selector resilient to future changes.

**The gap:** Tests need selectors that survive UI refactors. Current `analyze({what: "dom"})` returns current selectors but doesn't score them for stability. The AI has no way to choose between `.submit-btn` (fragile, structural) and `button[aria-label="Submit"]` (stable, semantic).

## Solution

Add `configure({what: "dom_fingerprint", selector: "..."})` -- a new action that analyzes an element and generates a ranked list of stable selectors. Selectors are scored based on semantic anchoring (ARIA roles, labels, data-testid), resilience to structural changes, and uniqueness.

DOM fingerprinting enables self-healing tests: when a test's selector breaks, the AI can query the fingerprint of the target element (identified by text content, role, or partial match) and get a more stable selector to use in the updated test.

## Requirements

- R1: Accept a CSS selector as input and return ranked list of alternative selectors for the same element
- R2: Score selectors based on stability factors: data-testid (highest), ARIA role/label (high), tag+text (medium), CSS classes (low), positional (lowest)
- R3: Return uniqueness score for each selector (true = unique on page, false = matches multiple elements)
- R4: Include semantic attributes in ranked selectors (role, aria-label, name, placeholder, title, alt)
- R5: Support fuzzy matching: if exact selector not found, find elements by text content or role and return their fingerprints
- R6: Generate hybrid selectors combining semantic and structural attributes (e.g., `button[aria-label="Submit"]`)
- R7: Validate generated selectors actually match the target element
- R8: Cap selector list at 10 alternatives to avoid overwhelming AI
- R9: Execute in < 100ms for typical page
- R10: Return error if selector matches zero elements or if element is not interactive

## Out of Scope

- This feature does NOT modify test code. It provides stable selectors; the AI or Agentic E2E Repair uses them to update tests.
- This feature does NOT track selector changes over time. It's a point-in-time analysis.
- This feature does NOT inject data-testid attributes into the page. If the page lacks semantic attributes, generated selectors use available attributes (may still be fragile).
- Out of scope: Shadow DOM traversal. Only light DOM elements supported initially.
- Out of scope: XPath selectors. Only CSS selectors returned.

## Success Criteria

- AI can call dom_fingerprint after detecting selector drift in a failing test
- Response includes ranked selectors with stability scores
- Top-ranked selector uses semantic attributes when available
- Fuzzy matching finds target element even when exact selector fails
- Generated selectors pass validation (actually select the target element)
- Agentic E2E Repair uses fingerprinting to update tests with stable selectors
- Tests using fingerprint-generated selectors survive UI refactors

## User Workflow

1. E2E test fails with "element not found" for selector `.submit-btn`
2. Agentic E2E Repair detects selector drift
3. AI calls `analyze({what: "dom", selector: "button", text_content: "Submit"})` to find the button by text
4. Element found but at different selector (`.btn-submit`)
5. AI calls `configure({what: "dom_fingerprint", selector: ".btn-submit"})`
6. Server returns ranked selectors: `button[aria-label="Submit"]` (score: 9), `button[name="submit"]` (score: 7), `.btn-submit` (score: 3)
7. AI updates test to use `button[aria-label="Submit"]` (highest stability score)
8. Test now passes and is resilient to future CSS class renames

## Examples

### Example 1: Button with ARIA label

Request:
```json
{
  "tool": "configure",
  "arguments": {
    "action": "dom_fingerprint",
    "selector": ".submit-btn"
  }
}
```

Response:
```json
{
  "element": {
    "tag": "button",
    "text": "Submit",
    "visible": true
  },
  "ranked_selectors": [
    {
      "selector": "button[aria-label=\"Submit\"]",
      "score": 9,
      "rationale": "ARIA label (semantic, stable)",
      "unique": true
    },
    {
      "selector": "button[name=\"submit\"]",
      "score": 7,
      "rationale": "Name attribute (stable)",
      "unique": true
    },
    {
      "selector": "form button[type=\"submit\"]",
      "score": 6,
      "rationale": "Semantic type + parent context",
      "unique": true
    },
    {
      "selector": ".submit-btn",
      "score": 3,
      "rationale": "CSS class (fragile, may change)",
      "unique": true
    }
  ]
}
```

### Example 2: Input with data-testid

Request:
```json
{
  "tool": "configure",
  "arguments": {
    "action": "dom_fingerprint",
    "selector": "#email-input"
  }
}
```

Response:
```json
{
  "element": {
    "tag": "input",
    "text": "",
    "visible": true
  },
  "ranked_selectors": [
    {
      "selector": "[data-testid=\"email-input\"]",
      "score": 10,
      "rationale": "data-testid attribute (highest stability)",
      "unique": true
    },
    {
      "selector": "input[name=\"email\"]",
      "score": 7,
      "rationale": "Name attribute (stable)",
      "unique": true
    },
    {
      "selector": "input[type=\"email\"]",
      "score": 6,
      "rationale": "Semantic type (stable but may not be unique)",
      "unique": false
    },
    {
      "selector": "#email-input",
      "score": 5,
      "rationale": "ID (stable if ID doesn't change)",
      "unique": true
    }
  ]
}
```

### Example 3: Fuzzy match by text

Request:
```json
{
  "tool": "configure",
  "arguments": {
    "action": "dom_fingerprint",
    "text_content": "Add to Cart"
  }
}
```

Response (exact selector not provided, using fuzzy match):
```json
{
  "element": {
    "tag": "button",
    "text": "Add to Cart",
    "visible": true
  },
  "ranked_selectors": [
    {
      "selector": "button[aria-label=\"Add to Cart\"]",
      "score": 9,
      "rationale": "ARIA label matches text (semantic)",
      "unique": true
    },
    {
      "selector": "button:has-text(\"Add to Cart\")",
      "score": 7,
      "rationale": "Text content (Playwright-specific, stable)",
      "unique": true
    },
    {
      "selector": ".product button.primary",
      "score": 4,
      "rationale": "Context + classes (fragile)",
      "unique": false
    }
  ]
}
```

---

## Notes

- Integrates with Agentic E2E Repair (provides stable selectors for test fixes)
- Scoring algorithm prioritizes: data-testid (10) > ARIA role/label (9) > name attribute (7) > type attribute (6) > ID (5) > text content (7 for Playwright) > CSS classes (3) > nth-child (1)
- Uniqueness determined by testing selector against entire page DOM
