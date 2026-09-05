---
title: "Configure Executable Examples"
description: "Runnable examples for every configure mode with response shapes and failure fixes."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['reference', 'examples', 'configure']
---

# Configure Executable Examples

Each section provides one runnable baseline call, expected response shape, and one failure example with a concrete fix. Use these as copy/paste starters and then adjust for your page or workflow.

## Quick Reference

```json
{
  "tool": "configure",
  "arguments": {
    "what": "store"
  }
}
```

## Common Parameters

| Parameter | Type | Description |
| --- | --- | --- |
| `what` | string | Mode name to execute. |
| `tab_id` | number | Optional target browser tab. |
| `telemetry_mode` | string | Optional telemetry verbosity: `off`, `auto`, `full`. |

## Modes

### `store`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "store",
    "key": "project.current",
    "data": {
      "value": "checkout-redesign"
    }
  }
}
```

#### Expected response shape

```json
{
  "what": "store",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "store"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "key": "project.current",
    "data": {
      "value": "checkout-redesign"
    }
  }
}
```

Fix: Use a valid configure mode value, e.g. `store`.

### `load`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "load",
    "key": "project.current"
  }
}
```

#### Expected response shape

```json
{
  "what": "load",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "load"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "key": "project.current"
  }
}
```

Fix: Use a valid configure mode value, e.g. `load`.

### `noise_rule`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "noise_rule",
    "noise_action": "add",
    "category": "console",
    "message_regex": "ResizeObserver loop limit exceeded"
  }
}
```

#### Expected response shape

```json
{
  "what": "noise_rule",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "noise_rule"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "noise_action": "add",
    "category": "console",
    "message_regex": "ResizeObserver loop limit exceeded"
  }
}
```

Fix: Use a valid configure mode value, e.g. `noise_rule`.

### `clear`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "clear"
  }
}
```

#### Expected response shape

```json
{
  "what": "clear",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "clear"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `clear`.

### `health`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "health"
  }
}
```

#### Expected response shape

```json
{
  "what": "health",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "health"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `health`.

### `tutorial`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "tutorial"
  }
}
```

#### Expected response shape

```json
{
  "what": "tutorial",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "tutorial"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `tutorial`.

### `examples`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "examples"
  }
}
```

#### Expected response shape

```json
{
  "what": "examples",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "examples"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `examples`.

### `streaming`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "streaming"
  }
}
```

#### Expected response shape

```json
{
  "what": "streaming",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "streaming"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `streaming`.

### `test_boundary_start`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "test_boundary_start"
  }
}
```

#### Expected response shape

```json
{
  "what": "test_boundary_start",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "test_boundary_start"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `test_boundary_start`.

### `test_boundary_end`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "test_boundary_end"
  }
}
```

#### Expected response shape

```json
{
  "what": "test_boundary_end",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "test_boundary_end"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `test_boundary_end`.

### `event_recording_start`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "event_recording_start"
  }
}
```

#### Expected response shape

```json
{
  "what": "event_recording_start",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "event_recording_start"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `event_recording_start`.

### `event_recording_stop`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "event_recording_stop"
  }
}
```

#### Expected response shape

```json
{
  "what": "event_recording_stop",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "event_recording_stop"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `event_recording_stop`.

### `playback`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "playback",
    "recording_id": "rec_123"
  }
}
```

#### Expected response shape

```json
{
  "what": "playback",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "playback"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "playback",
    "recording_id": 123
  }
}
```

Fix: Use `recording_id` as a string like `rec_123`.

### `log_diff`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "log_diff",
    "original_id": "rec_123",
    "replay_id": "rec_456"
  }
}
```

#### Expected response shape

```json
{
  "what": "log_diff",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "log_diff"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "original_id": "rec_123",
    "replay_id": "rec_456"
  }
}
```

Fix: Use a valid configure mode value, e.g. `log_diff`.

### `telemetry`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "telemetry"
  }
}
```

#### Expected response shape

```json
{
  "what": "telemetry",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "telemetry"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `telemetry`.

### `describe_capabilities`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "describe_capabilities"
  }
}
```

