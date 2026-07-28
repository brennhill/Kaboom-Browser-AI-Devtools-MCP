---
status: proposed
scope: feature/environment-manipulation
ai-priority: medium
tags: [v7, testing, environment-config]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-environment-manipulation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Environment Manipulation — QA Plan

## Test Scenarios

### Scenario 1: Discover Services & Config Files
**Objective:** Verify service discovery finds running services and their configs

#### Setup:
- Frontend service running (Node.js): npm run dev (PID 12345)
- Backend service running (Python): python -m flask run (PID 12346)
- .env file in frontend directory
- .env.local file in backend directory

#### Steps:
1. Call `interact({what: "environment_inspect", service: "all"})`
2. Verify both services discovered
3. Verify PIDs correct
4. Verify config files listed for each service
5. Verify service types detected (node, python)

#### Expected Result:
- Both services in response
- Config files list accurate
- PIDs match running processes

#### Acceptance Criteria:
- [ ] Services discovered <100ms
- [ ] All running services found
- [ ] Config file paths correct
- [ ] Service types correct

---

### Scenario 2: Inspect Environment Variables (with Secret Redaction)
**Objective:** Verify environment inspection and secret redaction

#### Setup:
- Frontend service with variables:
  - API_ENDPOINT=<http://localhost:3001>
  - API_KEY=sk_test_12345abc (secret)
  - DEBUG=false

#### Steps:
1. Call `interact({what: "environment_inspect", service: "frontend"})`
2. Verify API_ENDPOINT returned as plaintext
3. Verify API_KEY redacted: ***redacted***
4. Verify is_set: true for API_KEY (so we know it exists)
5. Verify DEBUG returned as plaintext
6. Verify all three variables present

#### Expected Result:
- Non-secrets visible
- Secrets redacted but marked as set
- Complete variable list returned
- No plaintext secrets in output

#### Acceptance Criteria:
- [ ] Secret redaction works
- [ ] is_set flag accurate
- [ ] Non-secrets not affected
- [ ] All variables listed

---

### Scenario 3: Modify Single Environment Variable
**Objective:** Verify single environment variable can be modified

#### Setup:
- Frontend running with API_ENDPOINT=<http://localhost:3001>
- No modifications yet

#### Steps:
1. Create BEFORE snapshot
2. Call `interact({what: "environment_modify", service: "frontend", changes: {API_ENDPOINT: "http://localhost:9999"}, restart_service: true})`
3. Verify response shows change
4. Verify service restarted (PID changed)
5. Verify frontend still running (health check)
6. Read updated .env file: verify change persisted

#### Expected Result:
- Change applied successfully
- Service restarted with new value
- Restart completed in <5s
- New value takes effect

#### Acceptance Criteria:
- [ ] Variable modified
- [ ] Service restarted
- [ ] New value active
- [ ] Service healthy after restart

---

### Scenario 4: Modify Multiple Variables at Once
**Objective:** Verify multiple variables can be changed together

#### Setup:
- Backend service with:
  - LOG_LEVEL=info
  - DATABASE_POOL_SIZE=10
  - TIMEOUT_MS=5000

#### Steps:
1. Call `interact({what: "environment_modify", service: "backend", changes: {LOG_LEVEL: "debug", TIMEOUT_MS: "1000"}})`
2. Verify both changes applied
3. Verify service restarted
4. Verify both new values active
5. Verify unchanged variable (DATABASE_POOL_SIZE) unaffected

#### Expected Result:
- Multiple changes applied atomically
- Service restarted once
- All new values active
- No side effects on other variables

#### Acceptance Criteria:
- [ ] All changes applied
- [ ] Single restart (not one per change)
- [ ] Only target variables changed
- [ ] Service stable

---

### Scenario 5: Read .env Config File
**Objective:** Verify .env file can be read and parsed

#### Setup:
- Frontend .env file with standard format:
```
API_ENDPOINT=http://localhost:3001
API_KEY=sk_test_12345
DEBUG=false
# Comment line
FEATURE_FLAGS=new_checkout:true,beta:false
```

## Steps:
1. Call `interact({what: "config_read", service: "frontend", file: ".env"})`
2. Verify all lines returned
3. Verify line numbers correct
4. Verify comments included
5. Verify multi-value entries preserved

## Expected Result:
- Full file content returned
- Line numbers accurate
- Comments preserved
- Complex values (flags) not corrupted

## Acceptance Criteria:
- [ ] Complete file read
- [ ] Line numbers correct
- [ ] Comments preserved
- [ ] No truncation

---

### Scenario 6: Write .env Config File with Backup
**Objective:** Verify .env can be written with backup created

#### Setup:
- Original .env exists with 3 variables
- Want to change 1 variable

