---
title: "Interact Executable Examples"
description: "Runnable examples for every interact action with response shapes and failure fixes."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['reference', 'examples', 'interact']
---

# Interact Executable Examples

Each section provides one runnable baseline call, expected response shape, and one failure example with a concrete fix. Use these as copy/paste starters and then adjust for your page or workflow.

## Quick Reference

```json
{
  "tool": "interact",
  "arguments": {
    "what": "highlight"
  }
}
```

## Common Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| `what` | string | Action name to execute. |
| `tab_id` | number | Optional target browser tab. |
| `telemetry_mode` | string | Optional telemetry verbosity: `off`, `auto`, `full`. |

## Actions

### `highlight`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "highlight"
  }
}
```

#### Expected response shape

```json
{
  "action": "highlight",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "highlight"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `highlight`.

### `subtitle`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "subtitle",
    "text": "Opening checkout flow"
  }
}
```

#### Expected response shape

```json
{
  "action": "subtitle",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "subtitle"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "text": "Opening checkout flow"
  }
}
```

Fix: Use a valid interact action value, e.g. `subtitle`.

### `save_state`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "save_state"
  }
}
```

#### Expected response shape

```json
{
  "action": "save_state",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "save_state"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `save_state`.

### `load_state`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "load_state"
  }
}
```

#### Expected response shape

```json
{
  "action": "load_state",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "load_state"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `load_state`.

### `list_states`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "list_states"
  }
}
```

#### Expected response shape

```json
{
  "action": "list_states",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "list_states"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `list_states`.

### `delete_state`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "delete_state"
  }
}
```

#### Expected response shape

```json
{
  "action": "delete_state",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "delete_state"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `delete_state`.

### `set_storage`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "set_storage",
    "storage_type": "local",
    "key": "theme",
    "value": "dark"
  }
}
```

#### Expected response shape

```json
{
  "action": "set_storage",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "set_storage"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "storage_type": "local",
    "key": "theme",
    "value": "dark"
  }
}
```

Fix: Use a valid interact action value, e.g. `set_storage`.

### `delete_storage`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "delete_storage",
    "storage_type": "local",
    "key": "theme"
  }
}
```

#### Expected response shape

```json
{
  "action": "delete_storage",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "delete_storage"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "storage_type": "local",
    "key": "theme"
  }
}
```

Fix: Use a valid interact action value, e.g. `delete_storage`.

### `clear_storage`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "clear_storage"
  }
}
```

#### Expected response shape

```json
{
  "action": "clear_storage",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "clear_storage"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `clear_storage`.

### `set_cookie`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "set_cookie",
    "name": "theme",
    "value": "dark",
    "domain": "example.com"
  }
}
```

#### Expected response shape

```json
{
  "action": "set_cookie",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "set_cookie"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "name": "theme",
    "value": "dark",
    "domain": "example.com"
  }
}
```

Fix: Use a valid interact action value, e.g. `set_cookie`.

### `delete_cookie`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "delete_cookie",
    "name": "theme",
    "domain": "example.com"
  }
}
```

#### Expected response shape

```json
{
  "action": "delete_cookie",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "delete_cookie"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "name": "theme",
    "domain": "example.com"
  }
}
```

Fix: Use a valid interact action value, e.g. `delete_cookie`.

### `execute_js`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "execute_js",
    "script": "document.title"
  }
}
```

#### Expected response shape

```json
{
  "action": "execute_js",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "execute_js"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "script": "document.title"
  }
}
```

Fix: Use a valid interact action value, e.g. `execute_js`.

### `navigate`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "navigate",
    "url": "https://example.com"
  }
}
```

#### Expected response shape

```json
{
  "action": "navigate",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "navigate"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "navigate",
    "url": 404
  }
}
```

Fix: Use a fully qualified URL string, e.g. `https://example.com`.

