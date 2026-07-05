---
status: proposed
scope: feature/backend-control
ai-priority: high
tags: [v7, testing, backend-integration]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-backend-control
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Control — QA Plan

## Test Scenarios

### Scenario 1: Reset Database & Verify Clean State
**Objective:** Verify reset_database operation succeeds and leaves database in expected state

#### Setup:
- Start backend service with /.kaboom/control handler
- Pre-populate database with 1250 users, 500 orders
- Kaboom MCP connected and discovered service

#### Steps:
1. Call `interact({action: "backend_control", operation: "reset_database", tables: ["users", "orders"]})`
2. Verify Kaboom creates BEFORE snapshot
3. Verify Kaboom creates AFTER snapshot
4. Query backend state: `observe({what: "backend_state", service: "api-server", keys: ["user_count"]})`
5. Verify user_count is 0 (or seed data only)

#### Expected Result:
- Operation returns `{status: "success", result: {rows_deleted: 1250}, duration_ms: 350}`
- BEFORE snapshot checksum != AFTER snapshot checksum
- Backend state reflected in Kaboom observation
- Audit log shows operation with before/after hashes

#### Acceptance Criteria:
- [ ] Database reset completes in <500ms
- [ ] Checksums differ, proving state change
- [ ] Subsequent operations use clean state
- [ ] No data loss outside target tables

---

### Scenario 2: Inject Test User & Query State
**Objective:** Verify data injection and state inspection

#### Setup:
- Backend database is clean (from Scenario 1)
- AI agent ready to create test user

#### Steps:
1. Call `interact({action: "backend_control", operation: "create_test_user", params: {email: "test@example.com", plan: "pro", credits: 100}})`
2. Verify operation returns `{user_id: 12345}`
3. Call `observe({what: "backend_state", service: "api-server", keys: ["user_count"]})`
4. Verify user_count is 1
5. Query by email in backend: user exists with plan="pro", credits=100

#### Expected Result:
- User created successfully, assigned user_id
- State observable through Kaboom
- Data persists through service restart
- Audit log shows create_test_user operation

#### Acceptance Criteria:
- [ ] Test user created with all fields set correctly
- [ ] User_id is deterministic for repeated test runs (seeded)
- [ ] State queries return accurate results
- [ ] Operation appears in audit log with correlation_id

---

### Scenario 3: Simulate Payment Timeout & Observe Frontend Response
**Objective:** Verify failure simulation and full-stack correlation

#### Setup:
- Test user created (from Scenario 2)
- Frontend loaded on localhost:3000
- Kaboom has record correlation_id="test-payment-003"

#### Steps:
1. Create BEFORE snapshot: `configure({action: "snapshot", session_action: "create", name: "before_payment"})`
2. Enable payment timeout simulation: `interact({action: "backend_control", operation: "simulate_payment_timeout", params: {duration_ms: 3000, error_code: "TIMEOUT"}})`
3. Frontend user clicks "Checkout" button
4. Observe frontend error: `observe({what: "logs", correlation_id: "test-payment-003"})`
5. Observe backend timeout: `observe({what: "backend-logs", correlation_id: "test-payment-003", level: "ERROR"})`
6. Verify correlation_id appears in both frontend and backend logs

#### Expected Result:
- Timeout simulation active for 3 seconds
- Frontend XHR returns 500 or timeout error
- User sees error message
- Backend logs show timeout error with same correlation_id
- Frontend & backend logs correlated in observation output

#### Acceptance Criteria:
- [ ] Frontend error appears within 3.5s (timeout + overhead)
- [ ] Backend logs contain correlation_id
- [ ] Frontend and backend events have matching timestamps (±100ms)
- [ ] Error details preserved in correlation view

---

### Scenario 4: Restore State & Retry Test
**Objective:** Verify state restoration and deterministic test retry

#### Setup:
- BEFORE snapshot taken (from Scenario 3)
- Database modified by payment test
- AI agent wants to retry test with clean state

#### Steps:
1. Call `configure({action: "snapshot", session_action: "restore", snapshot_id: "before_payment"})`
2. Verify `{status: "restored", rows_recovered: 1}`
3. Verify state matches pre-test: `observe({what: "backend_state", service: "api-server", keys: ["user_count", "order_count"]})`
4. Re-run payment flow
5. Verify same error path as first attempt (deterministic)

