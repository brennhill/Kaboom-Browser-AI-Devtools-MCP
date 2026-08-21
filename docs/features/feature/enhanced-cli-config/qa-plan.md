---
feature: Enhanced CLI Configuration Management
doc_type: qa-plan
feature_id: feature-enhanced-cli-config
last_reviewed: 2026-07-05
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# QA Plan: Enhanced CLI Configuration Management

> How to test the enhanced CLI features. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

#### Unit Tests: Config File Operations

- [ ] `CLIENT_DEFINITIONS` contains all 5 clients with correct fields
- [ ] `getClientConfigPath()` returns correct platform-specific paths
- [ ] `isClientInstalled()` detects existing dirs for file-type clients
- [ ] `isClientInstalled()` detects CLI commands for cli-type clients
- [ ] `getDetectedClients()` returns only installed clients
- [ ] `expandPath()` resolves `~` and `%APPDATA%` correctly
- [ ] `readConfigFile()` returns parsed JSON for valid file
- [ ] `readConfigFile()` returns error for non-existent file
- [ ] `readConfigFile()` returns error for invalid JSON with line number
- [ ] `writeConfigFile()` with `dryRun=true` doesn't actually write file
- [ ] `writeConfigFile()` with `dryRun=false` creates file with correct permissions
- [ ] `writeConfigFile()` preserves other MCP server entries
- [ ] `validateMCPConfig()` rejects config without mcpServers
- [ ] `validateMCPConfig()` accepts valid MCP config structure

#### Unit Tests: Install Flow

- [ ] `executeInstall()` with `dryRun=true` returns diff without writing
- [ ] `executeInstall()` installs to all detected clients by default
- [ ] `installToClient()` dispatches file-type to file write, cli-type to subprocess
- [ ] `installViaCli()` runs correct subprocess with CLAUDECODE env var unset
- [ ] `executeInstall()` merges env vars into config
- [ ] `executeInstall()` with empty env vars doesn't add env object
- [ ] `executeInstall()` with multiple env vars merges all
- [ ] `executeInstall()` rejects invalid env var format (no = sign)
- [ ] `executeInstall()` reports success for each installed client
- [ ] `executeInstall()` reports errors for permission/write failures
- [ ] `executeInstall()` creates config file if it doesn't exist (file-type clients)
- [ ] `executeInstall()` idempotent: running twice produces same result
- [ ] npm `--install` reports bundled skill install summary (created/updated/unchanged/skipped/errors)
- [ ] PyPI `--install` reports bundled skill install summary with same semantics
- [ ] `KABOOM_SKIP_SKILL_INSTALL=1` skips skill installation in both wrappers

#### Unit Tests: Bundled Skills

- [ ] PyPI `install_bundled_skills()` writes managed skills to codex/claude/gemini roots
- [ ] Existing user-owned skill files are not overwritten
- [ ] Legacy managed `kaboom-*`, `gasoline-*`, and `strum-*` skill IDs are removed on successful install
- [ ] Packaged `skills.json` and `skills/*/SKILL.md` are present in wheel/sdist

#### Unit Tests: Doctor Flow

- [ ] `runDiagnostics()` checks all 5 client definitions
- [ ] `runDiagnostics()` reports "ok" for valid config with `kaboom-browser-devtools` entry
- [ ] `runDiagnostics()` reports "error" for invalid JSON
- [ ] `runDiagnostics()` reports "error" for missing `kaboom-browser-devtools` entry
- [ ] `runDiagnostics()` reports "info" for undetected clients
- [ ] `runDiagnostics()` handles CLI-type clients via subprocess check
- [ ] `runDiagnostics()` detects legacy paths and warns about orphaned configs
- [ ] `runDiagnostics()` provides recovery suggestions for each issue
- [ ] Doctor output is human-readable with ✅/❌/⚠️/⚪ symbols

#### Unit Tests: Uninstall Flow

- [ ] `executeUninstall()` with `dryRun=true` shows what would be removed
- [ ] `executeUninstall()` removes Kaboom-managed entries cleanly
- [ ] `executeUninstall()` preserves other MCP servers
- [ ] `executeUninstall()` handles Kaboom not being in config gracefully
- [ ] `executeUninstall()` reports count of clients from which Kaboom was removed
- [ ] `uninstallViaCli()` runs correct subprocess for CLI-type clients
- [ ] Uninstall produces valid JSON in remaining config