### `refresh`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "refresh"
  }
}
```

#### Expected response shape

```json
{
  "action": "refresh",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "refresh"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `refresh`.

### `back`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "back"
  }
}
```

#### Expected response shape

```json
{
  "action": "back",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "back"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `back`.

### `forward`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "forward"
  }
}
```

#### Expected response shape

```json
{
  "action": "forward",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "forward"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `forward`.

### `new_tab`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "new_tab",
    "url": "https://example.com"
  }
}
```

#### Expected response shape

```json
{
  "action": "new_tab",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "new_tab"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "new_tab",
    "url": 404
  }
}
```

Fix: Use a fully qualified URL string, e.g. `https://example.com`.

### `switch_tab`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "switch_tab",
    "tab_id": 123
  }
}
```

#### Expected response shape

```json
{
  "action": "switch_tab",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "switch_tab"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "switch_tab",
    "tab_id": "123"
  }
}
```

Fix: Use `tab_id` as a number, e.g. `tab_id: 123`.

### `close_tab`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "close_tab",
    "tab_id": 123
  }
}
```

#### Expected response shape

```json
{
  "action": "close_tab",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "close_tab"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "close_tab",
    "tab_id": "123"
  }
}
```

Fix: Use `tab_id` as a number, e.g. `tab_id: 123`.

### `screenshot`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "screenshot"
  }
}
```

#### Expected response shape

```json
{
  "action": "screenshot",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "screenshot"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `screenshot`.

### `click`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "click",
    "selector": "text=Submit"
  }
}
```

#### Expected response shape

```json
{
  "action": "click",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "click"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "click",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `type`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "type",
    "selector": "label=Email",
    "text": "user@example.com"
  }
}
```

#### Expected response shape

```json
{
  "action": "type",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "type"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "type",
    "selector": 42,
    "text": "user@example.com"
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `select`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "select",
    "selector": "#country",
    "value": "US"
  }
}
```

#### Expected response shape

```json
{
  "action": "select",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "select"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "select",
    "selector": 42,
    "value": "US"
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `check`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "check",
    "selector": "#terms",
    "checked": true
  }
}
```

#### Expected response shape

```json
{
  "action": "check",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "check"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "check",
    "selector": 42,
    "checked": true
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `get_text`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "get_text",
    "selector": "h1"
  }
}
```

#### Expected response shape

```json
{
  "action": "get_text",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "get_text"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "get_text",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `get_value`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "get_value",
    "selector": "input[name=\"email\"]"
  }
}
```

#### Expected response shape

```json
{
  "action": "get_value",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "get_value"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "get_value",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `get_attribute`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "get_attribute",
    "selector": "a.primary",
    "name": "href"
  }
}
```

#### Expected response shape

```json
{
  "action": "get_attribute",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "get_attribute"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "get_attribute",
    "selector": 42,
    "name": "href"
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `query`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "query",
    "selector": "button",
    "query_type": "count"
  }
}
```

#### Expected response shape

```json
{
  "action": "query",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "query"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "query",
    "selector": 42,
    "query_type": "count"
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `set_attribute`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "set_attribute",
    "selector": "input[name=\"email\"]",
    "name": "value",
    "value": "user@example.com"
  }
}
```

#### Expected response shape

```json
{
  "action": "set_attribute",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "set_attribute"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "set_attribute",
    "selector": 42,
    "name": "value",
    "value": "user@example.com"
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `focus`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "focus",
    "selector": "#search"
  }
}
```

#### Expected response shape

```json
{
  "action": "focus",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "focus"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "focus",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `scroll_to`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "scroll_to",
    "direction": "bottom"
  }
}
```

#### Expected response shape

```json
{
  "action": "scroll_to",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "scroll_to"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "direction": "bottom"
  }
}
```

Fix: Use a valid interact action value, e.g. `scroll_to`.

### `wait_for`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "wait_for",
    "selector": "main"
  }
}
```

#### Expected response shape

