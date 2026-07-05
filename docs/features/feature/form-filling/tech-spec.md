---
feature: form-filling
status: proposed
doc_type: tech-spec
feature_id: feature-form-filling
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Form Filling

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Form filling is implemented as a new action (`fill_form`) under the existing `interact` tool. Server receives field specifications via MCP, creates async pending query, extension polls, executes filling logic in page context via inject.js, posts results back to server.

Follows standard async command pattern: MCP returns immediately with correlation_id, extension polls `/pending-queries`, executes with a 60s timeout window, posts result to `/execute-result`, client polls via `observe({what: "command_result"})`.

## Key Components

- **Server (cmd/browser-agent/tools_core.go)**: Add `fill_form` case to interact tool handler, validate field specifications, create pending query with timeout metadata
- **Extension (inject.js)**: Implement field filling logic with input type detection, event triggering, validation handling
- **Query executor (background.js)**: Poll `/pending-queries`, dispatch fill_form commands to inject.js via window.postMessage
- **Result aggregator**: Collect per-field results (success/error), return structured response with validation feedback

## Data Flows

```
AI: interact({action: "fill_form", fields: [...]})
  → Server: validate fields, create pending query, return correlation_id
  → Extension polls /pending-queries
  → background.js sends postMessage to inject.js
  → inject.js:
      1. Query each selector
      2. Detect input type (text/checkbox/select/etc)
      3. Set value
      4. Trigger events (input, change, blur)
      5. Check for validation errors (via HTML5 validation API)
  → inject.js posts result back via content.js → background.js → POST /execute-result
  → AI polls: observe({what: "command_result", correlation_id: "..."})
  → Server returns aggregated results
```

## Implementation Strategy

### Field filling approach:
1. Iterate through fields array in order (order matters for conditional logic)
2. For each field:
   - Query selector (try querySelector first, fallback to XPath)
   - Detect element type (input, select, textarea, etc)
   - Detect input type attribute (text, email, number, checkbox, radio, date, etc)
   - Set value using appropriate method:
     - Text inputs: `element.value = value`, trigger input/change/blur
     - Checkboxes/radios: `element.checked = value`, trigger change
     - Selects: `element.value = value`, trigger change
     - File inputs: not supported (security restriction)
   - Trigger validation: call `element.checkValidity()` to detect HTML5 validation errors
   - Collect result: {selector, status: "filled"|"error", value, message}
3. Return aggregated results: {status: "success"|"partial"|"error", results: [...]}

### Event triggering:
- Modern frameworks (React, Vue, Angular) require proper event sequences
- Trigger: `input` event (React detects changes), `change` event (standard DOM), `blur` event (validation)
- Use `new Event()` with `bubbles: true` to propagate through shadow DOM

### Validation handling:
- After setting value, call `element.checkValidity()` (HTML5 validation API)
- If invalid, capture `element.validationMessage` and include in error result
- Respect `required`, `pattern`, `min`, `max`, `minlength`, `maxlength` attributes

## Edge Cases & Assumptions

- **Edge Case 1**: Selector matches multiple elements → **Handling**: Fill only first match, warn in result
- **Edge Case 2**: Field appears after conditional selection → **Handling**: Fill in document order, if selector not found return "not_found" error
- **Edge Case 3**: Field is readonly or disabled → **Handling**: Skip field, return "skipped" status with reason
- **Edge Case 4**: Shadow DOM fields → **Handling**: Support shadowRoot.querySelector() if selector contains `:host`
- **Edge Case 5**: Iframe fields → **Handling**: Detect iframe context from selector (e.g., `iframe[name='payment'] >>> #cardNumber`), inject into iframe
- **Assumption 1**: Forms use standard HTML elements (not canvas-based custom inputs)
- **Assumption 2**: Fields are visible and interactable (not hidden via CSS)

## Risks & Mitigations

- **Risk 1**: Framework validation doesn't trigger → **Mitigation**: Trigger all standard events (input, change, blur) that frameworks listen to
- **Risk 2**: Timeout on complex forms → **Mitigation**: Use 60s timeout, fail fast on first error, return partial results
- **Risk 3**: Value doesn't persist (React overwrites) → **Mitigation**: Trigger input event with `bubbles: true` to notify React of change
- **Risk 4**: File upload security block → **Mitigation**: Document limitation, return clear error message
- **Risk 5**: CAPTCHA blocks form submission → **Mitigation**: Out of scope, agent must handle separately

## Dependencies

- Existing `interact` tool and async command architecture
- `/pending-queries` polling mechanism
- inject.js execution context (page-level JS access)

## Performance Considerations

- Field filling is synchronous in inject.js (loops through fields)
- Large forms (100+ fields) should complete well within the 60s timeout
- If form triggers complex JS on field change, monitor for timeout conditions
- Return partial results if timeout occurs mid-filling

## Security Considerations

- Same security boundary as existing `interact` tool: localhost-only, human opt-in (AI Web Pilot toggle)
- File upload inputs cannot be set programmatically (browser security restriction)
- Cross-origin iframes cannot be accessed (same-origin policy)
- No credential storage: values are transient, passed per-request
- Agent must handle sensitive data (passwords, credit cards) carefully (user responsibility)