#### Unit Tests: CLI Argument Parsing

- [ ] `--dry-run` flag recognized for install/uninstall
- [ ] `--env KEY=VALUE` parsed correctly; multiple `--env` supported
- [ ] `--verbose` flag enables debug logging
- [ ] Invalid flag combinations rejected with helpful error
- [ ] Help text updated with all new commands

#### Unit Tests: Error Handling

- [ ] Permission denied on file write shows recovery suggestion
- [ ] Invalid JSON in existing config shows line number and suggestion
- [ ] Binary not found shows installation instructions
- [ ] Directory doesn't exist creates parent directories
- [ ] File locked/in-use shows helpful error message
- [ ] All errors include next-step suggestions

---

### Integration Tests

#### Install + Doctor Integration

- [ ] Run `--install` then `--doctor` shows Kaboom as configured for all detected clients
- [ ] Run `--install --env VAR=val` then read config file shows env object

#### Uninstall + Doctor Integration

- [ ] Run `--install`, then `--uninstall`, then `--doctor` shows unconfigured
- [ ] Uninstall preserves other MCP servers

#### Dry-Run Verification

- [ ] `--install --dry-run` followed by `--install` without dry-run produces same result
- [ ] `--uninstall --dry-run` shows same entries that actual `--uninstall` removes
- [ ] `--install --dry-run` does not mutate skill directories

#### Manual/Local Build Verification

- [ ] `./scripts/setup/install-bundled-skills.sh` installs managed skills from source tree
- [ ] Script respects `KABOOM_SKILL_SCOPE` (`global`, `project`, `all`)
- [ ] Script respects target overrides (`KABOOM_SKILL_TARGETS`, `KABOOM_CODEX_SKILLS_DIR`, etc.)

#### File System Scenarios

- [ ] Works when config files don't exist (creates them)
- [ ] Works when config files exist but are empty
- [ ] Works when config files have other MCP servers
- [ ] Works with both absolute and relative home paths
- [ ] Handles symlinks in config directories

---

### Security/Compliance Testing

#### Data Leak Tests

- [ ] Config files contain only Kaboom-managed entry and env vars
- [ ] Auth headers/tokens not captured in config
- [ ] User home path not exposed in error messages

#### Permission Tests

- [ ] Cannot write to read-only config directory (error message)
- [ ] Config file has appropriate read permissions for user
- [ ] Respects umask when creating new files
- [ ] Works with chmod 700 and chmod 755 homes

#### JSON Injection Tests

- [ ] Env var keys validated (no null bytes, control chars)
- [ ] Env var values safely escaped in JSON
- [ ] Config cannot be corrupted by malformed --env input
- [ ] Multiple writes don't cause data loss

#### Path Traversal Tests

- [ ] Cannot use `--env` to inject shell commands
- [ ] Cannot use paths to escape home directory
- [ ] symlink attacks mitigated with path.resolve()

---

## Human UAT Walkthrough

### Estimated Total Time: ~1 hour for all scenarios (46 minutes execution + 14 minutes setup/cleanup)

### Scenario 1: New User Safe Installation
#### Time: ~7 minutes

**Goal**: Verify a user can safely preview before installing.

**Setup**:
1. Backup any existing configs
2. Fresh Kaboom install (`npm install -g kaboom-agentic-browser@latest`)

**Steps**:
- [ ] Run `kaboom-agentic-browser --config`
  - **Expected**: Shows all 5 client definitions with detection status
  - **Verification**: Detected clients marked, correct paths shown

- [ ] Run `kaboom-agentic-browser --install --dry-run`
  - **Expected**: Shows what would be installed for each detected client
  - **Verification**: No actual files created or CLI commands run

- [ ] Run `kaboom-agentic-browser --install`
  - **Expected**: Shows success for each detected client
  - **Verification**: Config files created / CLI commands executed

- [ ] Run `kaboom-agentic-browser --doctor`
  - **Expected**: All installed clients show as configured
  - **Verification**: Correct status for each client

**Result**: ✅ PASS (user can preview before committing)

---

### Scenario 2: Multi-Client Installation
#### Time: ~8 minutes

**Goal**: Verify `--install` auto-detects and installs to all clients.

**Setup**:
1. Ensure at least 2 clients are detected (e.g., Cursor dir exists, `claude` CLI on PATH)
2. Create existing config with other MCP servers for one file-type client

