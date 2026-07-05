---
status: proposed
scope: feature/environment-manipulation
ai-priority: medium
tags: [v7, environment-config, hands, infrastructure]
relates-to: [../backend-control/product-spec.md, ../code-navigation-modification/product-spec.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-environment-manipulation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Environment Manipulation

## Overview
Environment Manipulation enables Kaboom to inspect and modify environment variables, configuration files, and runtime settings across frontend, backend, and infrastructure layers. When debugging issues related to configuration (API endpoints, feature flags, timeouts, auth tokens, database URLs), AI can now directly verify current settings and make targeted changes to test different behaviors without manual CLI operations.

## Problem
Configuration management is fragmented across multiple locations:
- `.env` and `.env.local` files (frontend)
- Environment variables in backend services (backend)
- Docker/Kubernetes configs (infrastructure)
- Feature flag services (Unleash, LaunchDarkly, etc.)
- Database connection strings (secrets)

Current workflow:
1. Developer manually edits `.env` file
2. Developer restarts dev server (if needed)
3. Test the change
4. Revert manually

This manual process prevents autonomous testing of configuration edge cases (wrong API endpoint, missing auth token, timeout too short, etc.).

## Solution
Environment Manipulation provides:
1. **Environment Discovery** — List all environment variables and config files accessible to services
2. **Environment Inspection** — Read current values (with redaction for secrets)
3. **Environment Mutation** — Temporarily modify environment variables for testing
4. **Config File Management** — Read/write configuration files (.env, JSON, YAML)
5. **Service Restart** — Safely restart services with new config
6. **Verification** — Confirm changes took effect

All operations are:
- **Safe** — Changes are scoped to current session; persisted changes require explicit opt-in
- **Auditable** — All environment changes logged with correlation IDs
- **Reversible** — Previous environment snapshots allow rollback
- **Secret-Aware** — Sensitive values redacted in logs and responses

## User Stories
- As an AI agent, I want to inspect API_ENDPOINT environment variable so that I can verify it points to the correct backend
- As a developer, I want to temporarily set DEBUG=true to enable logging so that I can diagnose an issue
- As an AI agent, I want to modify timeout values to test how the app handles slow backends
- As a QA engineer, I want to test with an invalid API key so that I can verify error handling
- As an AI agent, I want to verify changes took effect by querying the service's runtime config
- As an AI agent, I want to restore the original environment after testing so that subsequent tests start clean

## Acceptance Criteria
- [ ] Kaboom can list all environment variables for frontend and backend services
- [ ] Kaboom can read individual environment variable values (with secret redaction)
- [ ] Kaboom can modify environment variables and restart services
- [ ] Changes take effect within 2s after service restart
- [ ] Sensitive values (API keys, passwords, tokens) are never logged in plaintext
- [ ] Environment changes can be verified via service introspection
- [ ] Environment snapshots can be created and restored for deterministic testing
- [ ] Config files (.env, config.json, etc.) can be read and modified
- [ ] Performance: environment discovery <100ms, modification <500ms
- [ ] Support Node.js, Python, Go backends

## Not In Scope
- System-level environment variables (only app-level)
- Kubernetes secrets management (read-only for now)
- Configuration for non-standard formats
- Hot-reloading without service restart
- Multi-environment deployments (single dev environment)

## Data Structures

### Environment Discovery Result
```json
{
  "services": [
    {
      "name": "frontend",
      "type": "node",
      "config_sources": [
        {
          "type": "env_file",
          "path": ".env",
          "exists": true,
          "readable": true,
          "writable": true
        },
        {
          "type": "env_file",
          "path": ".env.local",
          "exists": true,
          "readable": true,
          "writable": true
        },
        {
          "type": "process_env",
          "readable": true,
          "writable": false
        }
      ],
      "variables": [
        {
          "name": "API_ENDPOINT",
          "value": "http://localhost:3001",
          "source": ".env",
          "type": "string"
        },
        {
          "name": "API_KEY",
          "value": "***redacted***",
          "source": ".env.local",
          "type": "secret",
          "is_set": true
        }
      ]
    }
  ]
}
```

### Environment Modification
```json
{
  "timestamp": "2026-01-31T10:15:23.456Z",
  "correlation_id": "test-config-001",
  "changes": [
    {
      "service": "frontend",
      "variable": "API_ENDPOINT",
      "previous_value": "http://localhost:3001",
      "new_value": "http://localhost:9999",
      "source": ".env",
      "applied": true,
      "restart_required": true
    }
  ],
  "restart_status": {
    "service": "frontend",
    "status": "restarting",
    "estimated_restart_time_ms": 2000
  }
}
```

### Environment Snapshot
```json
{
  "snapshot_id": "env-snap-20260131-101523",
  "timestamp": "2026-01-31T10:15:23.456Z",
  "services": {
    "frontend": {
      "API_ENDPOINT": "http://localhost:3001",
      "API_KEY": "***redacted***",
      "DEBUG": "false"
    },
    "backend": {
      "DATABASE_URL": "postgres://...",
      "LOG_LEVEL": "info"
    }
  }
}
```

## Examples

### Example 1: Test with Wrong API Endpoint
```javascript
// AI wants to test error handling when API is unreachable
// Step 1: Discover current config
const config = await interact({
  action: "environment_inspect",
  service: "frontend"
});
// Returns: {API_ENDPOINT: "http://localhost:3001", DEBUG: "false", ...}

// Step 2: Create snapshot for rollback
await configure({
  action: "snapshot",
  operation: "create",
  service: "frontend",
  name: "before_api_test"
});

// Step 3: Modify API endpoint to unreachable address
await interact({
  action: "environment_modify",
  service: "frontend",
  changes: {
    "API_ENDPOINT": "http://localhost:9999"  // Non-existent server
  },
  correlation_id: "test-api-error-handling"
});
// Service automatically restarts, new endpoint takes effect

// Step 4: Test frontend error handling
await interact({action: "navigate", url: "http://localhost:3000"});
// User sees error message for API unavailable

// Step 5: Restore original config
await configure({
  action: "snapshot",
  operation: "restore",
  snapshot_id: "before_api_test"
});
```

### Example 2: Debug Mode with Verbose Logging
```javascript
// AI wants to enable debug logging to diagnose intermittent issue
await interact({
  action: "environment_modify",
  service: "backend",
  changes: {
    "LOG_LEVEL": "debug",
    "DEBUG": "true"
  },
  correlation_id: "debug-session-001"
});

// Now backend logs include detailed debug output
const logs = await observe({
  what: "backend-logs",
  correlation_id: "debug-session-001"
});
// Should see [DEBUG] messages that weren't there before
```

### Example 3: Timeout Testing
```javascript
// AI wants to test what happens with short timeout
await interact({
  action: "environment_modify",
  service: "frontend",
  changes: {
    "REQUEST_TIMEOUT_MS": "500"  // Very short timeout
  },
  correlation_id: "test-timeout-handling"
});

// Frontend makes slow request
await interact({
  action: "navigate",
  url: "http://localhost:3000/slow-endpoint"
});

// Observe how frontend handles timeout
const response = await observe({
  what: "network_waterfall",
  correlation_id: "test-timeout-handling"
});
// Should see request abort or error after 500ms
```

## MCP Tool Changes

### New `interact()` mode: environment_inspect
```javascript
interact({
  action: "environment_inspect",
  service: "frontend",  // or "backend", or omit for all
  filter: "API_*"  // Optional: pattern to filter variables
})
→ {
    service: "frontend",
    variables: [
      {name: "API_ENDPOINT", value: "http://localhost:3001"},
      {name: "API_KEY", value: "***redacted***", is_set: true},
      {name: "DEBUG", value: "false"}
    ],
    config_files: [
      {path: ".env", exists: true, writable: true},
      {path: ".env.local", exists: true, writable: true}
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
  permanent: false,  // false = session only, true = persist to .env
  restart_service: true  // auto-restart to apply changes
})
→ {
    status: "applied",
    changes: [
      {variable: "API_ENDPOINT", status: "changed", restart_required: true},
      {variable: "DEBUG", status: "changed", restart_required: false}
    ],
    restart_status: {service: "frontend", status: "restarting", eta_ms: 2000}
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
  correlation_id: "test-001"
})
→ {
    status: "success",
    file: ".env",
    lines_changed: 2,
    git_staged: true
  }
```

### New `configure()` mode: environment snapshot
```javascript
configure({
  action: "snapshot",
  operation: "create",
  service: "frontend",
  name: "before_api_test"
})
→ {
    snapshot_id: "env-snap-20260131-101523",
    variables_captured: 12
  }

configure({
  action: "snapshot",
  operation: "restore",
  snapshot_id: "env-snap-20260131-101523"
})
→ {
    status: "restored",
    variables_restored: 12,
    services_restarted: ["frontend"]
  }
```