#### Steps:
1. Call `interact({what: "config_write", service: "frontend", file: ".env", content: "API_ENDPOINT=http://localhost:9999\nAPI_KEY=sk_test_12345\nDEBUG=true"})`
2. Verify response: lines_changed: 2
3. Verify response: backup_created
4. Check filesystem: .env.TIMESTAMP.backup exists
5. Verify backup contains original content
6. Verify .env has new content
7. Verify git_staged: true

#### Expected Result:
- Changes written successfully
- Backup created automatically
- Original recoverable
- Changes staged to git

#### Acceptance Criteria:
- [ ] File written correctly
- [ ] Backup created
- [ ] Backup contains original
- [ ] git add executed
- [ ] Service restarted if needed

---

### Scenario 7: Environment Snapshot & Restore
**Objective:** Verify environment snapshots can be created and restored

#### Setup:
- Environment in known state (from Scenario 3+4 changes)

#### Steps:
1. Create snapshot: `configure({what: "snapshot", operation: "create", service: "frontend", name: "test_snapshot"})`
2. Verify response: snapshot_id returned
3. Modify environment significantly
4. Verify environment changed
5. Call restore: `configure({what: "snapshot", operation: "restore", snapshot_id: "..."})`
6. Verify environment restored to snapshot state
7. Verify service restarted
8. Verify API_ENDPOINT back to original value

#### Expected Result:
- Snapshot created successfully
- Can restore to exact previous state
- Service restarted with original values
- Full environment restoration

#### Acceptance Criteria:
- [ ] Snapshot creation successful
- [ ] Snapshot restored completely
- [ ] All variables restored
- [ ] Service healthy after restore

---

### Scenario 8: Error Handling: Invalid API Endpoint
**Objective:** Verify error handling when service becomes unreachable

#### Setup:
- Frontend running normally

#### Steps:
1. Create BEFORE snapshot
2. Modify to invalid API endpoint: `interact({what: "environment_modify", service: "frontend", changes: {API_ENDPOINT: "http://localhost:9999"}, restart_service: true})`
3. Frontend service restarts but can't connect to backend
4. Call `interact({what: "navigate", url: "http://localhost:3000"})`
5. Observe error: no response from backend
6. AI automatically offers rollback
7. Restore BEFORE snapshot
8. Verify connectivity restored

#### Expected Result:
- Environment change applied
- Frontend reflects error state
- Rollback available
- Rollback successful

#### Acceptance Criteria:
- [ ] Error state observable
- [ ] Rollback offered
- [ ] Rollback successful
- [ ] Service recovers

---

### Scenario 9: Service Restart with Health Check
**Objective:** Verify service restart and health verification

#### Setup:
- Frontend service with health endpoint: GET /health → {status: "ok"}

#### Steps:
1. Record original PID
2. Modify environment with restart_service: true
3. Verify restart detected (PID changed)
4. Verify health check called
5. Verify health check returns {status: "ok"}
6. Verify response confirms service healthy
7. Verify restart_status.status: "restarted"

#### Expected Result:
- Service restarts
- PID changes
- Health check confirms ready
- Response shows success

#### Acceptance Criteria:
- [ ] PID changes
- [ ] Health check passes
- [ ] Restart completes in <5s
- [ ] No service downtime >2s

---

### Scenario 10: Audit Log Captures All Changes
**Objective:** Verify all environment changes are audited

#### Setup:
- Perform multiple environment modifications with same correlation_id

#### Steps:
1. Modify API_ENDPOINT with correlation_id: "env-audit-test"
2. Modify DEBUG with same correlation_id
3. Call `observe({what: "environment_audit", correlation_id: "env-audit-test"})`
4. Verify both modifications in audit log
5. Verify timestamps in order
6. Verify before/after values (redacted for secrets)
7. Verify restart_required flag

#### Expected Result:
- All changes logged
- Chronological order
- Before/after captured
- Secrets redacted in log

#### Acceptance Criteria:
- [ ] All changes audited
- [ ] Chronological order
- [ ] Secrets redacted
- [ ] Correlation ID links changes

---

### Scenario 11: Multiple Services Different Configs
**Objective:** Verify modifications work across multiple services

#### Setup:
- Frontend and Backend running
- Frontend has .env
- Backend has config.json

#### Steps:
1. Modify frontend .env: API_ENDPOINT
2. Modify backend config.json: DATABASE_URL
3. Verify both services restart
4. Verify both changes effective
5. Call inspect: frontend + backend
6. Verify all changes visible

#### Expected Result:
- Both services modified independently
- Each restarts correctly
- Changes isolated per service
- No cross-service side effects

