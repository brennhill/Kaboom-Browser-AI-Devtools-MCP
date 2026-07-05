---
feature: dynamic-exposure
status: proposed
tool: configure
mode: feature-flags
version: 0.7.12
doc_type: product-spec
feature_id: feature-dynamic-exposure
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Dynamic Exposure

## Problem Statement

New features may have bugs or unexpected behavior in production. Rolling out features incrementally (canary, gradual rollout) requires dynamic enable/disable without code changes or server restarts. Organizations need runtime feature flags to control capability exposure for risk mitigation.

## Solution

Add feature flag system for dynamic capability gating. Flags are boolean toggles (enabled/disabled) stored in configuration file or environment variables. Server checks flags at runtime before exposing tools/actions. Flags can be hot-reloaded without restart. Use cases: gradual rollout, emergency kill switch, A/B testing, beta features.

## Requirements

- Feature flags for major capabilities (e.g., interact.execute_js, generate.har, observe.accessibility)
- Configuration via YAML file or environment variables
- Hot-reload: update flags without server restart (watch config file)
- Default: all flags enabled (backwards compatible)
- Flag status visible via observe({what: "feature_flags"})
- Disabled features return clear error: "feature_disabled" with flag name
- Override flags via CLI for emergency: --disable-feature=execute_js

## Out of Scope

- Percentage-based rollouts (0-100% traffic) — MVP is binary on/off
- Per-user feature flags (server-wide only)
- Feature flag analytics (usage tracking)
- Time-based auto-expiration of flags

## Success Criteria

- Admin can disable interact.execute_js via config file, agents receive clear error
- Hot-reload: update config file, new flag takes effect within 10 seconds
- Emergency disable via CLI: --disable-feature=execute_js overrides config

## User Workflow

1. Admin defines feature flags in features.yaml
2. Start server: `kaboom --feature-flags=features.yaml`
3. Agent calls feature, server checks flag before execution
4. If disabled, return error with flag name
5. Admin updates features.yaml (enable/disable)
6. Server detects change, reloads flags (hot-reload)
7. New flag status takes effect immediately

## Examples

### Feature flags config:
```yaml
# features.yaml
features:
  interact_execute_js: true      # Enabled
  interact_fill_form: true       # Enabled
  generate_har: false            # Disabled (beta feature)
  observe_accessibility: true    # Enabled
  interact_drag_drop: false      # Disabled (rolling out gradually)
```

## Start with feature flags:
```bash
kaboom --feature-flags=features.yaml
```

## Query feature flags:
```json
observe({what: "feature_flags"})
// Returns:
{
  "interact_execute_js": true,
  "interact_fill_form": true,
  "generate_har": false,
  "observe_accessibility": true,
  "interact_drag_drop": false
}
```

## Disabled feature call:
```json
generate({format: "har"})
// Returns:
{
  "error": "feature_disabled",
  "message": "The 'har' generation feature is currently disabled. Contact administrator to enable.",
  "feature_flag": "generate_har"
}
```

## Emergency disable via CLI:
```bash
# Override config, force disable
kaboom --feature-flags=features.yaml --disable-feature=interact_execute_js
```

## Hot-reload:
```bash
# Admin edits features.yaml, changes generate_har: false → true
# Server detects file change within 10 seconds
# New requests: generate({format: "har"}) now succeeds
```

---

## Notes

- Feature flags gate capabilities at runtime (not compile-time)
- Compatible with allowlisting (allowlist is coarse, flags are fine-grained)
- Flags apply server-wide (all clients subject to same flags)
- Hot-reload enables rapid response to issues without downtime