#### Expected Result:
- Restore completes in <1s
- State matches BEFORE snapshot exactly
- Second test run identical to first
- No data loss during restore

#### Acceptance Criteria:
- [ ] State restored completely
- [ ] Test replay deterministic
- [ ] No partial restores or corruption
- [ ] Restore logged in audit with source snapshot_id

---

### Scenario 5: Feature Flag Toggle & Path Verification
**Objective:** Verify configuration mutation and conditional behavior

#### Setup:
- Backend has feature flag `use_new_checkout` (default: false)
- Frontend has conditional rendering based on flag
- Test user ready

#### Steps:
1. Call `interact({action: "backend_control", operation: "set_feature_flag", params: {flag_name: "use_new_checkout", value: true}})`
2. Frontend reloads: `interact({action: "refresh"})`
3. Verify new checkout UI appears (old UI should not)
4. Toggle flag back to false: `interact({action: "backend_control", operation: "set_feature_flag", params: {flag_name: "use_new_checkout", value: false}})`
5. Refresh and verify old checkout UI appears

#### Expected Result:
- Feature flag toggle succeeds
- Frontend reflects flag change on reload
- Feature gates work correctly
- Audit log shows both toggle operations

#### Acceptance Criteria:
- [ ] Flag change reflected immediately in backend
- [ ] Frontend conditional rendering correct
- [ ] Flag persists through service restart
- [ ] Toggle operations audit logged

---

### Scenario 6: Dry Run Mode (No Actual Changes)
**Objective:** Verify dry-run prevents database modifications

#### Setup:
- Database has 1250 users
- Dry run mode enabled

#### Steps:
1. Call `interact({action: "backend_control", operation: "reset_database", dry_run: true})`
2. Verify response includes `{predicted_rows_deleted: 1250}` but no actual deletion
3. Query user_count: should still be 1250
4. Run same operation with `dry_run: false`
5. Verify user_count is now 0

#### Expected Result:
- Dry-run predicts result without executing
- Database unchanged after dry-run
- Actual execution removes data as expected
- Both operations logged separately

#### Acceptance Criteria:
- [ ] Dry-run marked as such in audit log
- [ ] Dry-run result matches actual result
- [ ] No side effects from dry-run

---

### Scenario 7: Shadow Mode (Read-Only Operation)
**Objective:** Verify shadow_mode prevents writes but allows reads

#### Setup:
- Backend service in shadow_mode support
- Database state at baseline

#### Steps:
1. Call `observe({what: "backend_state", service: "api-server"})` with implicit shadow_mode=true
2. Verify data readable without modification
3. Call `interact({action: "backend_control", operation: "reset_database", shadow_mode: true})`
4. Verify operation returns predicted result but doesn't execute
5. Query database: all users still present

#### Expected Result:
- Shadow mode read operations work normally
- Shadow mode write operations return predicted results without execution
- Database unchanged

#### Acceptance Criteria:
- [ ] Shadow mode clearly marked in operation response
- [ ] No data modifications in shadow mode
- [ ] Predictions accurate

---

### Scenario 8: Error Handling & Rollback
**Objective:** Verify error handling and automatic rollback

#### Setup:
- BEFORE snapshot taken
- Backend ready for failure injection

#### Steps:
1. Inject failure into reset_database: service returns error
2. Call `interact({action: "backend_control", operation: "reset_database"})`
3. Verify operation fails: `{status: "error", error: "database_lock_timeout"}`
4. Verify AI offers automatic restore to BEFORE snapshot
5. Call rollback: `configure({action: "snapshot", session_action: "restore", snapshot_id: "before_*"})`
6. Verify state unchanged

#### Expected Result:
- Error returned with descriptive message
- AI/developer offered rollback
- Rollback succeeds completely
- Error logged in audit with rollback confirmation

#### Acceptance Criteria:
- [ ] Error messages descriptive and actionable
- [ ] Automatic rollback available after failure
- [ ] State integrity maintained
- [ ] Error operations audit logged

---

### Scenario 9: Service Discovery & Operation Enumeration
**Objective:** Verify Kaboom discovers available control operations