#### Expected response shape

```json
{
  "what": "describe_capabilities",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "describe_capabilities"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `describe_capabilities`.

### `diff_sessions`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "diff_sessions"
  }
}
```

#### Expected response shape

```json
{
  "what": "diff_sessions",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "diff_sessions"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `diff_sessions`.

### `audit_log`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "audit_log"
  }
}
```

#### Expected response shape

```json
{
  "what": "audit_log",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "audit_log"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `audit_log`.

### `restart`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "restart"
  }
}
```

#### Expected response shape

```json
{
  "what": "restart",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "restart"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `restart`.

### `save_sequence`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "save_sequence",
    "name": "checkout-smoke",
    "steps": [
      {
        "what": "navigate",
        "url": "https://example.com"
      },
      {
        "what": "click",
        "selector": "text=Checkout"
      }
    ]
  }
}
```

#### Expected response shape

```json
{
  "what": "save_sequence",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "save_sequence"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "save_sequence",
    "name": "checkout-smoke",
    "steps": "navigate,click"
  }
}
```

Fix: Use `steps` as an array of action objects.

### `get_sequence`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "get_sequence",
    "name": "checkout-smoke"
  }
}
```

#### Expected response shape

```json
{
  "what": "get_sequence",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "get_sequence"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "name": "checkout-smoke"
  }
}
```

Fix: Use a valid configure mode value, e.g. `get_sequence`.

### `list_sequences`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "list_sequences"
  }
}
```

#### Expected response shape

```json
{
  "what": "list_sequences",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "list_sequences"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `list_sequences`.

### `delete_sequence`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "delete_sequence",
    "name": "checkout-smoke"
  }
}
```

#### Expected response shape

```json
{
  "what": "delete_sequence",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "delete_sequence"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "name": "checkout-smoke"
  }
}
```

Fix: Use a valid configure mode value, e.g. `delete_sequence`.

### `replay_sequence`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "replay_sequence",
    "name": "checkout-smoke"
  }
}
```

#### Expected response shape

```json
{
  "what": "replay_sequence",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "replay_sequence"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "name": "checkout-smoke"
  }
}
```

Fix: Use a valid configure mode value, e.g. `replay_sequence`.

### `doctor`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "doctor"
  }
}
```

#### Expected response shape

```json
{
  "what": "doctor",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "doctor"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode"
  }
}
```

Fix: Use a valid configure mode value, e.g. `doctor`.

### `security_mode`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "security_mode",
    "mode": "normal"
  }
}
```

#### Expected response shape

```json
{
  "what": "security_mode",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "security_mode"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "mode": "normal"
  }
}
```

Fix: Use a valid configure mode value, e.g. `security_mode`.

### `network_recording`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "network_recording",
    "operation": "start",
    "domain": "example.com"
  }
}
```

#### Expected response shape

```json
{
  "what": "network_recording",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "network_recording"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "operation": "start",
    "domain": "example.com"
  }
}
```

Fix: Use a valid configure mode value, e.g. `network_recording`.

### `action_jitter`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "action_jitter",
    "action_jitter_ms": 120
  }
}
```

#### Expected response shape

```json
{
  "what": "action_jitter",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "action_jitter"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "action_jitter_ms": 120
  }
}
```

Fix: Use a valid configure mode value, e.g. `action_jitter`.

### `report_issue`

#### Minimal call

```json
{
  "tool": "configure",
  "arguments": {
    "what": "report_issue",
    "operation": "preview",
    "title": "Intermittent checkout timeout"
  }
}
```

#### Expected response shape

```json
{
  "what": "report_issue",
  "ok": true,
  "result": {
    "summary": "Configuration updated",
    "mode": "report_issue"
  }
}
```

#### Failure example and fix

```json
{
  "tool": "configure",
  "arguments": {
    "what": "not_a_real_mode",
    "operation": "preview",
    "title": "Intermittent checkout timeout"
  }
}
```

Fix: Use a valid configure mode value, e.g. `report_issue`.
