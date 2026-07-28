---
feature: dynamic-exposure
status: proposed
doc_type: tech-spec
feature_id: feature-dynamic-exposure
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Dynamic Exposure

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Dynamic exposure uses runtime feature flags stored in YAML config file. Server loads flags at startup, stores in memory (map[string]bool, guarded by RWMutex). File watcher detects config changes, triggers hot-reload. Each tool handler checks feature flag before execution. If disabled, return error immediately. CLI flags can force-disable features (override config).

## Key Components

- **Feature flag registry**: Map[string]bool, protected by RWMutex
- **Config loader**: Parse YAML feature flags file
- **File watcher**: Detect config file changes, trigger reload
- **Feature gate guards**: Check flag before executing feature
- **Error response**: Return "feature_disabled" with flag name
- **CLI override**: --disable-feature flag forces disable
- **Observe integration**: Expose current flags via observe({what: "feature_flags"})

## Data Flows

```
Server startup: kaboom --feature-flags=features.yaml
  → Load features.yaml
  → Parse into map: {"interact_execute_js": true, "generate_har": false}
  → Store in registry with RWMutex
  → Start file watcher on features.yaml
  → Log: "Feature flags loaded, X features enabled"

Agent calls generate({what: "har"}):
  → Handler: check registry.IsEnabled("generate_har")
  → Registry: RLock, lookup, RUnlock → false
  → Return error: {error: "feature_disabled", feature_flag: "generate_har"}

Admin edits features.yaml (generate_har: false → true):
  → File watcher detects change
  → Reload: parse YAML, update registry with WLock
  → Log: "Feature flags reloaded, generate_har enabled"

Next request: generate({what: "har"}):
  → Check flag: now true
  → Execute normally
```

## Implementation Strategy

### Feature flag naming convention:
- Format: {tool}_{action} (e.g., "interact_execute_js", "generate_har")
- Underscore-separated, lowercase
- Map to tool and action names in code

### Registry implementation:
- Map[string]bool with sync.RWMutex
- IsEnabled(flag string) bool: RLock, lookup, return (default true if not found)
- SetFlag(flag string, enabled bool): WLock, set, WUnlock
- LoadFromYAML(path string): parse YAML, bulk update registry

### File watching:
- Use fsnotify (or similar) to watch config file
- On change event: debounce (wait 1s for write completion), reload YAML
- If reload fails: log error, keep existing flags (don't crash)

### Feature gate pattern:
```
In each tool handler:
if !featureFlags.IsEnabled("tool_action") {
  return error: "feature_disabled"
}
// Proceed with execution
```

### CLI override:
- --disable-feature=execute_js flag parses into list
- After loading config, force-set these flags to false
- CLI overrides take precedence (cannot be hot-reloaded)

### Error response format:
```json
{
  "error": "feature_disabled",
  "message": "The 'har' generation feature is currently disabled. Contact administrator to enable 'generate_har' flag.",
  "feature_flag": "generate_har",
  "feature_status": "disabled"
}
```

## Edge Cases & Assumptions

- **Edge Case 1**: Feature flag not defined in config → **Handling**: Default to enabled (backwards compatible)
- **Edge Case 2**: Config file deleted → **Handling**: Log error, keep existing flags
- **Edge Case 3**: Invalid YAML syntax → **Handling**: Log error, keep existing flags (don't crash)
- **Edge Case 4**: Concurrent hot-reload and request → **Handling**: RWMutex ensures consistency
- **Assumption 1**: Flags are binary (on/off), no percentage rollouts
- **Assumption 2**: Hot-reload delay (10s) is acceptable

## Risks & Mitigations

- **Risk 1**: Flag misconfiguration disables critical features → **Mitigation**: Validate config, log prominently, default to enabled
- **Risk 2**: Hot-reload race condition → **Mitigation**: RWMutex for thread-safe access
- **Risk 3**: File watcher fails, flags stale → **Mitigation**: Log warning, manual reload endpoint
- **Risk 4**: Too many flags, hard to manage → **Mitigation**: Document flag hierarchy, use sparingly

## Dependencies

- YAML parser
- File watcher library (fsnotify)
- sync.RWMutex for concurrency

## Performance Considerations

- Flag check: O(1) map lookup with RLock (fast, <0.01ms)
- Hot-reload: triggered by file change, parses YAML (1-10ms), no impact on requests
- RWMutex allows concurrent reads (no lock contention)

## Security Considerations

- **Flag tampering**: Protect features.yaml with file permissions
- **Feature disable as security**: Disable risky features quickly in emergency
- **Audit trail**: Log all flag changes, who changed what when
- **CLI override**: Emergency disable cannot be bypassed by config reload
- **No remote API**: Flags cannot be toggled via MCP (file-based only, prevents unauthorized changes)
