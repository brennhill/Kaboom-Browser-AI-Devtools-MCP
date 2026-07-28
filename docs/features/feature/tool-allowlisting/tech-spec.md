---
feature: tool-allowlisting
status: proposed
doc_type: tech-spec
feature_id: feature-tool-allowlisting
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Tool Allowlisting

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Tool allowlisting uses a configuration file (YAML) parsed at server start. The allowlist defines permitted tool.action combinations (e.g., "observe.*", "interact.navigate"). Each MCP tool handler checks incoming request against allowlist before execution. Non-allowed tools return error immediately. Allowlist is stored in memory, optionally reloaded on file change (hot-reload).

## Key Components

- **Config parser**: Parse YAML allowlist file into in-memory structure
- **Allowlist matcher**: Match incoming tool.action against allowlist patterns (supports wildcards)
- **MCP handler guards**: Each handler checks allowlist before execution
- **Error response**: Return standard error with actionable message and allowed tools list
- **Config exposure**: Include allowlist in configure({what:"health"})
- **Hot-reload (optional)**: Watch config file, reload on change

## Data Flows

```
Server startup: kaboom --allowlist-config=allowlist.yaml
  → Parse YAML file
  → Build in-memory allowlist: {observe: [*], generate: [*], interact: [navigate, refresh]}
  → Log: "Allowlist enabled, X tools/actions permitted"

Agent calls interact({what: "execute_js"}):
  → Server receives MCP request
  → Extract tool="interact", action="execute_js"
  → Check allowlist: matcher.IsAllowed("interact.execute_js")
  → Match fails (not in list)
  → Return error with allowed_tools list

Agent calls interact({what: "navigate"}):
  → Check allowlist: matcher.IsAllowed("interact.navigate")
  → Match succeeds
  → Execute normally

Agent calls configure({what:"health"}):
  → Always allowed (observe.* in list)
  → Return config including allowlist
```

## Implementation Strategy

### Allowlist file format (YAML):
```yaml
# Comments supported
allowed_tools:
  - "observe.*"             # Wildcard: all observe modes
  - "generate.reproduction" # Specific: only reproduction
  - "interact.navigate"     # Specific: only navigate
  - "interact.refresh"      # Specific: only refresh
```

## Pattern matching:
- Exact match: "interact.navigate" matches only that action
- Wildcard tool: "observe.*" matches all observe modes
- Wildcard all: "*" matches everything (dev profile)
- Case-insensitive matching

## Allowlist matcher algorithm:
1. Parse incoming request: extract tool name, action name
2. Build qualified name: "{tool}.{action}" (e.g., "interact.navigate")
3. Iterate allowlist patterns:
   - If pattern is "*", allow
   - If pattern is "{tool}.*", check tool matches
   - If pattern is exact match, check exact match
4. If any pattern matches, allow; else deny

## Error response format:
```json
{
  "error": "tool_not_allowed",
  "message": "interact.execute_js is not permitted. Allowed: observe.*, generate.*, interact.navigate",
  "tool": "interact",
  "action": "execute_js",
  "allowed_tools": ["observe.*", "generate.*", "interact.navigate"]
}
```

## Hot-reload (optional):
- Use file watcher (fsnotify or similar) to detect config file changes
- On change, re-parse YAML, update in-memory allowlist
- Log: "Allowlist reloaded, X tools/actions permitted"
- If parse fails, keep old allowlist, log error

## Edge Cases & Assumptions

- **Edge Case 1**: Empty allowlist file → **Handling**: Default to all allowed (backwards compatible)
- **Edge Case 2**: Invalid YAML syntax → **Handling**: Server fails to start with clear error
- **Edge Case 3**: Conflicting patterns ("observe.*" and "!observe.logs") → **Handling**: No deny syntax, only allow (simpler model)
- **Edge Case 4**: Agent retries non-allowed tool → **Handling**: Return same error every time
- **Assumption 1**: Allowlist is uniform across all clients (no per-client rules)
- **Assumption 2**: Hot-reload is optional, not required for MVP

## Risks & Mitigations

- **Risk 1**: Misconfigured allowlist blocks all tools → **Mitigation**: Validate config at startup, fail fast with clear error
- **Risk 2**: Allowlist too complex, hard to maintain → **Mitigation**: Support simple wildcard syntax only (no regex)
- **Risk 3**: Agent doesn't handle error gracefully → **Mitigation**: Return clear error with list of allowed tools
- **Risk 4**: Hot-reload causes race condition → **Mitigation**: Use RWMutex for allowlist access

## Dependencies

- YAML parser (gopkg.in/yaml.v3 or similar stdlib)
- MCP tool handlers
- Server config struct

## Performance Considerations

- Allowlist check is O(N) where N = number of patterns (typically <20)
- In-memory matching is fast (<0.1ms per request)
- Hot-reload: file watch overhead negligible (<1ms)

## Security Considerations

- **Immutable from clients**: Allowlist cannot be modified via MCP (only file-based)
- **Server-side enforcement**: Cannot be bypassed by clients
- **Clear audit trail**: All blocked attempts logged
- **Fail-safe default**: If config parsing fails, fall back to restrictive or fail-to-start (configurable)
- **No deny rules**: Only allow rules (simpler, less error-prone)
