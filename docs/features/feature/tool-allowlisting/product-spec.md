---
feature: tool-allowlisting
status: proposed
tool: configure
mode: security
version: 0.7.12
doc_type: product-spec
feature_id: feature-tool-allowlisting
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Tool Allowlisting

## Problem Statement

Different environments require different levels of access. Production needs observation-only. Staging might allow interactions but no configuration changes. Development needs full access. Read-only mode is too coarse (all or nothing). Organizations need granular control over which MCP tools and actions are available.

## Solution

Add tool allowlisting via configuration file. Define which tools (observe, generate, configure, interact) and which specific actions within each tool are permitted. Server enforces allowlist at MCP handler level. Attempts to call non-allowlisted tools return clear errors. Configuration supports per-environment profiles (production, staging, development).

## Requirements

- Allowlist at tool level (e.g., allow observe, block interact)
- Allowlist at action level (e.g., allow interact.navigate, block interact.execute_js)
- Configuration via YAML file: --allowlist-config=/path/to/allowlist.yaml
- Default allowlist: all tools/actions enabled (backwards compatible)
- Support wildcards (e.g., "observe.*" allows all observe modes)
- Clear error messages when non-allowlisted tool called
- Allowlist applies server-wide (all clients subject to same rules)
- Hot-reload configuration without server restart (optional)

## Out of Scope

- Per-client allowlists (all clients share same allowlist)
- Per-tab allowlists
- Time-based access windows
- Dynamic allowlist modification via MCP (security boundary)

## Success Criteria

- Admin can configure production profile (observe-only)
- Admin can configure staging profile (observe + interact.navigate)
- Non-allowlisted tools fail with clear error
- Agent can query current allowlist via configure({action:"health"})

## User Workflow

1. Admin creates allowlist.yaml defining permitted tools/actions
2. Start server: `kaboom --allowlist-config=allowlist.yaml`
3. Agent connects, queries allowlist: `configure({action:"health"})`
4. Agent calls allowed tool: succeeds
5. Agent calls non-allowed tool: fails with "tool_not_allowed" error

## Examples

### Production profile (observation-only):
```yaml
# allowlist-production.yaml
allowed_tools:
  - observe.*          # All observe modes
  - generate.*         # All generate types
  - analyze.dom    # DOM queries only
  - configure.health       # Health checks
```

## Staging profile (observe + safe interactions):
```yaml
# allowlist-staging.yaml
allowed_tools:
  - observe.*
  - generate.*
  - analyze.dom
  - configure.health
  - interact.navigate      # Allow navigation
  - interact.refresh       # Allow page refresh
  # Block: execute_js, fill_form, drag_drop
```

## Development profile (full access):
```yaml
# allowlist-development.yaml
allowed_tools:
  - "*"    # All tools and actions
```

## Query current allowlist:
```json
configure({action:"health"})
// Returns:
{
  "allowlist_enabled": true,
  "allowed_tools": ["observe.*", "generate.*", "analyze.dom", "interact.navigate"],
  "read_only_mode": false
}
```

## Non-allowed tool call:
```json
interact({action: "execute_js", code: "alert('test')"})
// Returns:
{
  "error": "tool_not_allowed",
  "message": "interact.execute_js is not permitted by current allowlist. Allowed: observe.*, generate.*, interact.navigate",
  "allowed_tools": ["observe.*", "generate.*", "interact.navigate"]
}
```

---

## Notes

- Allowlist is enforced at server MCP handler level (cannot be bypassed)
- Compatible with read-only mode (read-only is broader, allowlist is granular)
- Configuration file supports comments, environment-specific profiles
