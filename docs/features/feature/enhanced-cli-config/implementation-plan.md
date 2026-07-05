---
status: proposed
scope: feature/enhanced-cli-config/implementation
ai-priority: high
tags: [implementation, planning]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
last_reviewed: 2026-07-05
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Implementation Plan: Enhanced CLI Configuration Management

**Timeline:** 24-30 hours (developer time for both NPM + PyPI)
**Test-Driven Approach:** Write tests first, implement after tests pass
**Strategy:** Implement NPM wrapper first (fully tested), then port to Python with parallel testing

---

## Phase Overview

| Phase | Task | Time | Status |
|-------|------|------|--------|
| **Phase 0** | NPM Setup + Test Framework | 1 hour | TODO |
| **Phase 1** | NPM Refactor Existing Code | 1 hour | TODO |
| **Phase 2** | NPM Implement --dry-run | 2 hours | TODO |
| **Phase 3** | NPM Implement --doctor | 2 hours | TODO |
| **Phase 4** | NPM Client Registry + Multi-Client | 1.5 hours | DONE |
| **Phase 5** | NPM Implement --env | 1.5 hours | TODO |
| **Phase 6** | NPM Implement --uninstall | 2 hours | TODO |
| **Phase 7** | NPM Error Messages + Testing | 2 hours | TODO |
| **Phase 8** | NPM Integration Testing + UAT | 2 hours | TODO |
| **Phase 9** | Python Setup + Port from NPM | 1.5 hours | TODO |
| **Phase 10** | Python Testing + UAT | 2 hours | TODO |
| **Phase 11** | Feature Parity Verification | 1.5 hours | TODO |
| **Total** | | **24 hours** | |

### Notes:
- Phases 0-8: NPM wrapper (full implementation + testing)
- Phases 9-11: Python wrapper (port + parity verification)
- Both wrappers must produce identical output and behavior
- Tests for Python written after implementation (mirroring NPM tests)

---

## Phase 0: Setup + Test Framework (1 hour)

### Tasks

1. **Create module structure**
   - [ ] Create `/Users/brenn/dev/kaboom/npm/kaboom-agentic-browser/lib/` directory
   - [ ] Create `lib/config.js` (config file utilities)
   - [ ] Create `lib/doctor.js` (diagnostics)
   - [ ] Create `lib/install.js` (install logic)
   - [ ] Create `lib/uninstall.js` (uninstall logic)
   - [ ] Create `lib/output.js` (formatters)
   - [ ] Create `lib/errors.js` (error classes and messages)

2. **Create test structure**
   - [ ] Create `/Users/brenn/dev/kaboom/tests/cli/` directory
   - [ ] Create `config.test.js` (config utilities tests)
   - [ ] Create `install.test.js` (install flow tests)
   - [ ] Create `doctor.test.js` (doctor tests)
   - [ ] Create `uninstall.test.js` (uninstall tests)
   - [ ] Create `cli.test.js` (CLI argument parsing)

3. **Error Message Catalog**
   - [ ] Document all error messages user will see
   - [ ] Define format: `{icon} {action-required? | info} | recovery suggestion`

### Deliverables

- Empty module files (stub functions)
- Test file structure
- Error message list

---

## Phase 1: Refactor Existing Code (1 hour)

**Goal:** Extract config utilities from existing code without changing behavior

### Current Code Location
`npm/kaboom-agentic-browser/bin/kaboom-agentic-browser` (Lines 70-151)

### Tasks

1. **Extract `lib/config.js`**
   - [ ] Create `readConfigFile(path)` function
     - Read JSON from file
     - Return `{valid: bool, data: obj, error: string, stats: {size, mtime}}`
     - Handle missing files gracefully
   - [ ] Create `writeConfigFile(path, data, dryRun)` function
     - If `dryRun=true`: return what would be written (no actual write)
     - If `dryRun=false`: atomic write (temp file + rename)
     - Return `{success: bool, message: string, path: string}`
   - [ ] Create `validateMCPConfig(data)` function
     - Check `data.mcpServers` is object
     - Return `{valid: bool, errors: [strings]}`
   - [ ] Create `CLIENT_DEFINITIONS` registry with all 5 clients
     - Each entry: `{id, name, type, configPath|detectCommand, detectDir|installArgs, removeArgs}`
   - [ ] Create `getDetectedClients()` function
     - Return array of clients whose config dir or CLI exists
   - [ ] Create `getClientConfigPath(def)` function
     - Return platform-specific config path
   - [ ] Create `isClientInstalled(def)` function
     - Check dir existence (file-type) or command existence (cli-type)

