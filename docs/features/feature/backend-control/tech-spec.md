---
status: proposed
scope: feature/backend-control
ai-priority: high
tags: [v7, backend-integration, control]
relates-to: [product-spec.md, ../backend-log-streaming/tech-spec.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-backend-control
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Control — Technical Specification

## Architecture

### System Diagram
```
┌─────────────────────────────────────────────────────┐
│  Kaboom MCP Server (Go)                           │
│  ┌───────────────────────────────────────────────┐  │
│  │ Control Command Router                        │  │
│  │ - Parse operation requests                    │  │
│  │ - Validate permissions (shadow_mode, etc.)    │  │
│  │ - Execute or dry-run                          │  │
│  │ - Audit & journal all operations              │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ State Snapshot Manager                        │  │
│  │ - Create before/after snapshots               │  │
│  │ - Restore to previous state                   │  │
│  │ - TTL cleanup (24h default)                   │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Audit & Correlation Logger                    │  │
│  │ - Log all operations with timestamps          │  │
│  │ - Index by correlation_id                     │  │
│  │ - Expose for observe({what: 'audit_log'})     │  │
│  └───────────────────────────────────────────────┘  │
│  ↓                                                   │
│  ┌───────────────────────────────────────────────┐  │
│  │ Service Discovery & Health Check              │  │
│  │ - Discover /.kaboom/control endpoints       │  │
│  │ - Monitor service availability                │  │
│  │ - Fail-fast on unreachable services           │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
          ↓ gRPC or HTTP
┌─────────────────────────────────────────────────────┐
│ Backend Service (Node.js/Python/Go/etc.)           │
│ ┌───────────────────────────────────────────────┐  │
│ │ /.kaboom/control Handler                    │  │
│ │ - Operations: reset_db, create_user, etc.     │  │
│ │ - Journaled transactions                      │  │
│ │ - Returns operation results & checksums       │  │
│ └───────────────────────────────────────────────┘  │
│ ↓                                                   │
│ ┌───────────────────────────────────────────────┐  │
│ │ Backend State (Database + Cache + Config)     │  │
│ │ - Snapshot checksum for verification          │  │
│ │ - Journal log for rollback                    │  │
│ └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

### Data Flow: Reset Database
```
1. AI calls: interact({what: "backend_control", operation: "reset_database"})
2. Kaboom MCP creates BEFORE snapshot: snap-20260131-101500
3. Kaboom MCP sends HTTP POST /.kaboom/control/reset_database
4. Backend service resets DB, returns {rows_deleted: 1250, duration_ms: 350}
5. Kaboom MCP creates AFTER snapshot: snap-20260131-101503
6. Kaboom MCP logs operation with correlation_id, timestamps, before/after hashes
7. AI observes backend-logs and sees "DATABASE_RESET" event
8. Next test runs on clean state
```

## Implementation Plan

### Phase 1: Core Control Infrastructure (Week 1)
1. **Define Control Protocol** — Standardize /.kaboom/control endpoint across Go/Node.js/Python
   - Request format: `{operation, params, correlation_id, request_id}`
   - Response format: `{status, result, duration_ms, checksum}`
   - Error handling: descriptive error codes, rollback support

2. **Implement Control Router in MCP**
   - Route `interact({what: "backend_control", ...})` to service discovery + gRPC/HTTP client
   - Validate `shadow_mode` and `dry_run` flags
   - Add retry logic with exponential backoff

3. **Snapshot Manager**
   - Implement create/restore/list/delete snapshots
   - Store in `.kaboom/snapshots/` with compression
   - Track parent snapshots for restore chains

4. **Audit Logger**
   - Log all operations to `.kaboom/audit.jsonl`
   - Index by correlation_id for fast lookup
   - Expose via `observe({what: 'audit_log'})`

### Phase 2: Service Integration (Week 2)
1. **Service Discovery**
   - Detect /.kaboom/control endpoint via OPTIONS request
   - Cache discovered services with TTL
   - Health check every 30s

2. **Operation Discovery**
   - Fetch available operations from each service
   - Parse operation schema (params, return types)
   - Display in `observe({what: 'backend_state', service: 'api-server'})`

3. **Permission Gating**
   - Implement `shadow_mode` (read-only, returns predicted result)
   - Implement `dry_run` (logs operation, doesn't commit)
   - Add `require_confirmation` for destructive ops

### Phase 3: Correlation & Integration (Week 3)
1. **Correlation ID Propagation**
   - Generate unique correlation_id for each test
   - Pass through to backend operations
   - Backend logs include correlation_id
   - Frontend captures correlation_id in session

2. **Full-Stack Tracing**
   - `observe()` can filter by correlation_id across:
     - Frontend events
     - Backend logs
     - Control operations
   - Timeline visualization

3. **Error Handling & Rollback**
   - If operation fails, automatically offer rollback to before-snapshot
   - Log rollback as separate operation
   - Track rollback success/failure

## API Changes

### New `interact()` modes:
```javascript
// Reset database
interact({
  action: "backend_control",
  operation: "reset_database",
  service: "api-server",
  params: {
    tables: ["users", "orders"],
    exclude_seed_data: true
  },
  correlation_id: "test-001",
  shadow_mode: false,
  dry_run: false
})
→ {
    status: "success",
    result: {rows_deleted: 1250, duration_ms: 350},
    checksum: "abc123",
    snapshot_id: "snap-after-001"
  }

// Inject test data
interact({
  action: "backend_control",
  operation: "create_test_user",
  service: "api-server",
  params: {
    email: "test@example.com",
    plan: "pro",
    credits: 100
  }
})
→ {
    status: "success",
    result: {user_id: 12345, email: "test@example.com"},
    checksum: "def456"
  }

// Simulate failure
interact({
  action: "backend_control",
  operation: "simulate_payment_timeout",
  service: "payment-processor",
  params: {
    duration_ms: 3000,
    error_code: "TIMEOUT"
  }
})
→ {
    status: "success",
    result: {simulation_active: true, expires_at: "2026-01-31T10:20:23.456Z"}
  }
```

### New `configure()` modes:
```javascript
// Snapshot management
configure({
  action: "snapshot",
  session_action: "create",
  name: "before_payment_test",
  service: "api-server"
})
→ {
    snapshot_id: "snap-20260131-101523",
    timestamp: "2026-01-31T10:15:23.456Z",
    checksum: "abc123"
  }

configure({
  action: "snapshot",
  session_action: "restore",
  snapshot_id: "snap-20260131-101523",
  service: "api-server"
})
→ {
    status: "restored",
    rows_recovered: 1250
  }

configure({
  action: "snapshot",
  session_action: "list",
  service: "api-server"
})
→ {
    snapshots: [
      {snapshot_id: "snap-20260131-101523", timestamp: "...", size_mb: 12}
    ]
  }
```

### New `observe()` modes:
```javascript
// Query backend state
observe({
  what: "backend_state",
  service: "api-server",
  query_type: "key",
  keys: ["user_count", "cache:hit_rate", "feature_flags.use_new_checkout"]
})
→ {
    state: {
      user_count: 1250,
      "cache:hit_rate": 0.85,
      "feature_flags.use_new_checkout": true
    }
  }

// Audit log
observe({
  what: "audit_log",
  correlation_id: "test-001",
  limit: 50
})
→ {
    audit_log: [
      {
        timestamp: "2026-01-31T10:15:23.456Z",
        operation: "reset_database",
        result: "success",
        changes_made: {users: {rows_deleted: 1250}}
      }
    ]
  }
```

## Code References

### New files to create:
- `cmd/server/control/router.go` — Control command router
- `cmd/server/control/snapshot.go` — Snapshot manager
- `cmd/server/control/audit.go` — Audit logger
- `cmd/server/control/discovery.go` — Service discovery
- `cmd/server/control/protocol.go` — Control protocol types
- `extension/src/mcp-tools/backend-control.ts` — Chrome extension MCP tool

### Existing files to modify:
- `cmd/server/mcp/server.go` — Add control action handler to interact()
- `cmd/server/mcp/observe.go` — Add audit_log and backend_state modes
- `cmd/server/mcp/configure.go` — Add snapshot action

## Performance Requirements
- Control operation execution: <50ms for reads, <200ms for writes
- Service discovery: <100ms (cached, 30s TTL)
- Snapshot creation: <500ms for typical DB state
- Snapshot restore: <1s for typical DB state
- Audit log query: <50ms for <10K entries

## Testing Strategy

### Unit Tests
1. Test snapshot creation/restore logic
2. Test audit log formatting and indexing
3. Test operation validation (shadow_mode, dry_run)
4. Test error handling and rollback

### Integration Tests
1. Start test backend with /.kaboom/control endpoint
2. Execute reset_database, verify DB state
3. Execute create_test_user, verify user exists
4. Simulate failure, verify error path
5. Restore snapshot, verify original state
6. Query audit log, verify all operations logged

### E2E Tests
1. Run full test flow: reset → inject data → navigate → verify → rollback
2. Verify correlation IDs flow through all layers
3. Verify snapshots survive service restart

## Dependencies
- Backend Log Streaming (for audit log correlation)
- Git Event Tracking (for change attribution)
- Request/Session Correlation (for trace context propagation)

## Backend Service Implementation Requirements

Services must implement `GET|POST /.kaboom/control` handler:

```go
// GET /.kaboom/control — Discover available operations
type ControlOperation struct {
  Name      string
  Category  string // "state_management", "data_injection", etc.
  Safe      bool
  Params    map[string]interface{}
  Returns   map[string]interface{}
  Examples  []string
}

// POST /.kaboom/control — Execute operation
type ControlRequest struct {
  Operation   string
  Params      map[string]interface{}
  CorrelationID string
  ShadowMode  bool
  DryRun      bool
}

type ControlResponse struct {
  Status      string // "success", "error"
  Result      map[string]interface{}
  Checksum    string // SHA256 of state after operation
  Duration    int    // milliseconds
  Error       string // if status == "error"
}
```
