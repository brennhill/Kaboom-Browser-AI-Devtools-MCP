---
feature: Enhanced CLI Configuration Management
status: proposed
doc_type: tech-spec
feature_id: feature-enhanced-cli-config
last_reviewed: 2026-07-05
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Tech Spec: Enhanced CLI Configuration Management

> Plain language only. Implementation guide for developers.

## Architecture Overview

The enhancement extends **both wrappers** with the same CLI commands:

### NPM Wrapper (`npm/kaboom-agentic-browser/bin/kaboom-agentic-browser`)
Node.js CLI entry point. Current structure:
- **Lines 32-68**: `findBinary()` - Locates the Kaboom binary on disk
- **Lines 70-81**: `generateMCPConfig()` - Builds MCP config object
- **Lines 83-102**: `showConfigCommand()` - Displays config template + locations
- **Lines 104-151**: `installCommand()` - Writes config to first matching location
- **Lines 153-169**: Command routing (--config, --install, --help)

### PyPI Wrapper (`pypi/kaboom-agentic-browser/kaboom_agentic_browser/__main__.py`)
Python CLI entry point. Current structure:
- **Lines 1-12**: Entry point that calls `run()` function
- Related: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py` has `get_binary_path()` and `run()` functions

### New additions (both wrappers)
- Shared utility functions for config file operations (JSON read/write/validate)
- Command handlers for: `--dry-run`, `--doctor`, `--uninstall`, `--env`
- Refactored install logic to support `--dry-run` and multi-client targeting
- New error handling with recovery suggestions
- Verbose logging support
- **Feature parity**: Both wrappers must behave identically

### Bundled Skill Installation Paths

- **npm**:
  - `npm/kaboom-agentic-browser/lib/postinstall-skills.js` installs bundled skills on package install.
  - `npm/kaboom-agentic-browser/lib/cli.js` invokes `installBundledSkills()` during `--install`.
- **PyPI**:
  - `pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py` invokes `install_bundled_skills()` during `--install`.
  - `pypi/kaboom-agentic-browser/kaboom_agentic_browser/skills.py` implements managed write/update/legacy cleanup behavior.
  - `pypi/kaboom-agentic-browser/pyproject.toml` includes bundled skill files as package data.
- **Manual/local source build**:
  - `scripts/install-bundled-skills.sh` installs bundled skills from `npm/kaboom-agentic-browser/skills` into target agent directories.

## Implementation: NPM vs. Python

### NPM (Node.js)
- **Language**: JavaScript (Node.js stdlib)
- **Location**: `npm/kaboom-agentic-browser/bin/kaboom-agentic-browser`
- **Modules**: `lib/config.js`, `lib/doctor.js`, `lib/install.js`, `lib/uninstall.js`, `lib/output.js`, `lib/errors.js`
- **File I/O**: `fs` module
- **JSON**: Built-in `JSON.parse()` and `JSON.stringify()`

### Python
- **Language**: Python 3 (stdlib only)
- **Location**: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/__main__.py` (entry point) + new module files
- **Modules**:
  - `pypi/kaboom-agentic-browser/kaboom_agentic_browser/config.py` (config utilities)
  - `pypi/kaboom-agentic-browser/kaboom_agentic_browser/doctor.py` (diagnostics)
  - `pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py` (install logic)
  - `pypi/kaboom-agentic-browser/kaboom_agentic_browser/uninstall.py` (uninstall logic)
  - `pypi/kaboom-agentic-browser/kaboom_agentic_browser/output.py` (formatters)
  - `pypi/kaboom-agentic-browser/kaboom_agentic_browser/errors.py` (error classes)
- **File I/O**: `pathlib.Path` or `os`/`open()`
- **JSON**: Built-in `json` module
- **Entry Point**: Updated `__main__.py` to parse CLI args and route commands

### Feature Parity Requirement
Both wrappers must:
- Accept same command flags (`--config`, `--install`, `--doctor`, `--uninstall`, `--dry-run`, `--env`, `--verbose`, `--help`)
- Produce identical output (same messages, same emojis, same error text)
- Behave identically (same logic, same edge case handling)
- Use same config file locations (hardcoded paths, not user input)
- Apply equivalent bundled skill installation semantics during `--install`

