---
feature: project-isolation
status: proposed
tool: configure
mode: multi-tenancy
version: 0.7.12
doc_type: product-spec
feature_id: feature-project-isolation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Project Isolation

## Problem Statement

Single Kaboom server instance shared across multiple projects (or multiple agents working on different tabs) risks data leakage. Logs, network events, and captured telemetry from Project A should not be visible to Project B. Organizations need isolated capture contexts on one server.

## Solution

Add project isolation via project keys. Each client connects with a unique project_key (via MCP initialization or HTTP header). Server maintains separate memory buffers (logs, network, websocket, etc.) per project. Clients can only access data associated with their project key. Projects are isolated at data level, not process level (single server process, multiple logical contexts).

## Requirements

- Client specifies project_key on connection (MCP init param or HTTP header X-Kaboom-Project)
- Server creates separate buffers per project: logs, network, websocket, vitals, queries
- Cross-project data access forbidden (client can only see own project's data)
- Default project: if no key specified, use "default" (backwards compatible)
- Project lifecycle: auto-created on first use, auto-expired after inactivity timeout (default 1 hour)
- Observe tool filtered by project_key
- Configure tool scoped to project_key
- Interact tool (pending queries) scoped to project_key

## Out of Scope

- Per-project allowlists (server-wide allowlist applies to all projects)
- Per-project read-only mode
- Cross-project data sharing (deliberate isolation)
- Persistent projects across server restarts (in-memory only)

## Success Criteria

- Agent A (project_key="projectA") cannot see Agent B's (project_key="projectB") logs
- Multiple agents can work on same server without data leakage
- Extension can send telemetry to specific project via HTTP header
- Projects auto-expire after inactivity (memory cleanup)

## User Workflow

1. Agent A connects: `mcp connect --init-param project_key=projectA`
2. Agent B connects: `mcp connect --init-param project_key=projectB`
3. Extension sends telemetry with header: `X-Kaboom-Project: projectA`
4. Agent A observes logs: sees only projectA logs
5. Agent B observes logs: sees only projectB logs
6. After 1 hour inactivity, projectA buffer expires, memory freed

## Examples

### MCP connection with project_key:
```json
// MCP initialize request
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {
    "project_key": "myapp-staging"
  }
}
```

### Extension HTTP request with project:
```http
POST /logs HTTP/1.1
X-Kaboom-Project: myapp-staging
Content-Type: application/json

{"level": "error", "message": "..."}
```

### Observe filtered by project:
```json
// Agent with project_key="projectA"
observe({what: "logs"})
// Returns only projectA logs, not projectB

// Server internally:
// return server.projects["projectA"].logBuffer
```

### Project expiration:
```json
// After 1 hour of no activity
// Server: detect last_activity > 1 hour
// Server: delete(server.projects["projectA"])
// Server: log "Project projectA expired, buffers freed"
```

### List active projects (admin):
```json
observe({what: "projects"})
// Returns:
{
  "projects": [
    {"key": "projectA", "created": "2026-01-28T10:00:00Z", "last_activity": "2026-01-28T10:30:00Z"},
    {"key": "projectB", "created": "2026-01-28T10:15:00Z", "last_activity": "2026-01-28T10:35:00Z"}
  ]
}
```

---

## Notes

- Projects are logical isolation, not process isolation (single server, multiple contexts)
- Memory per project: ~50MB (all buffers combined)
- Compatible with all other security features (read-only, allowlisting, profiles)