**Steps**:
- [ ] Run `kaboom-agentic-browser --install`
  - **Expected**: Shows success for each detected client (CLI-type and file-type)
  - **Verification**: Each client's config modified/created as expected

- [ ] Read each config file and verify:
  - `kaboom-browser-devtools` entry present
  - Other MCP servers preserved
  - Valid JSON syntax

- [ ] For CLI-type (Claude Code): verify via `claude mcp list`
  - **Expected**: `kaboom-browser-devtools` entry present

**Result**: ✅ PASS (multi-client install works)

---

### Scenario 3: Environment Variables
#### Time: ~6 minutes

**Goal**: Verify env vars properly injected into config.

**Setup**:
1. Fresh config (uninstall previous)

**Steps**:
- [ ] Run:
  ```bash
  kaboom-agentic-browser --install --env KABOOM_SERVER=http://custom:7890 --env DEBUG=1
  ```
  - **Expected**: Success for all detected clients
  - **Verification**: Config files contain env vars

- [ ] Read a file-type client config and verify env section:
  - **Expected**:
    ```json
    "env": {
      "KABOOM_SERVER": "http://custom:7890",
      "DEBUG": "1"
    }
    ```
  - **Verification**: Both env vars present and properly formatted

- [ ] Run again with different env var:
  ```bash
  kaboom-mcp --install --env LOG_LEVEL=debug
  ```
  - **Expected**: Merged with existing env vars (not replaced)
  - **Verification**: All 3 env vars still present (`KABOOM_SERVER`, `DEBUG`, `LOG_LEVEL`)

**Result**: ✅ PASS (env vars work correctly)

---

### Scenario 4: Doctor Diagnostics
#### Time: ~10 minutes

**Goal**: Verify --doctor provides useful diagnostics.

**Setup**:
1. Create test scenario: some clients configured, some not
   - Claude Code: `claude` on PATH, Kaboom installed via CLI
   - Cursor: `~/.cursor/mcp.json` - valid with Kaboom
   - Windsurf: `~/.codeium/windsurf/mcp_config.json` - invalid JSON
   - Claude Desktop / VS Code: not installed

**Steps**:
- [ ] Run `kaboom-agentic-browser --doctor`
  - **Expected**:
    ```
    Kaboom Agentic Browser Diagnostic Report

    ✅ Claude Code
       Configured via CLI

    ✅ Cursor
       ~/.cursor/mcp.json - configured and healthy

    ❌ Windsurf
       ~/.codeium/windsurf/mcp_config.json - invalid JSON
       Suggestion: Fix syntax error or run: kaboom-agentic-browser --install

    ⚪ Claude Desktop
       Not detected

    ⚪ VS Code
       Not detected

    Summary: 2 clients healthy, 1 needs repair, 2 not detected
    ```
  - **Verification**: Correct status for each client; suggestions are actionable

- [ ] Follow one suggestion: fix Windsurf JSON and re-run doctor
  - **Expected**: Windsurf status changes to ✅
  - **Verification**: Doctor properly detects repair

**Result**: ✅ PASS (doctor provides useful diagnostics)

---

### Scenario 5: Uninstall
#### Time: ~5 minutes

**Goal**: Verify clean uninstall without losing other MCP servers.

**Setup**:
1. Install Kaboom to at least one file-type client with other MCP servers present

**Steps**:
- [ ] Run `kaboom-agentic-browser --uninstall --dry-run`
  - **Expected**: Shows which clients Kaboom would be removed from
  - **Verification**: other MCP server entries shown as preserved

- [ ] Run `kaboom-agentic-browser --uninstall`
  - **Expected**: "Removed from N clients"
  - **Verification**: Kaboom-managed entries removed from all clients

- [ ] Read a file-type config file:
  - **Expected**: other MCP server entries still present; Kaboom gone
  - **Verification**: Config valid JSON with only other servers

- [ ] Run `kaboom-agentic-browser --doctor`
  - **Expected**: Shows unconfigured for all clients
  - **Verification**: Doctor confirms removal

**Result**: ✅ PASS (uninstall works cleanly)

---

### Scenario 6: Error Recovery - Invalid JSON
#### Time: ~7 minutes

**Goal**: Verify helpful error when config has invalid JSON.

**Setup**:
1. Create broken config for a file-type client (e.g., `~/.cursor/mcp.json`) with missing quote

