---
feature: project-isolation
status: proposed
doc_type: tech-spec
feature_id: feature-project-isolation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Project Isolation

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Project isolation is implemented via a project registry (map of project_key to ProjectContext). Each ProjectContext contains dedicated buffers (logs, network, websocket, etc.) and metadata (created time, last activity). MCP clients provide project_key on initialize. Extension provides project_key via HTTP header. All data operations are scoped to project_key. Background goroutine expires inactive projects (default 1 hour timeout).

## Key Components

- **Project registry**: Map[string]*ProjectContext, guarded by RWMutex
- **ProjectContext struct**: Contains all buffers (logs, network, websocket, vitals, pending queries, etc.) and metadata
- **MCP initialize handler**: Extract project_key from init params, create/get ProjectContext
- **HTTP middleware**: Extract X-Kaboom-Project header, route to correct ProjectContext
- **Expiration goroutine**: Periodically scan projects, delete inactive ones
- **Observe integration**: Filter data by project_key
- **Configure/Interact integration**: Scope operations to project_key

## Data Flows

```
Agent A connects via MCP:
  → initialize({project_key: "projectA"})
  → Server: registry.GetOrCreate("projectA")
  → Create ProjectContext with empty buffers
  → Associate session with projectA

Extension POSTs log:
  → HTTP header: X-Kaboom-Project: projectA
  → Server: extract header, get ProjectContext
  → Append log to projectA.logBuffer

Agent A observes logs:
  → observe({what: "logs"})
  → Server: lookup session project_key (projectA)
  → Return projectA.logBuffer
  → Agent A sees only projectA logs

Agent B with project_key="projectB":
  → Same flow, but separate ProjectContext
  → Cannot access projectA data

Expiration:
  → Background goroutine runs every 5 minutes
  → For each project: if time.Since(last_activity) > 1 hour
  → Delete from registry: delete(registry, projectKey)
  → Log: "Project {key} expired, buffers freed"
```

## Implementation Strategy

### ProjectContext structure:
- logBuffer: ring buffer (1000 entries)
- extensionLogBuffer: ring buffer (500 entries)
- websocketBuffer: ring buffer (500 events)
- networkBodies: ring buffer (100 bodies)
- networkWaterfall: ring buffer (1000 entries)
- connections: active + closed connections
- pendingQueries: async command queue
- metadata: {created: timestamp, last_activity: timestamp}

### Project registry:
- Map[string]*ProjectContext
- Protected by sync.RWMutex
- GetOrCreate method: if not exists, create new ProjectContext
- UpdateActivity method: set last_activity = now

### MCP integration:
- In initialize handler, extract project_key from params (default "default")
- Store project_key in session state
- All subsequent MCP calls use session's project_key to lookup ProjectContext

### HTTP integration:
- Middleware extracts X-Kaboom-Project header (default "default")
- Route to correct ProjectContext for data storage
- Return 400 if project_key invalid characters (security: prevent injection)

### Expiration strategy:
- Background goroutine: ticker every 5 minutes
- Iterate registry, check each project's last_activity
- If inactive > threshold (default 1 hour), delete from map
- Configurable via CLI: --project-expiration-minutes=60

## Edge Cases & Assumptions

- **Edge Case 1**: Project_key collision (two clients same key) → **Handling**: Expected behavior, they share data (same project)
- **Edge Case 2**: No project_key provided → **Handling**: Use "default" project (backwards compatible)
- **Edge Case 3**: Extension sends to expired project → **Handling**: Auto-recreate project on demand
- **Edge Case 4**: Project_key with special characters → **Handling**: Validate alphanumeric + dash only, reject others
- **Assumption 1**: Project keys are provided by trusted clients (no auth in MVP)
- **Assumption 2**: Projects are ephemeral (no persistence across restarts)

## Risks & Mitigations

- **Risk 1**: Memory exhaustion from too many projects → **Mitigation**: Aggressive expiration, warn if >100 active projects
- **Risk 2**: Accidental data leakage via default project → **Mitigation**: Document that default project is shared
- **Risk 3**: Project key guessing/snooping → **Mitigation**: Future: add project auth tokens, out of scope for MVP
- **Risk 4**: Expiration too aggressive, deletes active projects → **Mitigation**: Track activity on every operation, default 1 hour is conservative

## Dependencies

- Existing buffer implementations (logBuffer, networkBuffer, etc.)
- MCP session management
- HTTP middleware/routing

## Performance Considerations

- Registry lookup: O(1) map access with RWMutex (fast)
- Per-project memory: ~50MB (worst case, all buffers full)
- Max projects: recommend <100 active, warn above threshold
- Expiration scan: O(N) where N = active projects, runs every 5 minutes (negligible)

## Security Considerations

- **Data isolation**: No cross-project access possible (enforced at data layer)
- **No authentication in MVP**: Any client can access any project_key (assume trusted environment)
- **Future enhancement**: Project auth tokens (require token to access project)
- **Injection prevention**: Validate project_key format (alphanumeric, dash, underscore only)
- **Memory DoS**: Expiration and project count limits prevent unbounded growth
- **Audit trail**: Log project creation, expiration, access attempts