#### Setup:
- Backend service with /.kaboom/control handler
- Kaboom MCP just started

#### Steps:
1. Kaboom MCP starts, pings /.kaboom/control?discover=true
2. Backend returns available operations with schemas
3. Call `observe({what: "backend_state", service: "api-server"})`
4. Verify operations list includes reset_database, create_test_user, simulate_*, etc.
5. Call operation with wrong params: should fail with schema validation error

#### Expected Result:
- Service discovered automatically
- All operations enumerated
- Operations cacheable (TTL 30s)
- Schema validation enforced

#### Acceptance Criteria:
- [ ] Discovery completes <100ms
- [ ] All operations schema-validated
- [ ] Cache prevents excessive discovery calls
- [ ] Invalid params rejected with helpful error

---

### Scenario 10: Correlation ID Propagation Through Full Stack
**Objective:** Verify correlation IDs flow through all layers

#### Setup:
- Backend logging with correlation_id support
- Frontend with session tracking
- Kaboom MCP with correlation tracing

#### Steps:
1. Generate correlation_id: `req-test-flow-10-001`
2. Reset database with correlation_id
3. Create test user with same correlation_id
4. Simulate timeout with same correlation_id
5. Frontend user clicks checkout
6. Backend processes and logs with correlation_id
7. Call `observe({what: "logs", correlation_id: "req-test-flow-10-001"})`
8. Verify all events appear: reset, create_user, simulate, click, XHR, backend error

#### Expected Result:
- All operations tagged with same correlation_id
- Unified timeline shows causality
- Correlation_id searchable across all buffers
- Timeline helps developer trace issue root cause

#### Acceptance Criteria:
- [ ] Correlation ID preserved through all layers
- [ ] Timeline shows chronological order
- [ ] All related events accessible via single correlation_id query

---

## Acceptance Criteria (Overall)
- [ ] All scenarios pass on Linux, macOS, Windows
- [ ] Performance meets <50ms read, <200ms write targets
- [ ] Snapshot/restore completes in <1s
- [ ] Audit log records all operations
- [ ] Correlation IDs propagate correctly
- [ ] Error messages help developers debug issues
- [ ] No data loss during snapshot/restore
- [ ] Operations survive service restart

## Test Data

### Database Seed
```sql
INSERT INTO users (id, email, plan, credits, created_at) VALUES
  (1, 'seed-user-1@example.com', 'free', 0, NOW()),
  (2, 'seed-user-2@example.com', 'pro', 100, NOW()),
  (3, 'seed-user-3@example.com', 'enterprise', 1000, NOW());

INSERT INTO products (id, name, price, active) VALUES
  (1, 'Product A', 9.99, true),
  (2, 'Product B', 19.99, true),
  (3, 'Product C', 99.99, true);

INSERT INTO feature_flags (name, value, percentage) VALUES
  ('use_new_checkout', false, 0),
  ('enable_beta_features', false, 0);
```

### Test Fixtures
- Valid payment: {user_id: 1, product_id: 1, amount: 9.99}
- Invalid payment: {user_id: 999, product_id: 1, amount: -10}
- Timeout params: {duration_ms: 3000, error_code: "TIMEOUT"}

## Regression Tests

**Critical:** After each change, verify:
1. reset_database doesn't affect other services
2. Snapshots don't corrupt database
3. Corruption detection works (checksum validation)
4. Audit log never loses entries
5. Restore never creates orphaned data
6. Correlation IDs never collide
7. Timeout simulation doesn't permanently break service

## Performance Baseline

| Operation | Target | Measured | Status |
|-----------|--------|----------|--------|
| reset_database | <200ms | _ | _ |
| create_test_user | <50ms | _ | _ |
| set_feature_flag | <50ms | _ | _ |
| snapshot create | <500ms | _ | _ |
| snapshot restore | <1000ms | _ | _ |
| query backend_state | <50ms | _ | _ |
| discover operations | <100ms | _ | _ |

## Known Issues & Limitations

- [ ] Multi-service rollback (each service snapshotted separately)
- [ ] No distributed transaction coordination (best-effort)
- [ ] Race conditions possible if service crashes mid-operation
- [ ] No encryption for snapshots (dev-only, single machine)