2. **Extract `lib/install.js`**
   - [ ] Create `mergeKaboomConfig(existing, kaboomEntry, envVars)` function
     - Merge Kaboom-managed entry into `existing.mcpServers`
     - Preserve other MCP servers
     - Return merged config

3. **Update `lib/output.js`**
   - [ ] Create `formatSuccess(message, details)` → `✅ message`
   - [ ] Create `formatError(message, recovery)` → `❌ message\n   Recovery: recovery`
   - [ ] Create `formatInfo(message)` → `ℹ️  message`
   - [ ] Create `formatWarning(message)` → `⚠️  message`

4. **Update `bin/kaboom-agentic-browser`**
   - [ ] Import from `lib/config.js`, `lib/install.js`, `lib/output.js`
   - [ ] Replace inline code with function calls
   - [ ] Verify existing `--config`, `--install`, `--help` still work (regression test)

### Tests to Write First

```javascript
// npm/kaboom-agentic-browser/lib/config.test.js
- [ ] CLIENT_DEFINITIONS has 5 entries with correct fields
- [ ] getClientConfigPath() returns correct platform paths
- [ ] isClientInstalled() detects existing/missing dirs
- [ ] commandExistsOnPath() finds/misses CLIs
- [ ] getDetectedClients() returns only installed clients
- [ ] expandPath() resolves ~ and %APPDATA%
- [ ] getConfigCandidates() backward compat wrapper
- [ ] getToolNameFromPath() backward compat wrapper
```

### Deliverables

- `lib/config.js` with client registry and detection functions
- `lib/install.js` with merge function
- `lib/output.js` with formatters
- Updated `bin/kaboom-agentic-browser` using new functions
- All Phase 1 tests passing

---

## Phase 2: Implement --dry-run (2 hours)

**Goal:** Show what changes would be made without writing files

### Tasks

1. **Update CLI argument parsing**
   - [ ] Parse `--dry-run` flag
   - [ ] Pass `dryRun: true` through install flow

2. **Implement in `lib/install.js`**
   - [ ] Create `executeInstall(options)` function
     - `options = {dryRun: bool, envVars: {}, verbose: bool}`
     - Return `{success: bool, updated: [paths], errors: [details], diffs: [beforeAfter]}`
   - [ ] If `dryRun=true`: collect diffs, don't write
   - [ ] Return detailed diff for each candidate

3. **Add diff formatter in `lib/output.js`**
   - [ ] Create `formatDiff(before, after)` function
     - Show before/after JSON
     - Highlight changes
     - Clear for user to understand

4. **Update `bin/kaboom-agentic-browser`**
   - [ ] Handle `--install --dry-run` flag
   - [ ] Call `executeInstall({dryRun: true})`
   - [ ] Display diffs without prompting for confirmation

### Tests to Write First

```javascript
// tests/cli/install.test.js
- [ ] executeInstall({dryRun: true}) returns diff without writing
- [ ] executeInstall({dryRun: true}) file doesn't exist after call
- [ ] executeInstall({dryRun: false}) actually writes file
- [ ] Diff shows before/after JSON clearly
- [ ] Multiple --dry-run calls produce same result
```

### Deliverables

- `executeInstall()` function with dryRun support
- Diff formatter
- Updated CLI handling
- Phase 2 tests passing

---

## Phase 3: Implement --doctor (2 hours)

**Goal:** Diagnostic tool to check all configs are valid

### Tasks

1. **Update CLI argument parsing**
   - [ ] Parse `--doctor` flag
   - [ ] Pass through to doctor engine

2. **Implement in `lib/doctor.js`**
   - [ ] Create `runDiagnostics(verbose)` function
     - For each config candidate:
       - Check file exists
       - Validate JSON syntax
       - Verify kaboom-browser-devtools entry present
       - Check binary exists and is executable
     - Return report: `{clients: [{name, type, status, issues, suggestions}], legacyWarnings: [], summary: string}`

3. **Binary testing in `lib/doctor.js`**
   - [ ] Create `testBinary()` function
     - Run `kaboom-agentic-browser --version`
     - Check exit code = 0
     - Extract version from output
     - Return `{ok: bool, version: string, error?: string}`

4. **Add diagnostic report formatter in `lib/output.js`**
   - [ ] Create `formatDiagnosticReport(report)` function
     - Show each tool with status (✅/❌/⚠️)
     - List issues and recovery suggestions
     - Summary at bottom