```json
{
  "action": "wait_for",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "wait_for"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "wait_for",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `key_press`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "key_press",
    "text": "Enter"
  }
}
```

#### Expected response shape

```json
{
  "action": "key_press",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "key_press"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "text": "Enter"
  }
}
```

Fix: Use a valid interact action value, e.g. `key_press`.

### `paste`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "paste",
    "selector": "textarea",
    "text": "Pasted from automation"
  }
}
```

#### Expected response shape

```json
{
  "action": "paste",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "paste"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "paste",
    "selector": 42,
    "text": "Pasted from automation"
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `open_composer`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "open_composer"
  }
}
```

#### Expected response shape

```json
{
  "action": "open_composer",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "open_composer"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `open_composer`.

### `submit_active_composer`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "submit_active_composer"
  }
}
```

#### Expected response shape

```json
{
  "action": "submit_active_composer",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "submit_active_composer"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `submit_active_composer`.

### `confirm_top_dialog`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "confirm_top_dialog"
  }
}
```

#### Expected response shape

```json
{
  "action": "confirm_top_dialog",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "confirm_top_dialog"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `confirm_top_dialog`.

### `dismiss_top_overlay`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "dismiss_top_overlay"
  }
}
```

#### Expected response shape

```json
{
  "action": "dismiss_top_overlay",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "dismiss_top_overlay"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `dismiss_top_overlay`.

### `hover`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "hover",
    "selector": "text=Settings"
  }
}
```

#### Expected response shape

```json
{
  "action": "hover",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "hover"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "hover",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `auto_dismiss_overlays`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "auto_dismiss_overlays"
  }
}
```

#### Expected response shape

```json
{
  "action": "auto_dismiss_overlays",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "auto_dismiss_overlays"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `auto_dismiss_overlays`.

### `wait_for_stable`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "wait_for_stable"
  }
}
```

#### Expected response shape

```json
{
  "action": "wait_for_stable",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "wait_for_stable"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `wait_for_stable`.

### `list_interactive`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "list_interactive"
  }
}
```

#### Expected response shape

```json
{
  "action": "list_interactive",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "list_interactive"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `list_interactive`.

### `get_readable`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "get_readable"
  }
}
```

#### Expected response shape

```json
{
  "action": "get_readable",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "get_readable"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `get_readable`.

### `get_markdown`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "get_markdown"
  }
}
```

#### Expected response shape

```json
{
  "action": "get_markdown",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "get_markdown"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `get_markdown`.

### `navigate_and_wait_for`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "navigate_and_wait_for",
    "url": "https://example.com",
    "wait_for": "main"
  }
}
```

#### Expected response shape

```json
{
  "action": "navigate_and_wait_for",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "navigate_and_wait_for"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "navigate_and_wait_for",
    "url": 404,
    "wait_for": "main"
  }
}
```

Fix: Use a fully qualified URL string, e.g. `https://example.com`.

### `navigate_and_document`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "navigate_and_document",
    "url": "https://example.com",
    "include_screenshot": true
  }
}
```

#### Expected response shape

```json
{
  "action": "navigate_and_document",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "navigate_and_document"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "navigate_and_document",
    "url": 404,
    "include_screenshot": true
  }
}
```

Fix: Use a fully qualified URL string, e.g. `https://example.com`.

### `fill_form_and_submit`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "fill_form_and_submit",
    "fields": [
      {
        "selector": "input[name=\"email\"]",
        "value": "user@example.com"
      },
      {
        "selector": "input[name=\"password\"]",
        "value": "hunter2"
      }
    ],
    "submit_selector": "button[type=\"submit\"]"
  }
}
```

#### Expected response shape

```json
{
  "action": "fill_form_and_submit",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "fill_form_and_submit"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "fields": [
      {
        "selector": "input[name=\"email\"]",
        "value": "user@example.com"
      },
      {
        "selector": "input[name=\"password\"]",
        "value": "hunter2"
      }
    ],
    "submit_selector": "button[type=\"submit\"]"
  }
}
```

Fix: Use a valid interact action value, e.g. `fill_form_and_submit`.

### `fill_form`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "fill_form",
    "fields": [
      {
        "selector": "input[name=\"email\"]",
        "value": "user@example.com"
      }
    ]
  }
}
```

