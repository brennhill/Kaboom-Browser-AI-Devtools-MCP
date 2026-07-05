---
feature: config-profiles
status: proposed
doc_type: tech-spec
feature_id: feature-config-profiles
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Configuration Profiles

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Configuration profiles are YAML files mapping setting names to values. At server startup, if --profile specified, load profile YAML, parse into config struct, then apply CLI flags (overriding profile). Profiles are syntactic sugar for CLI flags — no new runtime behavior, just convenient bundling.

## Key Components

- **Profile loader**: Parse YAML profile file at startup
- **Config merger**: Merge profile settings with CLI flags (CLI wins)
- **Profile validator**: Validate profile has no invalid keys or values
- **Built-in profiles**: Ship with Kaboom in profiles/ directory
- **Config exposure**: Include active profile name in configure({action:"health"})

## Data Flows

```
Server startup: kaboom --profile=restricted
  → Load profiles/restricted.yaml
  → Parse YAML into map[string]interface{}
  → Set config defaults from profile:
      config.ReadOnlyMode = profile["read_only"]
      config.AllowlistConfig = profile["allowlist_config"]
      config.ProjectExpirationMinutes = profile["project_expiration_minutes"]
  → Parse CLI flags
  → Apply CLI flags (override profile if set):
      if cli.ReadOnlyMode != nil { config.ReadOnlyMode = cli.ReadOnlyMode }
  → Validate config (fail if invalid combo)
  → Log: "Profile 'restricted' loaded, X settings applied"
```

## Implementation Strategy

### Profile YAML format:
- Flat key-value structure (no nesting)
- Keys match CLI flag names (snake_case)
- Values are typed (bool, string, int)

### Profile loading:
1. Check --profile flag or KABOOM_PROFILE env var
2. If set, load file from profiles/{name}.yaml or absolute path
3. Parse YAML into map
4. Iterate map, set corresponding config fields

### CLI flag override logic:
- CLI flags are parsed AFTER profile loading
- If CLI flag is explicitly set (not default), override profile value
- Use flag library's "Changed" method to detect explicit vs default

### Built-in profiles location:
- Ship with binary: embed profiles/*.yaml using go:embed
- User can override with --profile=/custom/path.yaml

### Profile validation:
- Check all keys in profile are recognized (typo detection)
- Check values match expected types (bool not string, etc.)
- Check no conflicting settings (e.g., read_only=true but allowlist allows mutations)
- Fail fast with clear error if invalid

## Edge Cases & Assumptions

- **Edge Case 1**: Profile file not found → **Handling**: Fail startup with "Profile X not found"
- **Edge Case 2**: Invalid YAML syntax → **Handling**: Fail startup with parse error
- **Edge Case 3**: Profile has unknown key → **Handling**: Warn, ignore unknown keys (forward compatibility)
- **Edge Case 4**: CLI flag conflicts with profile → **Handling**: CLI flag wins, log warning
- **Assumption 1**: Profiles are loaded once at startup (no hot-reload)
- **Assumption 2**: Profiles don't inherit (no "extends" mechanism)

## Risks & Mitigations

- **Risk 1**: Profile has insecure defaults → **Mitigation**: Review all built-in profiles for security
- **Risk 2**: User loads wrong profile (dev in production) → **Mitigation**: Log active profile prominently
- **Risk 3**: Profile misconfiguration breaks server → **Mitigation**: Validate at startup, fail fast
- **Risk 4**: Profile and CLI flags conflict → **Mitigation**: Clear precedence (CLI > profile), log overrides

## Dependencies

- YAML parser
- CLI flag library
- Config struct

## Performance Considerations

- Profile loading happens once at startup (no runtime cost)
- YAML parsing is fast (<1ms for small profiles)

## Security Considerations

- **Profiles as security boundaries**: Paranoid profile enforces max security
- **Profile tampering**: Profiles are file-based, protect with file permissions
- **Built-in profiles**: Embed in binary to prevent tampering
- **Validation**: Ensure profiles don't accidentally weaken security