---

## Key Components

### 1. **Client Registry & Config Utilities**
- `CLIENT_DEFINITIONS`: Structured registry of all 5 supported clients (Claude Code, Claude Desktop, Cursor, Windsurf, VS Code)
- `getDetectedClients()`: Returns array of clients whose config dir exists or CLI is on PATH
- `getClientConfigPath(def)`: Returns platform-specific config path for a client
- `isClientInstalled(def)`: Checks if client is present on the system
- `readConfigFile(path)`: Safely reads and parses JSON, returns `{valid: bool, data: obj, error: string}`
- `writeConfigFile(path, data, dryRun)`: Writes JSON to file; if `dryRun=true`, returns what would be written without actually writing
- `validateMCPConfig(data)`: Ensures config has `mcpServers` object
- `mergeKaboomConfig(existing, kaboomEntry, envVars)`: Merges the canonical Kaboom entry into existing `mcpServers` and strips legacy `kaboom-*`, `gasoline-*`, and `strum-*` keys

### 2. **Diagnostic Engine**
- `runDiagnostics()`: Iterates through all client definitions, tests each one:
  - For file-type clients: config file exists, JSON valid, Kaboom entry present, and legacy aliases are flagged
  - For CLI-type clients: runs `claude mcp get kaboom-browser-devtools` to check status
  - Detects legacy config paths (orphaned configs at old incorrect locations)
  - Returns report object with health status for each client

### 3. **Installation Engine (Refactored)**
- `executeInstall(options)`: Main install logic supporting:
  - `{dryRun: bool}` - Preview mode
  - `{envVars: {KEY: VALUE}}` - Environment variables to inject
  - Installs to all detected clients by default (file-type via config write, CLI-type via subprocess)
  - Claude Code: uses `claude mcp add-json --scope user kaboom-browser-devtools` subprocess (unsets `CLAUDECODE` env var)
  - Returns `{success: bool, installed: [{name, method, path}], errors: [details]}`

### 4. **Uninstall Engine**
- `executeUninstall(options)`: Removes Kaboom and legacy managed aliases from configs:
  - `{dryRun: bool}` - Preview mode
  - Iterates all detected clients
  - For file-type: removes Kaboom-managed entries from `mcpServers`, preserves other servers
  - For CLI-type: runs `claude mcp remove --scope user kaboom-browser-devtools`
  - Returns `{removed: [{name, method, path}], notConfigured, errors}`

### 5. **Output Formatters**
- `formatConfigDiff(before, after)`: Shows JSON before/after for dry-run
- `formatDiagnosticsReport(report)`: Pretty-prints doctor output with ✅/❌/⚠️  symbols
- `formatInstallResult(result)`: Shows which tools updated, any errors
- `formatErrorWithRecovery(error, context)`: Error message + next steps

## Data Flows

### Install Flow (with --dry-run support)
```
Parse CLI args (--install, --dry-run, --env)
  ↓
Build install options object {dryRun, envVars}
  ↓
Execute install:
  - Get detected clients (CLIENT_DEFINITIONS filtered by isClientInstalled)
  - For each detected client:
    - If CLI-type: run subprocess (e.g., `claude mcp add-json --scope user kaboom-browser-devtools`)
    - If file-type: read existing config, merge the Kaboom entry + env vars, write
    - If dryRun: show what would be written, don't actually execute
  ↓
Report results (clients installed, errors)
```

### Doctor Flow
```
Parse CLI args (--doctor, --verbose)
  ↓
For each client in CLIENT_DEFINITIONS:
  - If file-type: check config file exists, validate JSON, verify the Kaboom entry, and flag legacy aliases
  - If CLI-type: run subprocess to check status (e.g., `claude mcp get kaboom-browser-devtools`)
  - If not detected: report as "not detected" (info)
  ↓
Check legacy paths for orphaned configs
  ↓
Compile diagnostic report:
  - Client name → health status
  - Issues found → recovery suggestions
  - Legacy warnings
  ↓
Format and display report
```