**Steps**:
- [ ] Run `kaboom-agentic-browser --install`
  - **Expected**: Error for that client, success for other detected clients
  - **Verification**: Error message clear and actionable, includes recovery suggestions

- [ ] Fix the broken JSON file
- [ ] Run `kaboom-agentic-browser --install` again
  - **Expected**: Now succeeds for all clients
  - **Verification**: Error recovery works

**Result**: ✅ PASS (error messages are helpful)

---

### Scenario 7: Backward Compatibility
#### Time: ~5 minutes

**Goal**: Verify existing commands still work.

**Setup**:
1. Clean state

**Steps**:
- [ ] Run `kaboom-agentic-browser --config`
  - **Expected**: Shows all 5 client definitions with detection status
  - **Verification**: Command works without errors

- [ ] Run `kaboom-agentic-browser --install`
  - **Expected**: Installs to all detected clients (new default behavior)
  - **Verification**: All detected clients configured

- [ ] Run `kaboom-agentic-browser --help`
  - **Expected**: Help shows all commands including new features
  - **Verification**: 5 supported clients listed

**Result**: ✅ PASS (backward compatible)

---

## Regression Testing

### Backward Compatibility (v5.2 → v5.3)

#### Critical: Verify all v5.2 CLI commands still work unchanged

- [ ] `kaboom-agentic-browser --config`
  - Expected: Shows all 5 client definitions with detection status
  - Verification: Correct paths, detection status

- [ ] `kaboom-agentic-browser --install` (without flags)
  - Expected: Installs to ALL detected clients (new default behavior)
  - Verification: All detected clients configured

- [ ] `kaboom-agentic-browser --help`
  - Expected: Lists all commands (old + new)
  - Verification: Shows --config, --install, --help, --doctor, --uninstall, --env, --dry-run, --verbose

- [ ] Binary path resolution
  - Expected: the `sh` exec shim resolves via `KABOOM_BINARY_PATH`, source-tree
    `dist/`, then the platform optionalDependency — never via PATH
  - Verification: `kaboom-agentic-browser` finds the binary on macOS/Linux/Windows,
    and the process the client spawned IS the Go binary (no launcher in between)

- [ ] Missing platform package fails loudly
  - Expected: a JSON-RPC error naming the missing optionalDependency and the
    repair command; no silent PATH fallback to another kaboom on the machine
  - Verification: remove `node_modules/@brennhill/kaboom-agentic-browser-*` and run

- [ ] Unsupported platform error
  - Expected: Shows clear error for unsupported OS (unchanged)
  - Verification: Error message format and text unchanged from v5.2

- [ ] Other npm packages install correctly
  - Expected: NPM ecosystem not affected by `kaboom-agentic-browser` changes
  - Verification: `npm install` and `npm ci` work normally

### Concurrent Operations

- [ ] Two simultaneous `kaboom-agentic-browser --install` processes
  - Setup: Start two install processes to same config file in parallel
  - Expected: Both complete without corruption
  - Verification: Config file has valid JSON and `kaboom-browser-devtools` entry present
  - Implementation: Use atomic writes (temp file + rename) to prevent race conditions

### Integration with Kaboom Binary

- [ ] `kaboom-browser-devtools` entry in config correctly points to binary
- [ ] Binary path resolution works across macOS, Linux, Windows paths
- [ ] Extension still loads MCP correctly with new config format
- [ ] Kaboom binary can be invoked from config (test with `--version`)

---

## Performance/Load Testing

- [ ] Doctor completes in < 1 second (5 clients)
- [ ] Install completes in < 1 second (all detected clients)
- [ ] Doctor with --verbose doesn't add significant time
- [ ] Uninstall completes in < 1 second (all detected clients)

---

## Test Files

### npm (`npm/kaboom-agentic-browser/lib/`)
- `config.test.js` — Client registry, detection, path resolution (23 tests)
- `install.test.js` — Install flow for CLI + file-type clients (11 tests)
- `uninstall.test.js` — Uninstall flow for CLI + file-type clients (7 tests)

### Python (`pypi/kaboom-agentic-browser/tests/`)
- `test_config.py` — Client registry, detection, path resolution (23 tests)
- `test_install.py` — Install flow (9 tests)
- `test_uninstall.py` — Uninstall flow (6 tests)

### Shared (`tests/cli/`)
- `errors.test.cjs` — Error classes and formatting