#### Expected response shape

```json
{
  "action": "fill_form",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "fill_form"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "fields": [
      {
        "selector": "input[name=\"email\"]",
        "value": "user@example.com"
      }
    ]
  }
}
```

Fix: Use a valid interact action value, e.g. `fill_form`.

### `run_a11y_and_export_sarif`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "run_a11y_and_export_sarif",
    "save_to": ".kaboom/reports/a11y.sarif"
  }
}
```

#### Expected response shape

```json
{
  "action": "run_a11y_and_export_sarif",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "run_a11y_and_export_sarif"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "save_to": ".kaboom/reports/a11y.sarif"
  }
}
```

Fix: Use a valid interact action value, e.g. `run_a11y_and_export_sarif`.

### `screen_recording_start`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "screen_recording_start"
  }
}
```

#### Expected response shape

```json
{
  "action": "screen_recording_start",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "screen_recording_start"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `screen_recording_start`.

### `screen_recording_stop`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "screen_recording_stop"
  }
}
```

#### Expected response shape

```json
{
  "action": "screen_recording_stop",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "screen_recording_stop"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `screen_recording_stop`.

### `upload`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "upload",
    "file_path": "/tmp/example.png",
    "selector": "input[type=\"file\"]"
  }
}
```

#### Expected response shape

```json
{
  "action": "upload",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "upload"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "upload",
    "file_path": "/tmp/example.png",
    "selector": 42
  }
}
```

Fix: Use `selector` as a CSS or semantic selector string.

### `draw_mode_start`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "draw_mode_start",
    "annot_session": "checkout-review"
  }
}
```

#### Expected response shape

```json
{
  "action": "draw_mode_start",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "draw_mode_start"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "annot_session": "checkout-review"
  }
}
```

Fix: Use a valid interact action value, e.g. `draw_mode_start`.

### `activate_tab`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "activate_tab",
    "tab_id": 123
  }
}
```

#### Expected response shape

```json
{
  "action": "activate_tab",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "activate_tab"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "activate_tab",
    "tab_id": "123"
  }
}
```

Fix: Use `tab_id` as a number, e.g. `tab_id: 123`.

### `explore_page`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "explore_page"
  }
}
```

#### Expected response shape

```json
{
  "action": "explore_page",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "explore_page"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `explore_page`.

### `batch`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "batch",
    "steps": [
      {
        "what": "navigate",
        "url": "https://example.com"
      },
      {
        "what": "click",
        "selector": "text=Sign in"
      }
    ]
  }
}
```

#### Expected response shape

```json
{
  "action": "batch",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "batch"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "batch",
    "steps": "navigate,click"
  }
}
```

Fix: Use `steps` as an array of action objects.

### `clipboard_read`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "clipboard_read"
  }
}
```

#### Expected response shape

```json
{
  "action": "clipboard_read",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "clipboard_read"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid interact action value, e.g. `clipboard_read`.

### `clipboard_write`

#### Minimal call

```json
{
  "tool": "interact",
  "arguments": {
    "what": "clipboard_write",
    "text": "Copied by KaBOOM"
  }
}
```

#### Expected response shape

```json
{
  "action": "clipboard_write",
  "ok": true,
  "url": "https://example.com",
  "result": {
    "summary": "Action completed",
    "mode": "clipboard_write"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "interact",
  "arguments": {
    "what": "not_a_real_mode",
    "text": "Copied by KaBOOM"
  }
}
```

Fix: Use a valid interact action value, e.g. `clipboard_write`.