5. **Update `bin/kaboom-agentic-browser`**
   - [ ] Handle `--doctor` flag
   - [ ] Call `runDiagnostics(verbose)`
   - [ ] Display formatted report

### Tests to Write First

```javascript
// tests/cli/doctor.test.js
- [ ] runDiagnostics() checks all 4 candidates
- [ ] Status "ok" when config valid + kaboom present + binary works
- [ ] Status "error" when JSON invalid
- [ ] Status "error" when kaboom missing
- [ ] Status "error" when binary not found
- [ ] Provides recovery suggestions for each issue
- [ ] Summary text is accurate
- [ ] testBinary() runs kaboom-agentic-browser --version
- [ ] testBinary() returns version string
```

### Deliverables

- `runDiagnostics()` function
- `testBinary()` function
- Diagnostic report formatter
- Phase 3 tests passing

---

## Phase 4: Implement Client Registry + Multi-Client Install (1.5 hours)

**Goal:** Install to all detected clients using client registry

### Tasks

1. **Implement client registry in `lib/config.js`**
   - [ ] Create `CLIENT_DEFINITIONS` array with 5 client entries
   - [ ] Support `type: 'cli'` (Claude Code) and `type: 'file'` (rest)
   - [ ] Platform-specific paths: `{ darwin, win32, linux, all }`

2. **Update `lib/install.js`**
   - [ ] Create `installViaCli()` for CLI-type clients (subprocess)
   - [ ] Create `installViaFile()` for file-type clients (config write)
   - [ ] Create `installToClient()` dispatcher
   - [ ] `executeInstall()` targets all detected clients by default

3. **Add multi-client formatter in `lib/output.js`**
   - [ ] Show count: "5/5 clients installed"
   - [ ] List each client with method (via CLI / at path)
   - [ ] Show any errors

### Tests to Write First

```javascript
// npm/kaboom-agentic-browser/lib/install.test.js
- [ ] installToClient() dispatches file-type correctly
- [ ] installToClient() creates new config
- [ ] installToClient() merges into existing config
- [ ] installToClient() handles dry-run
- [ ] executeInstall() processes all detected clients
- [ ] Errors in one client don't stop processing others
```

### Deliverables

- Client registry in `lib/config.js`
- Updated `executeInstall()` with multi-client support
- Phase 4 tests passing

---

## Phase 5: Implement --env (1.5 hours)

**Goal:** Allow injecting environment variables into config

### Tasks

1. **Update CLI argument parsing**
   - [ ] Parse `--env KEY=VALUE` arguments (multiple allowed)
   - [ ] Validate format: contains `=`, non-empty key and value
   - [ ] Collect into `{KEY: VALUE, ...}` object
   - [ ] Reject invalid formats with helpful error