#### Acceptance Criteria:
- [ ] Multi-service modification works
- [ ] Services restart independently
- [ ] Changes isolated
- [ ] No interference

---

### Scenario 12: Secret Redaction in All Outputs
**Objective:** Verify secrets never appear in plaintext

#### Setup:
- Environment with multiple secrets:
  - API_KEY=sk_test_12345
  - GITHUB_TOKEN=ghp_abcdef123456
  - DB_PASSWORD=super_secret_password

#### Steps:
1. Call environment_inspect: verify redacted
2. Modify a secret
3. Check git diff: verify redacted
4. Call observe audit_log: verify redacted
5. Check log files: verify no plaintext

#### Expected Result:
- All outputs show ***redacted***
- But is_set: true indicates variable exists
- No plaintext secrets anywhere
- Audit trail secure

#### Acceptance Criteria:
- [ ] No plaintext secrets in responses
- [ ] No plaintext in git diffs
- [ ] No plaintext in logs
- [ ] Audit trail secure

---

### Scenario 13: Permanent vs Session Changes
**Objective:** Verify permanent changes persist across restarts

#### Setup:
- Frontend running
- Want to test both session and permanent changes

#### Steps:
1. Call modify with permanent: false, changes: {API_ENDPOINT: "..."}
2. Restart entire development environment
3. Verify change did NOT persist (temp change only)
4. Call modify with permanent: true, changes: {API_ENDPOINT: "..."}
5. Check .env file: change written
6. Restart development environment
7. Verify change persisted (permanent)

#### Expected Result:
- Session changes don't persist
- Permanent changes written to file
- Permanent changes survive restart

#### Acceptance Criteria:
- [ ] Session changes ephemeral
- [ ] Permanent changes written
- [ ] Permanent changes survive restart

---

### Scenario 14: Config File Format Preservation
**Objective:** Verify formatting and comments preserved during edits

#### Setup:
- Original .env with formatting:
```
# API Configuration
API_ENDPOINT=http://localhost:3001
API_KEY=sk_test

# Debug Settings
DEBUG=false

# Feature Flags
FEATURES=new_checkout:true,beta:false
```

## Steps:
1. Read file: verify formatting
2. Modify single value: API_ENDPOINT
3. Write file: verify only that value changed
4. Read file: verify comments still present
5. Verify blank lines preserved
6. Verify section structure intact

## Expected Result:
- Formatting preserved
- Comments preserved
- Only target value changed
- File structure intact

## Acceptance Criteria:
- [ ] Comments preserved
- [ ] Formatting intact
- [ ] Only target changed
- [ ] No reformatting

---

## Acceptance Criteria (Overall)
- [ ] All scenarios pass on Linux, macOS, Windows
- [ ] Service discovery <100ms
- [ ] Environment inspection <50ms
- [ ] Config read/write <100ms
- [ ] Service restart <5s
- [ ] Snapshots create/restore <500ms
- [ ] Secrets always redacted
- [ ] Audit trail complete
- [ ] All changes reversible
- [ ] No data loss

## Test Data

### .env Sample
```
API_ENDPOINT=http://localhost:3001
API_KEY=sk_test_abcdef123456
DATABASE_URL=postgres://user:pass@localhost:5432/testdb
LOG_LEVEL=info
DEBUG=false
TIMEOUT_MS=5000
FEATURE_FLAGS=new_checkout:true,beta:false
```

### config.json Sample
```json
{
  "api": {
    "endpoint": "http://localhost:3001",
    "key": "sk_test_abcdef123456",
    "timeout": 5000
  },
  "database": {
    "url": "postgres://user:pass@localhost:5432/testdb"
  }
}
```

## Regression Tests

**Critical:** After each change, verify:
1. Service discovery doesn't hang
2. No infinite restart loops
3. Backups never overwrite each other
4. Secrets in backups also redacted
5. No environment variable pollution
6. Health checks accurate
7. Snapshots don't consume excessive disk
8. Audit logs never lose entries
9. Config file format preserved
10. Git staging works correctly

## Performance Baseline

| Operation | Target | Measured | Status |
|-----------|--------|----------|--------|
| discover_services | <100ms | _ | _ |
| inspect_environment | <50ms | _ | _ |
| read_config | <50ms | _ | _ |
| write_config | <100ms | _ | _ |
| service_restart | <5s | _ | _ |
| snapshot_create | <500ms | _ | _ |
| snapshot_restore | <500ms | _ | _ |

## Known Limitations

- [ ] No Kubernetes secrets integration (read-only access only)
- [ ] No automatic environment variable validation
- [ ] No hot-reload without restart
- [ ] Config format support limited (env, JSON, YAML subset)
- [ ] No multi-machine environment sync
- [ ] Health checks basic (GET /health only)
