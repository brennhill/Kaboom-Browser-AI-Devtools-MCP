---
status: proposed
scope: feature/environment-manipulation
ai-priority: medium
tags: [v7, environment-config, infrastructure]
relates-to: [product-spec.md, ../backend-control/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-environment-manipulation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Environment Manipulation — Technical Specification

## Architecture

### System Diagram
```
┌─────────────────────────────────────────────────────┐
│  Kaboom MCP Server (Go)                           │
│  ┌───────────────────────────────────────────────┐  │
│  │ Environment Router                            │  │
│  │ - Discover services & config sources          │  │
│  │ - Inspect environment variables               │  │
│  │ - Modify variables & config files             │  │
│  │ - Restart services with new config            │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Service Discovery                             │  │
│  │ - Detect running services (Node, Python, Go) │  │
│  │ - Find config files (.env, config.json, etc.)│  │
│  │ - List environment variables                  │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Config File Manager                           │  │
│  │ - Read .env, .env.local, JSON, YAML           │  │
│  │ - Parse key-value pairs                       │  │
│  │ - Write changes safely (backup first)         │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Secret Redaction                              │  │
│  │ - Detect sensitive keys (API_KEY, TOKEN, PWD)│  │
│  │ - Redact in logs and responses                │  │
│  │ - Never log plaintext secrets                 │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Environment Snapshot Manager                  │  │
│  │ - Create snapshots before changes             │  │
│  │ - Restore full environment to snapshot        │  │
│  │ - TTL cleanup (24h default)                   │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Service Restart Manager                       │  │
│  │ - Kill process gracefully                     │  │
│  │ - Restart with new environment                │  │
│  │ - Health check after restart                  │  │
│  │ - Timeout enforcement                         │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Audit Logger                                  │  │
│  │ - Log all environment changes                 │  │
│  │ - Track by correlation_id                     │  │
│  │ - Redact secrets in logs                      │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
          ↓
┌─────────────────────────────────────────────────────┐
│  Local Services (Node, Python, Go)                  │
│  - Read environment variables                       │
│  - Read config files                                │
│  - Receive SIGTERM/SIGKILL signals                  │
│  - Restart with new environment                     │
└─────────────────────────────────────────────────────┘
```

### Data Flow: Modify API Endpoint
```
1. AI calls: interact({action: "environment_modify", service: "frontend", changes: {API_ENDPOINT: "..."}})
2. Kaboom creates BEFORE snapshot: env-snap-20260131-101500
3. Kaboom reads current .env file
4. Kaboom updates API_ENDPOINT in memory
5. Kaboom writes updated .env file (backup created)
6. Kaboom sends SIGTERM to frontend service
7. Kaboom waits for graceful shutdown (<5s timeout)
8. Kaboom restarts service: npm run dev
9. Kaboom polls health check (/health endpoint)
10. After restart confirmed: create AFTER snapshot
11. AI calls: interact({action: "navigate", url: "..."})
12. Frontend uses new API_ENDPOINT from updated .env
```

## Implementation Plan

### Phase 1: Service Discovery & Inspection (Week 1)
1. **Service Discovery**
   - Detect running processes: ps aux filtering
   - Identify Node/Python/Go by process args
   - Extract service names and PIDs
   - List associated config files

2. **Environment Inspection**
   - Read process environment: /proc/<PID>/environ (Linux)
   - Read .env files from working directory
   - Parse environment variables
   - Detect and redact secrets

3. **Secret Redaction**
   - Patterns: API_KEY, TOKEN, PASSWORD, SECRET, URL (if contains @)
   - Replace with ***redacted*** in logs/responses
   - Still allow reads (but redacted)
   - Never log plaintext in audit logs

### Phase 2: Config Management (Week 2)
1. **Config File Reading**
   - Support .env format (KEY=VALUE)
   - Support JSON config files
   - Support YAML (subset)
   - Preserve comments and formatting

2. **Config File Writing**
   - Create backup before writing (.env.backup)
   - Update specific key-value pairs
   - Preserve formatting/comments
   - Generate git diff

3. **Validation**
   - Check file format before write
   - Validate syntax after write
   - Prevent orphaned quotes/braces
   - Rollback on validation failure

### Phase 3: Service Restart & Snapshot (Week 3)
1. **Service Restart**
   - Send SIGTERM (graceful shutdown)
   - Wait up to 5s for clean shutdown
   - If no shutdown: SIGKILL
   - Restart process with new environment
   - Verify restart via health check

2. **Environment Snapshot**
   - Capture all environment variables
   - Capture all config files (with redaction)
   - Store snapshot with timestamp
   - Support restore by snapshot_id

3. **Audit Logger**
   - Log all changes with timestamps
   - Include before/after values (redacted)
   - Track correlation_id
   - Replay-able via modification log

## API Changes

### New `interact()` mode: environment_inspect
```javascript
interact({
  action: "environment_inspect",
  service: "frontend|backend|all",
  filter: "API_*",  // Optional regex
  redact_secrets: true  // Default: true
})
→ {
    services: [
      {
        name: "frontend",
        pid: 12345,
        type: "node",
        variables: [
          {name: "API_ENDPOINT", value: "http://localhost:3001"},
          {name: "API_KEY", value: "***redacted***", is_set: true},
          {name: "DEBUG", value: "false"}
        ],
        config_files: [
          {path: ".env", exists: true, writable: true},
          {path: ".env.local", exists: false}
        ]
      }
    ]
  }
```

### New `interact()` mode: environment_modify
```javascript
interact({
  action: "environment_modify",
  service: "frontend",
  changes: {
    "API_ENDPOINT": "http://localhost:9999",
    "DEBUG": "true"
  },
  correlation_id: "test-001",
  permanent: false,  // false = session, true = persistent
  restart_service: true,
  timeout_ms: 10000
})
→ {
    status: "applied",
    changes: [
      {
        variable: "API_ENDPOINT",
        previous: "http://localhost:3001",
        current: "http://localhost:9999",
        restart_required: true
      }
    ],
    restart_status: {
      service: "frontend",
      status: "restarting",
      pid_old: 12345,
      pid_new: 12999,
      elapsed_ms: 2100
    }
  }
```

### New `interact()` mode: config_read
```javascript
interact({
  action: "config_read",
  service: "frontend",
  file: ".env"
})
→ {
    file: ".env",
    service: "frontend",
    format: "env",
    content: "API_ENDPOINT=http://localhost:3001\nDEBUG=false",
    lines: [
      {number: 1, content: "API_ENDPOINT=http://localhost:3001"},
      {number: 2, content: "DEBUG=false"}
    ]
  }
```

### New `interact()` mode: config_write
```javascript
interact({
  action: "config_write",
  service: "frontend",
  file: ".env",
  content: "API_ENDPOINT=http://localhost:9999\nDEBUG=true",
  correlation_id: "test-001",
  permanent: true,
  restart_service: true
})
→ {
    status: "success",
    file: ".env",
    lines_changed: 2,
    backup_created: ".env.20260131-101523.backup",
    git_staged: true
  }
```

### New `configure()` mode: environment snapshot
```javascript
configure({
  action: "snapshot",
  operation: "create",
  service: "frontend|all",
  name: "before_api_test"
})
→ {
    snapshot_id: "env-snap-20260131-101523",
    timestamp: "2026-01-31T10:15:23.456Z",
    services: 1,
    variables: 12
  }

configure({
  action: "snapshot",
  operation: "restore",
  snapshot_id: "env-snap-20260131-101523"
})
→ {
    status: "restored",
    services: ["frontend"],
    variables_restored: 12,
    services_restarted: ["frontend"],
    elapsed_ms: 3200
  }
```

### New `observe()` mode: environment_audit
```javascript
observe({
  what: "environment_audit",
  service: "frontend",
  correlation_id: "test-001"
})
→ {
    audit_log: [
      {
        timestamp: "2026-01-31T10:15:23.456Z",
        operation: "modify",
        service: "frontend",
        variable: "API_ENDPOINT",
        previous_value: "***redacted***",
        new_value: "***redacted***",
        restart_required: true
      }
    ]
  }
```

## Code References

### New files to create:
- `cmd/server/env/discovery.go` — Service and config discovery
- `cmd/server/env/inspector.go` — Environment variable inspection
- `cmd/server/env/modifier.go` — Environment modification
- `cmd/server/env/config.go` — Config file management (.env, JSON, YAML)
- `cmd/server/env/redaction.go` — Secret redaction logic
- `cmd/server/env/snapshot.go` — Environment snapshots
- `cmd/server/env/restart.go` — Service restart management
- `cmd/server/env/audit.go` — Audit logger

### Existing files to modify:
- `cmd/server/mcp/server.go` — Add environment_* action handlers
- `cmd/server/mcp/configure.go` — Add environment snapshot support
- `cmd/server/mcp/observe.go` — Add environment_audit mode

## Performance Requirements
- Service discovery: <100ms
- Environment inspection: <50ms
- Config file read: <50ms for files <1MB
- Config file write: <100ms
- Service restart: <5s (including graceful shutdown)
- Snapshot create/restore: <500ms
- Audit log query: <50ms

## Testing Strategy

### Unit Tests
1. Test secret redaction patterns
2. Test .env parsing/writing
3. Test JSON config parsing
4. Test YAML subset parsing
5. Test environment variable extraction

### Integration Tests
1. Start test services (Node, Python)
2. Discover running services
3. Inspect environment
4. Modify environment variables
5. Verify service restart
6. Verify new values take effect
7. Restore from snapshot
8. Verify original values restored

### E2E Tests
1. Full workflow: inspect → snapshot → modify → test → restore
2. Multiple services with dependent configs
3. Config file backups and rollback
4. Secret redaction in all outputs

## Dependencies
- Process management: ps, kill commands
- Service restart: npm/python/go start commands
- Health checks: curl or service-specific endpoints
- Git: for staging .env changes

## Security Considerations

1. **Secret Redaction**
   - Patterns: API_KEY, TOKEN, PASSWORD, SECRET, PRIVATE_KEY, JWT, AUTHORIZATION, AWS_, GITHUB_
   - Redact in logs, responses, diffs
   - Never log plaintext secrets
   - Redact in git diffs too

2. **File Permissions**
   - Verify write permissions before modifying
   - Create backups before write
   - Preserve original file permissions

3. **Process Safety**
   - Verify process is running before restart
   - Graceful shutdown with timeout
   - Health check after restart
   - Rollback on health check failure

4. **Audit Trail**
   - All changes logged with before/after (redacted)
   - Correlation IDs link to triggering observation
   - Cannot delete audit logs

## Configuration

Services can expose `/.kaboom/env` endpoint to customize:
- Health check endpoint
- Service restart command
- Supported config files
- Secrets patterns (additions to defaults)
- Shutdown timeout