2. **Update `lib/install.js`**
   - [ ] Modify `executeInstall()` to accept `envVars` option
   - [ ] Merge env vars into config's `env` object
   - [ ] If no env vars: don't add empty `env: {}` object
   - [ ] Multiple calls merge env vars (don't replace)

3. **Add env validation in `lib/errors.js`**
   - [ ] Create `validateEnvVar(str)` function
     - Check format: `KEY=VALUE`
     - Validate key: no null bytes, no control chars
     - Return `{valid: bool, key: string, value: string, error?: string}`

4. **Update `bin/kaboom-agentic-browser`**
   - [ ] Parse all `--env` arguments
   - [ ] Build env var object
   - [ ] Pass to `executeInstall({envVars: {...}})`
   - [ ] Show env vars in output

### Tests to Write First

```javascript
// tests/cli/install.test.js
- [ ] executeInstall({envVars: {KEY: "value"}}) adds env to config
- [ ] Multiple env vars merged correctly
- [ ] Empty env vars object doesn't add env field
- [ ] --env without --install shows error: "--env only works with --install"
- [ ] Invalid env format "--env BADFORMAT" shows error with example
- [ ] Env vars persisted in config file
- [ ] Multiple calls to --install --env merge vars (don't replace)
```

### Deliverables

- Updated `executeInstall()` with envVars support
- Env var validation
- CLI parsing for `--env` flag
- Phase 5 tests passing

---

## Phase 6: Implement --uninstall (2 hours)

**Goal:** Remove Kaboom-managed entries from configs cleanly, preserving other MCP servers

### Tasks

1. **Update CLI argument parsing**
   - [ ] Parse `--uninstall` flag
   - [ ] Pass through to uninstall engine

2. **Implement in `lib/uninstall.js`**
   - [ ] Create `executeUninstall(options)` function
     - `options = {dryRun: bool, verbose: bool}`
     - For each config candidate:
       - Read config
       - Check if kaboom present
       - Remove kaboom-browser-devtools entry
       - Preserve other MCP servers
       - Write back if changed
     - Return `{success: bool, removed: [paths], notFound: [paths], errors: [details]}`

3. **Add uninstall formatter in `lib/output.js`**
   - [ ] Create `formatUninstallResults(results)` function
     - Show count: "Removed from N clients"
     - List which clients updated
     - Show any errors

4. **Update `bin/kaboom-agentic-browser`**
   - [ ] Handle `--uninstall` flag
   - [ ] Handle `--uninstall --dry-run` (show what would be removed)
   - [ ] Call `executeUninstall()`
   - [ ] Display results

### Tests to Write First

```javascript
// tests/cli/uninstall.test.js
- [ ] executeUninstall() removes kaboom-browser-devtools entry
- [ ] Preserves other MCP servers
- [ ] Works when kaboom not present (no-op)
- [ ] dryRun=true shows what would be removed
- [ ] dryRun=false actually removes
- [ ] Produces valid JSON after removal
- [ ] Reports count: "Removed from N clients"
- [ ] Handles non-existent config gracefully
```

### Deliverables

- `executeUninstall()` function
- Uninstall formatter
- Phase 6 tests passing

---

## Phase 7: Error Messages + Help Text (2 hours)

**Goal:** Comprehensive error messages with recovery suggestions

### Tasks

1. **Create error message catalog in `lib/errors.js`**
   - [ ] Permission denied reading/writing
   - [ ] Invalid JSON + line number
   - [ ] Binary not found
   - [ ] Directory doesn't exist
   - [ ] File locked
   - [ ] --env without --install
   - [ ] Invalid env format
   - [ ] Invalid flag combinations
   - [ ] File size limit exceeded
   - [ ] Symlink resolution failed

2. **Update error handlers in each lib file**
   - [ ] All file operations wrapped in try-catch
   - [ ] Return descriptive errors with recovery suggestions
   - [ ] Use consistent format: `{success: false, message: string, recovery?: string}`

3. **Update help text in `bin/kaboom-agentic-browser`**
   - [ ] Add one-liner for each command
   - [ ] Add example for each flag combination
   - [ ] Update `--help` output

4. **Update `lib/output.js`**
   - [ ] Create `formatError(message, recovery)` function
   - [ ] Include actionable next steps
   - [ ] Format recovery suggestions clearly

### Tests to Write First

```javascript
// tests/cli/cli.test.js
- [ ] Error message on permission denied includes `sudo` suggestion
- [ ] Error message on invalid JSON shows line number
- [ ] Error message on missing binary shows install command
- [ ] All error messages are clear and actionable
- [ ] --help text is complete and accurate
- [ ] Help examples for all flag combinations
```

### Deliverables

- Error message catalog
- Updated error handlers
- Updated help text
- Phase 7 tests passing

---

## Phase 8: Integration Testing + UAT (3 hours)

**Goal:** End-to-end testing and human verification

### Tasks

1. **Integration tests**
   - [ ] Test all command combinations (install, doctor, uninstall, flags)
   - [ ] Test workflow: install → doctor → uninstall
   - [ ] Test backward compatibility (v5.2 commands still work)
   - [ ] Test concurrent operations (race conditions)
   - [ ] Test edge cases: missing files, invalid JSON, permissions

2. **Manual UAT (7 scenarios from QA_PLAN)**
   - [ ] Scenario 1: Safe installation (~7 min)
   - [ ] Scenario 2: Multi-client install (~8 min)
   - [ ] Scenario 3: Environment variables (~6 min)
   - [ ] Scenario 4: Doctor diagnostics (~10 min)
   - [ ] Scenario 5: Uninstall (~5 min)
   - [ ] Scenario 6: Error recovery (~7 min)
   - [ ] Scenario 7: Backward compatibility (~5 min)

3. **Cross-platform testing**
   - [ ] Test on macOS
   - [ ] Test on Linux (if available)
   - [ ] Test on Windows (path handling, permissions)

4. **Performance testing**
   - [ ] Doctor completes in < 1 second
   - [ ] Install completes in < 1 second
   - [ ] No file descriptor leaks
   - [ ] No memory leaks on repeated operations

### Deliverables

- All integration tests passing
- All 7 UAT scenarios passing
- Cross-platform testing verified
- Performance baselines met

---

## Error Message Catalog

Reference for Phase 7 implementation:

### Permission Errors
```
❌ Error: Permission denied writing ~/.cursor/mcp.json
   Recovery:
   1. Try: sudo kaboom-agentic-browser --install
   2. Or: Check permissions with: ls -la ~/.cursor/
   3. Or: Change permissions: chmod 755 ~/.cursor
```

### JSON Errors
```
❌ Error: Invalid JSON in ~/.cursor/mcp.json at line 15
   Unexpected token }

   Recovery:
   1. Manually edit: code ~/.cursor/mcp.json
   2. Or: Restore from backup and try --install again
   3. Or: Run: kaboom-agentic-browser --doctor (for diagnostics)
```

### Binary Not Found
```
❌ Error: Kaboom binary not found
   Expected: /usr/local/bin/kaboom-agentic-browser

   Recovery:
   1. Reinstall: npm install -g kaboom-agentic-browser@latest
   2. Check PATH: echo $PATH
   3. Or: specify PATH manually (advanced)
```

### Env Var Errors
```
❌ Error: --env only works with --install
   Usage: kaboom-agentic-browser --install --env KEY=VALUE

   Examples:
   - kaboom-agentic-browser --install --env DEBUG=1
   - kaboom-agentic-browser --install --env KABOOM_SERVER=http://localhost:7890
```

```
❌ Error: Invalid env format "BADFORMAT". Expected: KEY=VALUE

   Examples of valid formats:
   - --env DEBUG=1
   - --env KABOOM_SERVER=http://localhost:7890
   - --env LOG_LEVEL=info
```

---

## Testing Checklist (TDD Approach)

### Write Tests First, Then Implement

For each phase:
1. Write test file with all test cases
2. Verify tests fail (red)
3. Implement function
4. Verify tests pass (green)
5. Refactor if needed (blue)
6. Move to next phase

### Critical Tests (Must Pass)

- [ ] Multi-client: `--install` targets all detected clients by default
- [ ] No file corruption: `--dry-run` never writes
- [ ] Other MCP servers preserved: only kaboom-browser-devtools entry modified
- [ ] Atomic writes: files only written completely, never half-written
- [ ] Error messages: all errors have actionable recovery suggestions
- [ ] Security: env vars safely stored, paths safe from traversal
- [ ] CLI-type clients: subprocess with correct env var handling

---

## Phase 9: Python Setup + Port from NPM (1.5 hours)

**Goal:** Create Python equivalents of all NPM modules with identical logic

### Tasks

1. **Create Python module structure**
   - [ ] Create `pypi/kaboom-agentic-browser/kaboom_agentic_browser/config.py`
     - Port `CLIENT_DEFINITIONS`, detection, path resolution
     - Use `shutil.which()` for CLI detection, `os.path.isdir()` for file-type
     - Match Node.js behavior exactly (same defaults, same errors)

   - [ ] Create `pypi/kaboom-agentic-browser/kaboom_agentic_browser/doctor.py`
     - Port `runDiagnostics()`, `testBinary()`
     - Use `subprocess.run()` for binary invocation
     - Match Node.js error detection

   - [ ] Create `pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py`
     - Port `executeInstall()`, `installToClient()`, `installViaCli()`, `installViaFile()`
     - Handle dryRun, envVars options identically

   - [ ] Create `pypi/kaboom-agentic-browser/kaboom_agentic_browser/uninstall.py`
     - Port `executeUninstall()`
     - Preserve other MCP servers

   - [ ] Create `pypi/kaboom-agentic-browser/kaboom_agentic_browser/output.py`
     - Port all formatters: success, error, info, warning
     - Ensure emoji output matches NPM version

   - [ ] Create `pypi/kaboom-agentic-browser/kaboom_agentic_browser/errors.py`
     - Define error classes and messages
     - Use same error message catalog as NPM

2. **Update entry point**
   - [ ] Modify `pypi/kaboom-agentic-browser/kaboom_agentic_browser/__main__.py`
     - Add CLI argument parsing (similar to NPM)
     - Route commands to handlers (--config, --install, --doctor, etc.)
     - Call binary for non-config commands

3. **Verify imports work**
   - [ ] All Python modules import correctly
   - [ ] No missing stdlib imports
   - [ ] Entry point executable: `python -m kaboom_agentic_browser`

### Key Porting Rules

1. **Use pathlib.Path over os.path**
   ```python
   # Python idiomatic
   from pathlib import Path
   home = Path.home()
   config_path = home / '.cursor' / 'mcp.json'
   ```

2. **Use json module**
   ```python
   import json
   with open(config_path) as f:
       config = json.load(f)
   ```

3. **Atomic writes (same as Node.js)**
   ```python
   import tempfile
   with tempfile.NamedTemporaryFile(mode='w', delete=False) as tmp:
       json.dump(config, tmp)
       temp_path = tmp.name
   os.replace(temp_path, config_path)  # Atomic
   ```

4. **Subprocess for binary invocation**
   ```python
   import subprocess
   result = subprocess.run([binary, '--version'], capture_output=True, text=True)
   ```

### Deliverables

- 6 Python modules in `pypi/kaboom-agentic-browser/kaboom_agentic_browser/`
- Updated `__main__.py` with CLI routing
- All modules use stdlib only (no new dependencies)
- Python code structure mirrors NPM logic

---

## Phase 10: Python Testing + UAT (2 hours)

**Goal:** Verify Python implementation works correctly

### Tasks

1. **Unit tests (mirror NPM tests)**
   - [ ] Create `tests/cli/python/` directory
   - [ ] Create test files:
     - `test_config.py` - Config utilities
     - `test_install.py` - Install flow
     - `test_doctor.py` - Doctor diagnostics
     - `test_uninstall.py` - Uninstall flow
     - `test_cli.py` - CLI argument parsing
   - [ ] All tests pass (same coverage as NPM)

2. **Manual UAT (same 7 scenarios)**
   - [ ] Install via `pip install kaboom-agentic-browser`
   - [ ] Run all 7 UAT scenarios (Scenario 1-7 from QA_PLAN)
   - [ ] Verify output matches NPM version exactly

3. **Cross-platform testing**
   - [ ] Python on macOS (if available)
   - [ ] Python on Linux (CI should cover)
   - [ ] Python on Windows (if available)

### Deliverables

- All unit tests passing
- All 7 UAT scenarios passing
- Cross-platform verification

---

## Phase 11: Feature Parity Verification (1.5 hours)

**Goal:** Ensure NPM and Python wrappers are identical

### Tasks

1. **Output comparison**
   - [ ] Run same command in NPM and Python wrapper
   - [ ] Compare output byte-for-byte (same messages, emojis, line endings)
   - [ ] Test cases:
     - `--config` output identical
     - `--install` messages identical
     - `--doctor` report format identical
     - `--help` text identical
     - Error messages identical

2. **Behavior verification**
   - [ ] `--install` both update all detected clients
     - Verify all detected client configs modified
   - [ ] `--dry-run` both preview without writing
     - Verify no files written in both
   - [ ] `--env` parsing identical
     - Same validation, same error messages
   - [ ] `--doctor` diagnostics identical
     - Same checks, same status symbols

3. **Error parity**
   - [ ] Permission denied errors identical
   - [ ] Invalid JSON errors identical
   - [ ] Binary not found errors identical
   - [ ] Recovery suggestions identical

4. **CI/CD verification**
   - [ ] Both npm and pip releases build correctly
   - [ ] CI passes for both wrappers
   - [ ] PyPI package installs without errors

### Deliverables

- Parity verification checklist complete
- Both wrappers produce identical output
- Both wrappers have identical behavior
- CI/CD passes for both

---

## Success Criteria

✅ **Implementation is complete when:**

1. **NPM Wrapper (Phases 0-8)**
   - All 8 phases finished and tested
   - All automated tests passing (unit + integration)
   - All 7 UAT scenarios passing
   - Backward compatibility verified

2. **Python Wrapper (Phases 9-11)**
   - All Python modules created and tested
   - All 7 UAT scenarios passing with Python
   - Feature parity verified (output identical)
   - Python on multiple platforms tested

3. **Cross-Wrapper**
   - Both wrappers produce identical output
   - Both wrappers have identical behavior
   - Performance baselines met (all commands < 1 second)
   - Code review passed for both
   - CI/CD passes for both
   - Ready for v5.3.0 release on both NPM and PyPI

---

## References

- Product Spec: `product-spec.md`
- Tech Spec: `tech-spec.md`
- QA Plan: `qa-plan.md`
- Review Summary: `review-summary.md`
- Existing CLI Code: `/Users/brenn/dev/kaboom/npm/kaboom-agentic-browser/bin/kaboom-agentic-browser`
