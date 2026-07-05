---
feature: form-filling
status: proposed
tool: interact
mode: execute_js
version: 0.7.12
doc_type: product-spec
feature_id: feature-form-filling
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Form Filling

## Problem Statement

AI agents debugging web apps often need to test complex form workflows (multi-step forms, conditional fields, validation, file uploads). Manually filling forms during debugging is tedious and error-prone. Agents need programmatic form-filling to reproduce user workflows, test validation logic, and capture form submission telemetry.

## Solution

Add `fill_form` action to the `interact` tool. Agent provides field selectors and values; extension detects input types (text, email, checkbox, radio, select, file), handles validation, triggers appropriate events (input, change, blur), and fills all fields in a single operation.

## Requirements

- Support all standard HTML5 input types (text, email, number, date, checkbox, radio, select, textarea, file)
- Trigger proper DOM events to satisfy validation libraries (React, Vue, Angular)
- Handle conditional fields (fields that appear based on other selections)
- Return validation errors if browser detects invalid inputs
- Support both CSS selectors and XPath for field targeting
- Work with shadow DOM and iframes
- Respect readonly/disabled attributes (skip those fields)
- Handle multi-select dropdowns and checkbox groups

## Out of Scope

- CAPTCHA solving (requires human intervention)
- Complex drag-and-drop form builders (covered by separate drag-drop feature)
- File upload simulation (file input detection supported, but actual file upload requires user interaction due to security restrictions)
- Cross-origin iframe form filling (security boundary)

## Success Criteria

- Agent can fill a 10-field form in a single `interact` call
- Form validation is triggered correctly (e.g., "email invalid" errors appear)
- Complex forms (Stripe checkout, multi-step wizards) can be automated
- Zero false positives (fields that appear filled but values don't persist)

## User Workflow

1. Agent observes DOM to identify form structure (via `observe({what: "api", mode: "dom"})`)
2. Agent calls `interact({action: "fill_form", fields: [{selector: "#email", value: "test@example.com"}, ...]})`
3. Extension fills all fields, triggers validation events, returns success/error per field
4. Agent submits form (via `interact({action: "execute_js", code: "document.querySelector('form').submit()"})`) or clicks submit button
5. Agent observes network traffic to verify form submission

## Examples

### Simple login form:
```json
{
  "action": "fill_form",
  "fields": [
    {"selector": "#username", "value": "testuser"},
    {"selector": "#password", "value": "securepass123"}
  ]
}
```

### Complex multi-field form:
```json
{
  "action": "fill_form",
  "fields": [
    {"selector": "#firstName", "value": "John"},
    {"selector": "#lastName", "value": "Doe"},
    {"selector": "#email", "value": "john@example.com"},
    {"selector": "#country", "value": "US"},
    {"selector": "#agreeToTerms", "value": true},
    {"selector": "input[name='newsletter']", "value": true}
  ]
}
```

### Response with validation errors:
```json
{
  "status": "partial",
  "results": [
    {"selector": "#email", "status": "filled", "value": "john@example.com"},
    {"selector": "#phone", "status": "error", "message": "Invalid phone number format"}
  ]
}
```

---

## Notes

- Integrates with existing `interact` tool async command architecture
- Uses same 60s timeout window as other async interact actions
- Field filling order matters for forms with conditional logic (fill in document order)
