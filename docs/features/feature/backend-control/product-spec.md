---
status: proposed
scope: feature/backend-control
ai-priority: high
tags: [v7, backend-integration, control, hands]
relates-to: [../backend-log-streaming/product-spec.md, ../git-event-tracking/product-spec.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-backend-control
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Control

## Overview
Backend Control enables AI to safely inspect, modify, and control backend services through a standardized gRPC/HTTP API exposed by Kaboom-compatible backend services. This feature allows developers to inject test data, simulate failures, modify environment variables, reset state, and verify backend behavior without manual CLI interaction. By integrating with the backend control plane, Kaboom becomes a true full-stack debugging and testing tool.

## Problem
Currently, Kaboom excels at frontend and network-level debugging, but backend state remains a black box. Developers must:
- Manually SSH into servers or use CLI tools to reset database state
- Use separate admin dashboards or CLIs to inject test data
- Write scripts to simulate backend failures (circuit breaker errors, timeouts, rate limits)
- Manually verify backend state after each fix attempt
- Coordinate between frontend testing and backend setup, increasing test complexity

This friction prevents AI from autonomously validating full-stack behavior end-to-end.

## Solution
Backend Control establishes a bidirectional API between Kaboom MCP and backend services. Backend services expose:
1. **State Management** — Reset database, clear caches, reset feature flags to known states
2. **Data Injection** — Create test users, products, payments, any domain objects
3. **Failure Simulation** — Trigger circuit breakers, timeouts, rate limits, validation errors
4. **Configuration Mutation** — Modify environment variables, feature flags, feature toggles
5. **Introspection** — Query backend state without breaking encapsulation

All operations are:
- **Safe** — Operations are read-only by default; write operations require explicit opt-in
- **Auditable** — All changes logged with correlation IDs
- **Reversible** — State snapshots allow rollback
- **Scoped** — Per-service, per-environment permission model

## User Stories
- As an AI agent, I want to reset the user database to a known state before each test so that tests are deterministic and don't interfere with each other
- As an AI agent, I want to create test payment records with specific statuses (pending, failed, refunded) so that I can verify error handling paths
- As a QA engineer, I want to simulate a payment gateway timeout so that Kaboom can capture how the frontend handles this failure gracefully
- As an AI agent, I want to toggle feature flags on/off to verify A/B test logic is correct
- As a developer, I want to manually inspect backend state (user count, cache hit rate) to verify that state management is working correctly
- As an AI agent, I want to restore a previous state snapshot after a test fails so that I can retry the test with clean state

## Acceptance Criteria
- [ ] Backend services can expose a `/.kaboom/control` gRPC endpoint with state management APIs
- [ ] Kaboom MCP can discover available control operations via introspection
- [ ] State mutations are audited with timestamp, user, correlation ID, and operation details
- [ ] State snapshots can be created and restored for deterministic test retries
- [ ] Read operations (inspect state) complete in <50ms, write operations in <200ms
- [ ] All write operations are gated behind explicit `shadow_mode: true` or `dry_run: false` flags
- [ ] Each operation logs results as backend events for correlation with frontend events
- [ ] State operations survive service restarts (journaled)
- [ ] MCP tool `interact()` and new `configure()` modes enable backend control

## Not In Scope
- Long-lived transactions or complex multi-step orchestrations
- Auto-repair of backend state (only reset/snapshot, no ML inference)
- Direct database access (only service APIs)
- Multi-tenant state isolation (Kaboom is single-tenant)
- Backwards compatibility with non-Kaboom-aware backends

## Data Structures

### Backend Control Operations
```json
{
  "operations": [
    {
      "name": "reset_database",
      "category": "state_management",
      "safe": true,
      "params": {
        "tables": ["users", "products"],
        "exclude_seed_data": true
      },
      "returns": {"rows_deleted": 12500, "duration_ms": 350}
    },
    {
      "name": "create_test_user",
      "category": "data_injection",
      "safe": false,
      "params": {
        "email": "string",
        "plan": "free|pro|enterprise",
        "credits": 0
      },
      "returns": {"user_id": 12345, "email": "string"}
    },
    {
      "name": "simulate_payment_timeout",
      "category": "failure_simulation",
      "safe": false,
      "params": {
        "duration_ms": 5000,
        "error_code": "TIMEOUT"
      },
      "returns": {"simulation_active": true, "duration_ms": 5000}
    },
    {
      "name": "set_feature_flag",
      "category": "configuration",
      "safe": false,
      "params": {
        "flag_name": "use_new_checkout",
        "value": true,
        "percentage": 100
      },
      "returns": {"flag_name": "use_new_checkout", "active": true}
    }
  ]
}
```

### State Snapshot
```json
{
  "snapshot_id": "snap-20260131-101523",
  "timestamp": "2026-01-31T10:15:23.456Z",
  "service": "api-server",
  "changes": [
    {
      "table": "users",
      "count": 150,
      "hash": "abc123def456"
    },
    {
      "key": "cache:sessions",
      "count": 45,
      "hash": "xyz789"
    }
  ],
  "restored_from": null,
  "correlation_id": "test-run-12345"
}
```

### Control Audit Log
```json
{
  "audit_log": [
    {
      "timestamp": "2026-01-31T10:15:23.456Z",
      "operation": "reset_database",
      "service": "api-server",
      "correlation_id": "test-run-12345",
      "user": "ai-agent",
      "params": {"tables": ["users"]},
      "result": "success",
      "duration_ms": 350,
      "changes_made": {"users": {"rows_deleted": 1250}}
    }
  ]
}
```

## Examples

### Example 1: Reset State Before Test
```javascript
// AI agent prepares clean state
await configure({
  action: "snapshot",
  operation: "create",
  service: "api-server",
  name: "before_test"
});

await interact({
  action: "backend_control",
  operation: "reset_database",
  tables: ["users", "orders"],
  exclude_seed_data: true,
  correlation_id: "test-checkout-flow-001"
});
```

**Result:** Database reset, audit logged, next test starts with clean state.

### Example 2: Inject Test User & Trigger Failure
```javascript
// Create test user
const user = await interact({
  action: "backend_control",
  operation: "create_test_user",
  email: "test@example.com",
  plan: "pro",
  credits: 100,
  correlation_id: "test-checkout-flow-001"
});

// Simulate payment timeout
await interact({
  action: "backend_control",
  operation: "simulate_payment_timeout",
  duration_ms: 3000,
  error_code: "TIMEOUT",
  correlation_id: "test-checkout-flow-001"
});

// User clicks checkout, backend times out
await interact({
  action: "execute_js",
  script: "document.querySelector('button.checkout').click()"
});

// AI observes frontend + backend correlation
const logs = await observe({
  what: "backend-logs",
  correlation_id: "test-checkout-flow-001"
});
```

**Result:** Frontend error message appears, backend logs show timeout, correlation visible in UI.

### Example 3: Restore State After Failure
```javascript
// Test failed unexpectedly
await configure({
  action: "snapshot",
  operation: "restore",
  snapshot_id: "before_test",
  service: "api-server"
});

// Re-run test with fresh state
await interact({
  action: "navigate",
  url: "https://localhost:3000"
});
```

**Result:** Backend state restored, test retried deterministically.

## MCP Tool Changes

### New `interact()` mode:
```javascript
interact({
  action: "backend_control",
  operation: "reset_database|create_test_user|simulate_*|set_feature_flag|inspect_state",
  service: "api-server",
  params: {...},
  shadow_mode: false,  // false = actually execute, true = dry-run
  correlation_id: "test-id"
})
```

### New `configure()` modes:
```javascript
configure({
  action: "snapshot",
  session_action: "create|restore|list|delete",
  service: "api-server",
  name: "before_payment_test"
})
```

### New `observe()` mode:
```javascript
observe({
  what: "backend_state",
  service: "api-server",
  query: "SELECT COUNT(*) FROM users"  // or simple key paths
})
```