### Uninstall Flow
```
Parse CLI args (--uninstall, --dry-run)
  ↓
Execute uninstall:
  - Get detected clients
  - For each detected client:
    - If CLI-type: run remove subprocess
    - If file-type: read config, remove Kaboom-managed entries, write back
    - If dryRun: show what would be removed
  ↓
Report results (clients removed, errors)
```

## Implementation Strategy

### Phase 1: Refactor Current Code
1. Extract `generateMCPConfig()` to include `envVars` parameter
2. Extract config file I/O into `readConfigFile()` and `writeConfigFile()` utilities
3. Update `installCommand()` to call new utilities (no behavior change, just refactor)
4. Add `--verbose` flag support to existing commands

### Phase 2: Implement --dry-run
1. Add `--dry-run` flag parsing to CLI args
2. Pass `dryRun: true` through install flow
3. Implement `writeConfigFile(..., dryRun=true)` to return diff without writing
4. Display JSON diff using simple before/after format

### Phase 3: Implement --doctor
1. Create `runDiagnostics()` function checking each candidate
2. Create diagnostic report object: `{tool: string, status: 'ok'|'error'|'warn', issues: [], suggestions: []}`
3. For each candidate, run checks:
   - `fs.existsSync(path)` for file existence
   - `JSON.parse()` for JSON validity
   - Check `data.mcpServers["kaboom-browser-devtools"]` exists
   - Check binary path exists and is executable
4. Format output with emojis (✅/❌/⚠️) and recovery suggestions

### Phase 4: Implement Client Registry
1. Replace hardcoded path list with `CLIENT_DEFINITIONS` registry
2. Support CLI-type (Claude Code via subprocess) and file-type clients
3. Install to all detected clients by default
4. Report all successful installs and any errors

### Phase 5: Implement --env
1. Parse `--env KEY=VALUE` arguments into array
2. Validate format (contains =, non-empty key and value)
3. Merge into config's `env` object
4. Support multiple `--env` flags

### Phase 6: Implement --uninstall
1. Create `executeUninstall()` function
2. For each candidate:
   - Read config
   - Delete `data.mcpServers["kaboom-browser-devtools"]` and any legacy aliases during migration
   - Write back (if changed)
3. Report removed count and tool names

### Phase 7: Improve Error Messages
1. Wrap all file operations in try-catch
2. For each error type, provide context and next steps:
   - **Permission denied** → "Run with `sudo` or check file permissions"
   - **Invalid JSON** → "Config has syntax error at line X, use `--doctor` to see issues"
   - **Binary not found** → "Reinstall: `npm install -g kaboom-agentic-browser`"
3. Test error paths with deliberately broken configs

## Edge Cases & Assumptions

