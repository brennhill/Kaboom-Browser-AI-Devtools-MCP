---
feature: project-isolation
doc_type: qa-plan
feature_id: feature-project-isolation
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Project Isolation

> How to test this feature. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Project registry operations
- [ ] Test GetOrCreate returns existing project
- [ ] Test GetOrCreate creates new project if not exists
- [ ] Test UpdateActivity updates timestamp
- [ ] Test project_key validation (reject special characters)
- [ ] Test default project used when no key provided

**Integration tests:** Full isolation
- [ ] Test two MCP clients with different project_keys see separate logs
- [ ] Test extension HTTP with different project headers routes correctly
- [ ] Test observe tool filtered by project_key
- [ ] Test interact tool (pending queries) scoped to project

**Edge case tests:** Expiration
- [ ] Test project expires after inactivity timeout
- [ ] Test expired project recreated on new activity
- [ ] Test expiration doesn't affect active projects

### Security/Compliance Testing

**Isolation tests:** Verify no data leakage
- [ ] Test projectA cannot access projectB logs
- [ ] Test projectA cannot access projectB network data
- [ ] Test projectA cannot execute projectB pending queries

#### Injection tests:
- [ ] Test malicious project_key (SQL injection attempt) rejected
- [ ] Test project_key with path traversal (../../../) rejected

---

## Human UAT Walkthrough

### Scenario 1: Multi-Agent Isolation (Happy Path)
1. Setup:
   - Start Kaboom server
   - Open two Claude Code sessions (Agent A, Agent B)
2. Steps:
   - [ ] Agent A connects with project_key="projectA"
   - [ ] Agent B connects with project_key="projectB"
   - [ ] Agent A triggers error in browser tab
   - [ ] Agent A observes: `observe({what: "logs"})` — sees error
   - [ ] Agent B observes: `observe({what: "logs"})` — sees no error (empty or own logs only)
   - [ ] Agent B triggers different error
   - [ ] Agent B observes: sees only own error
   - [ ] Agent A observes: still sees only original error
3. Expected Result: Complete data isolation between projects
4. Verification: Logs are project-scoped, no cross-project visibility

### Scenario 2: Extension Multi-Project Routing
1. Setup:
   - Extension configured to send to specific project
2. Steps:
   - [ ] Tab 1: set project header to "projectX"
   - [ ] Tab 1: trigger console.log("Project X")
   - [ ] Tab 2: set project header to "projectY"
   - [ ] Tab 2: trigger console.log("Project Y")
   - [ ] Agent X (project_key="projectX") observes logs
   - [ ] Verify sees "Project X" only
   - [ ] Agent Y (project_key="projectY") observes logs
   - [ ] Verify sees "Project Y" only
3. Expected Result: Extension routes telemetry to correct project
4. Verification: Logs correctly isolated by project

### Scenario 3: Default Project (Backwards Compatibility)
1. Setup:
   - Agent connects without project_key
2. Steps:
   - [ ] Agent connects (no project_key param)
   - [ ] Trigger logs
   - [ ] Observe logs: succeeds
   - [ ] Verify using "default" project
3. Expected Result: Default project works transparently
4. Verification: No project_key required for single-project use

### Scenario 4: Project Expiration
1. Setup:
   - Start server with --project-expiration-minutes=1 (for testing)
   - Agent connects with project_key="temp"
2. Steps:
   - [ ] Agent triggers logs, observes (project active)
   - [ ] Wait 2 minutes (no activity)
   - [ ] Server logs "Project temp expired"
   - [ ] Agent attempts observe again
   - [ ] Project auto-recreated (empty buffers)
3. Expected Result: Inactive projects expire, memory freed
4. Verification: Project data cleared after expiration

### Scenario 5: List Active Projects (Admin)
1. Setup:
   - Multiple agents connected with different project_keys
2. Steps:
   - [ ] Admin observes: `observe({what: "projects"})`
   - [ ] Verify list shows all active projects with metadata
   - [ ] List includes: key, created time, last activity
3. Expected Result: Admin can see all active projects
4. Verification: Project metadata accurate

---

## Regression Testing

- Test single-project usage (no project_key) works as before
- Test all tools still work correctly with project isolation
- Test project isolation compatible with read-only mode and allowlisting

---

## Performance/Load Testing

- Test 10 simultaneous projects (memory usage <500MB)
- Test 100 projects (warn if threshold exceeded)
- Test expiration with 100 projects (scan completes <100ms)
- Test registry lookup overhead (<0.01ms per request)
