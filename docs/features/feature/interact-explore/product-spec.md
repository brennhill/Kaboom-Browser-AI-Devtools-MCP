---
status: shipped
scope: feature/interact-explore
ai-priority: high
tags: [core, interact, browser-control]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-interact-explore
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interact Product Spec (TARGET)

## Purpose
Provide deterministic browser control and DOM interaction for AI workflows.

## Actions (`action`)
`highlight`, `subtitle`, `save_state`, `load_state`, `list_states`, `delete_state`, `execute_js`, `navigate`, `refresh`, `back`, `forward`, `new_tab`, `screenshot`, `click`, `type`, `select`, `check`, `get_text`, `get_value`, `get_attribute`, `set_attribute`, `focus`, `scroll_to`, `wait_for`, `key_press`, `paste`, `list_interactive`, `record_start`, `record_stop`, `upload`, `draw_mode_start`

## Behavior Guarantees
1. Sync by default with correlation-aware command completion.
2. Browser automation requires AI Web Pilot enabled.
3. `subtitle` is composable with other actions via `subtitle` param.
4. `navigate` and `refresh` include performance baseline/diff context.
5. Screenshots are captured exclusively through `observe({what:"screenshot"})`;
   `interact` does not duplicate the observe mode.

## Requirements
- `INTERACT_PROD_001`: `action` is required and enum-validated.
- `INTERACT_PROD_002`: DOM primitive actions enforce required fields (`selector`, plus action-specific keys).
- `INTERACT_PROD_003`: async control flags (`sync`, `wait`, `background`) must behave deterministically.
- `INTERACT_PROD_004`: upload and draw mode actions must return correlation-aware queued states.
- `INTERACT_PROD_005`: Pilot-disabled state must return explicit actionable errors.