| Edge Case | Handling |
|-----------|----------|
| **No clients detected** | `--install` reports no clients found; `--doctor` reports all as "not detected" |
| **Config file has invalid JSON** | `--doctor` reports syntax error; `--install` refuses to write (don't corrupt) |
| **User lacks file permissions** | Error message with `sudo` suggestion; `--doctor` reports permission issue |
| **Kaboom binary not found** | All commands report binary path issue; suggest reinstall |
| **File partially written (crash)** | `--dry-run` prevents this; next `--install` attempt merges cleanly |
| **Multiple legacy/managed entries** | Rewrite to one canonical `kaboom-browser-devtools` entry and warn during doctor |
| **Other MCP servers in config** | Preserve them; only touch Kaboom-managed entries |
| **Config file deleted during operation** | No-op, file doesn't exist; report in results |
| **User runs --env without --install** | Show error: "--env only works with --install" |

**Assumptions**:
- Config files are valid JSON when user didn't edit them
- User's home directory is writable (or at least one config location is)
- Kaboom binary is installed and on PATH or findable via node_modules
- AI clients (Claude Code, VS Code, etc.) may not be running when --doctor executes

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| **Corrupt config on write failure** | Always `--dry-run` first; use atomic writes (write temp file, rename) |
| **Merge conflicts between Kaboom + user edits** | Read existing config, merge the Kaboom entry into `mcpServers`, preserve other keys |
| **User loses other MCP server configs** | Test with multi-server configs; never overwrite entire `mcpServers` |
| **--doctor false negatives** | Can only verify local state; cannot test actual MCP connection. Document limitation. |
| **--uninstall removes wrong data** | Test with configs having multiple MCP servers; verify only Kaboom-managed entries are removed |
| **Breaking change from v5.2 CLI** | Keep v5.2 commands working: --config, --install, --help all unchanged |

## Dependencies

- **Existing**: Node.js `fs`, `path`, `os` modules (already used)
- **Existing**: `execFileSync` for binary lookup (already used)
- **New**: `JSON.stringify(..., null, 2)` for formatting (already used)
- **New**: No new npm dependencies; stdlib only

## Platform-Specific Behavior

### Windows Path Handling
- Client registry uses `expandPath()` which resolves `~` via `os.homedir()` and `%APPDATA%` via `process.env`
- `path.join()` handles platform-specific separators (\ on Windows, / on Unix)
- Platform-specific paths defined per client in `CLIENT_DEFINITIONS` (e.g., `darwin`, `win32`, `linux`, or `all`)

### Symlink Behavior
- **Policy**: Follow symlinks and update the target file (standard Unix behavior)
- Implementation: Use `path.resolve()` to get canonical path and prevent traversal attacks
- Documented for users: "Config files will be followed if symlinked"
- Test: Verify behavior with symlinked config directories

## Performance Considerations

- `readConfigFile()`: O(n) where n = config file size (typically < 10KB); acceptable
- `runDiagnostics()`: O(m) where m = number of client definitions (5 clients); < 100ms
- `--install`: O(m) for all detected clients; < 500ms total
- **Critical**: `--dry-run` must not write any files (test with file system mock)
- **File I/O**: Single I/O per file; no loops or polling

## Security Considerations

1. **JSON Injection**: Validate env var keys/values before merging into config
   - Reject keys with null bytes or control characters
   - Reject values if they contain shell metacharacters (document that users are responsible)

2. **File Permissions**:
   - Check file readable before reading
   - Check directory writable before writing
   - Don't change ownership or permissions
   - Respect umask when creating new files

3. **Path Traversal**:
   - All paths come from hardcoded candidate list (no user input for paths)
   - Use `path.resolve()` and `path.normalize()` to avoid symlink attacks

4. **Env Var Injection**:
   - Document that env vars are passed as-is to the Kaboom binary
   - Users responsible for safe values (no shell injection from our side)
   - Config stored in JSON; not vulnerable to YAML or shell parsing
   - **Caution**: Never store API keys or secrets in --env
   - Example: Store API key in `~/.kaboom/secrets` (mode 600), pass only path with `--env SECRETS_FILE=...`

5. **File Size Limits**:
   - Limit config file size to 1MB (prevents DoS from crafted configs)
   - Typical MCP config is < 1KB; 1MB is generous safety margin
   - Implementation: Check `if (stats.size > 1024 * 1024) throw new Error(...)`

6. **Private Keys**:
   - Config files stored in user's home directory (standard)
   - Warn in help text: don't store secrets in env vars; use config file permissions instead
   - Example: Store API key in `~/.kaboom/config` instead of in `--env`

7. **Audit**:
   - Log all file modifications with timestamps when `--verbose`
   - Include diffs for transparency

## Data Model

```
Install Options:
{
  dryRun: boolean,
  envVars: {KEY: VALUE, ...},
  verbose: boolean
}

Install Result:
{
  success: boolean,
  installed: [{name: "Cursor", method: "file", path: "~/.cursor/mcp.json"}, ...],
  total: number,
  errors: [{name: "...", error: "..."}]
}

Diagnostic Report:
{
  clients: [
    {
      name: "Claude Code",
      type: "cli",
      status: "ok" | "error" | "warning" | "info",
      issues: ["kaboom-browser-devtools entry missing", ...],
      suggestions: ["Run: kaboom-agentic-browser --install", ...]
    },
    ...
  ],
  legacyWarnings: ["Found orphaned config at ~/.codeium/mcp.json", ...],
  summary: "2 clients configured, 1 needs repair, 2 not detected"
}

Config Structure:
{
  mcpServers: {
    "kaboom-browser-devtools": {
      command: "kaboom-agentic-browser",
      args: [],
      env: {
        KEY: "VALUE",
        ...
      }
    },
    otherTool: { ... }  // preserve
  }
}
```
