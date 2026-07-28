---
feature: config-profiles
status: proposed
tool: configure
mode: profiles
version: 0.7.12
doc_type: product-spec
feature_id: feature-config-profiles
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Configuration Profiles

## Problem Statement

Configuring Kaboom for different environments (production, staging, development) requires setting multiple flags (read-only, allowlist, retention, redaction, project expiration). Manually specifying all flags is error-prone. Organizations need pre-tuned configuration bundles that compose best-practice settings for common scenarios.

## Solution

Add configuration profiles as named bundles of settings. Profiles are YAML files defining multiple configuration options. Users specify profile via --profile flag or environment variable. Pre-defined profiles: paranoid (max security), restricted (production-safe), short-lived (ephemeral contexts), development (full access). Profiles compose settings that would otherwise require many CLI flags.

## Requirements

- Pre-defined profiles: paranoid, restricted, short-lived, development
- Custom profiles: users can define own YAML profiles
- Profile loading: --profile=paranoid or KABOOM_PROFILE=paranoid
- Profile composition: profile sets defaults, CLI flags can override
- Clear documentation of what each profile configures
- Profile validation: fail fast if profile has invalid settings

## Out of Scope

- Dynamic profile switching (requires server restart)
- Per-client profiles (server-wide only)
- Profile inheritance (no profile extends another)

## Success Criteria

- User can start server with --profile=paranoid, all security settings applied
- Custom profiles can be defined and loaded
- CLI flags override profile settings (for fine-tuning)
- Invalid profile fails server startup with clear error

## User Workflow

1. User selects appropriate profile for environment
2. Start server: `kaboom --profile=restricted`
3. Server loads profile, applies all settings
4. Agent connects, operates under profile constraints
5. User can query active profile: `configure({what:"health"})`

## Examples

### Pre-defined profiles:

### Paranoid profile (max security):
```yaml
# profiles/paranoid.yaml
read_only: true
allowlist_config: profiles/allowlist-readonly.yaml
project_expiration_minutes: 15
redaction_aggressive: true
network_body_capture: false
websocket_capture: false
retention_hours: 1
```

## Restricted profile (production-safe):
```yaml
# profiles/restricted.yaml
read_only: false
allowlist_config: profiles/allowlist-restricted.yaml  # observe + generate + safe interact
project_expiration_minutes: 60
redaction_standard: true
network_body_capture: false
retention_hours: 24
```

## Short-lived profile (ephemeral):
```yaml
# profiles/short-lived.yaml
project_expiration_minutes: 10
retention_hours: 1
clear_on_disconnect: true
```

## Development profile (full access):
```yaml
# profiles/development.yaml
read_only: false
allowlist_config: null  # all tools allowed
network_body_capture: true
websocket_capture: true
retention_hours: 168  # 1 week
```

## Load profile:
```bash
kaboom --profile=restricted
# or
KABOOM_PROFILE=restricted kaboom
```

## Override profile setting:
```bash
# Load restricted profile but override expiration
kaboom --profile=restricted --project-expiration-minutes=30
```

## Query active profile:
```json
configure({what:"health"})
// Returns:
{
  "profile": "restricted",
  "read_only_mode": false,
  "allowlist_enabled": true,
  "project_expiration_minutes": 60,
  "redaction": "standard"
}
```

---

## Notes

- Profiles are convenience wrappers around existing flags (no new capabilities)
- CLI flags always override profile settings (explicit > implicit)
- Profiles shipped with Kaboom in profiles/ directory
