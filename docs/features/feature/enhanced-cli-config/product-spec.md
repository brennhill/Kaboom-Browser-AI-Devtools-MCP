---
feature: Enhanced CLI Configuration Management
status: proposed
tool: configure
mode: cli
version: 0.8.1
doc_type: product-spec
feature_id: feature-enhanced-cli-config
last_reviewed: 2026-07-05
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Product Spec: Enhanced CLI Configuration Management

## Problem Statement

Users installing Kaboom Agentic Browser (via NPM or PyPI) need to:
1. Configure it across multiple AI assistant clients (Claude Code, Claude Desktop, VS Code, Cursor, Windsurf)
2. Verify the configuration is correct before committing changes
3. Diagnose connection/installation issues when things go wrong
4. Manage environment variables for different deployment scenarios
5. Uninstall Kaboom cleanly if needed

The current implementation (`--config`, `--install`) is functional but incomplete in both wrappers:
- **No preview mode**: Users can't see what changes will be made before committing
- **No diagnostics**: When installation fails, users have no way to troubleshoot
- **Single-client support**: Only first matching config file gets updated; other clients left out
- **No verification**: No way to confirm the config is actually working
- **No environment variables**: Can't inject custom env vars (e.g., API keys, server URLs) into MCP config
- **Limited error recovery**: Confusing error messages when JSON is malformed or permissions denied
- **Skill install mismatch**: Bundled skill installation behavior differs across npm, PyPI, and manual source builds

## Solution

Enhance the kaboom-agentic-browser CLI with four new features:

### 1. **Dry-Run Mode** (`--dry-run`)
Preview exactly what changes will be made without actually writing files. Shows:
- Which config file would be modified/created
- Before/after JSON diff
- Estimated file permissions
- Safe for checking before real installation

### 2. **Doctor Command** (`--doctor`)
Diagnostic tool that verifies local configuration:
- Checks all 5 client configs and their JSON validity
- Verifies the kaboom-browser-devtools entry is present in each config
- Confirms the Kaboom binary path resolves and is executable
- Tests basic binary invocation (runs `kaboom-agentic-browser --version`)
- Reports which clients are configured, which aren't
- Detects legacy config paths and warns about orphaned configs
- Provides actionable repair suggestions for fixable issues
- **Note**: Cannot test actual MCP connection without AI agent running (VS Code, Claude, etc.)

### 3. **Environment Variables** (`--install --env KEY=VALUE`)
Allow injecting environment variables into MCP config:
- `--env KABOOM_SERVER=http://localhost:7890`
- `--env DEBUG=1`
- Multiple `--env` flags supported
- Saved in `env` object of MCP config
- Useful for custom server URLs, debug modes, offline deployments

### 4. **Uninstall Command** (`--uninstall`)
Remove Kaboom from AI assistant configs:
- Find the kaboom-browser-devtools entry in all detected client configs
- Remove it cleanly (preserve other MCP servers)
- Confirm removal before writing
- Report which clients were cleaned up

### 5. **Bundled Skill Installation Parity**
Ensure bundled managed skills are installed consistently across all distribution channels:
- npm install: postinstall installs bundled skills
- `kaboom-agentic-browser --install` (npm + PyPI): installs MCP config and bundled skills
- manual/local source builds: supported via `scripts/setup/install-bundled-skills.sh`

## Requirements

1. **Usability**: All commands must have helpful output and clear error messages
2. **Safety**: Dry-run must work correctly; no actual changes on `--dry-run`
3. **Idempotence**: Multiple runs with same args produce same result
4. **Backward Compatible**: Existing `--config`, `--install`, `--help` must still work
5. **Composable**: `--dry-run` works with `--install`, `--uninstall`, `--doctor`
6. **Verbose**: `--verbose` flag shows detailed operation logs
7. **Recovery**: Commands provide next-step suggestions on failure
8. **Multi-client**: `--install` targets all detected clients by default
9. **Channel parity**: npm, PyPI, and manual/local install paths provide equivalent bundled-skill behavior

## Out of Scope

- GUI configuration tool (CLI only)
- Automatic config migration between versions
- Remote config management (local files only)
- Automatic version upgrades
- Configuration encryption/secrets management (users responsible for env vars)

## Success Criteria

1. ✅ `--dry-run` shows exact changes without writing files
2. ✅ `--doctor` diagnoses 90%+ of common config issues
3. ✅ `--install` auto-detects and installs to all 5 supported clients
4. ✅ `--env KEY=VALUE` properly saves env vars to config
5. ✅ `--uninstall` cleanly removes Kaboom-managed entries from all configs
6. ✅ All commands provide actionable error messages with recovery steps
7. ✅ 100% backward compatibility with v5.2.0 CLI behavior
8. ✅ All commands covered by automated tests (unit + integration)
9. ✅ Help text and examples available for each command
10. ✅ Skill install parity verified for npm, PyPI, and manual/local installer script

## User Workflows

### Workflow 1: Safe Installation
```
User wants to try Kaboom on their VS Code but isn't sure what will change

1. User runs:   kaboom-agentic-browser --config
2. Sees:        Detected clients + config status for each
3. User runs:   kaboom-agentic-browser --install --dry-run
4. Sees:        Exact JSON changes for each detected client
5. User runs:   kaboom-agentic-browser --install
6. Sees:        ✅ Installed to all detected clients
7. User runs:   kaboom-agentic-browser --doctor
8. Sees:        ✅ Kaboom configured in Claude Code, Cursor
                ⚪ Claude Desktop not detected
                ⚪ Windsurf not detected
```

### Workflow 2: Multi-Client Install
```
User wants Kaboom available in all AI assistants

1. User runs:   kaboom-agentic-browser --install
2. Sees:        ✅ Claude Code: Installed (via CLI)
                ✅ Claude Desktop: Updated
                ✅ Cursor: Updated
                ✅ Windsurf: Updated
                ✅ VS Code: Updated
3. User runs:   kaboom-agentic-browser --doctor
4. Sees:        ✅ All 5 clients configured
```

### Workflow 3: Environment Variables
```
User needs to point Kaboom at a custom server URL

1. User runs:   kaboom-agentic-browser --install --env KABOOM_SERVER=http://192.168.1.100:7890
2. Sees:        ✅ Installed to all detected clients
3. Config has:  "env": { "KABOOM_SERVER": "http://192.168.1.100:7890" }
```

### Workflow 4: Troubleshooting
```
User says "It's not working, can you check?"

1. User runs:   kaboom-agentic-browser --doctor
2. Sees:
                Kaboom Agentic Browser Diagnostic Report

                ✅ Claude Code
                   Configured via CLI

                ✅ Cursor
                   ~/.cursor/mcp.json - Configured and ready

                ⚠️  Windsurf
                   ~/.codeium/windsurf/mcp_config.json - Invalid JSON at line 5
                   Fix: Manually edit or restore from backup

                ⚪ Claude Desktop
                   Not detected

                ⚪ VS Code
                   Not detected

                ✅ Binary check
                   Kaboom binary found at /usr/local/bin/kaboom-agentic-browser

                Summary: 2 clients ready, 1 needs repair, 2 not detected

3. User gets:   Actionable next steps for each issue
```

### Workflow 5: Cleanup
```
User wants to remove Kaboom from all configs

1. User runs:   kaboom-agentic-browser --uninstall --dry-run
2. Sees:        Would remove Kaboom from:
                - Claude Code (via CLI)
                - ~/.cursor/mcp.json
3. User runs:   kaboom-agentic-browser --uninstall
4. Sees:        ✅ Removed from 2 clients
```

## Examples

### Basic Commands
```bash
# Show detected clients and config status
kaboom-agentic-browser --config

# Install to all detected clients
kaboom-agentic-browser --install
# Output:
# ✅ 5/5 clients installed:
#    ✅ Claude Code (via CLI)
#    ✅ Claude Desktop (at ~/Library/Application Support/Claude/claude_desktop_config.json)
#    ✅ Cursor (at ~/.cursor/mcp.json)
#    ✅ Windsurf (at ~/.codeium/windsurf/mcp_config.json)
#    ✅ VS Code (at ~/Library/Application Support/Code/User/mcp.json)

# Preview what --install would do (no files written)
kaboom-agentic-browser --install --dry-run
```

### Environment Variables
```bash
# Install with custom server URL
kaboom-agentic-browser --install --env KABOOM_SERVER=http://custom:7890

# Install with multiple env vars
kaboom-agentic-browser --install --env DEBUG=1 --env LOG_LEVEL=info

# ⚠️  SECURITY: Never store API keys in --env!
# WRONG: kaboom-agentic-browser --install --env API_KEY='sk-...'
# RIGHT: Store key in ~/.kaboom/secrets (mode 600) then:
#        kaboom-agentic-browser --install --env SECRETS_FILE=/home/user/.kaboom/secrets
```

### Diagnostics & Uninstall
```bash
# Run diagnostics (check all configs are valid)
kaboom-agentic-browser --doctor

# Uninstall from all configs (preserves other MCP servers)
kaboom-agentic-browser --uninstall

# Preview uninstall (no files written)
kaboom-agentic-browser --uninstall --dry-run

# Verbose output for all commands
kaboom-agentic-browser --install --verbose

# Help
kaboom-agentic-browser --help
```

### Error Scenarios
```bash
# Error: --env without --install
kaboom-agentic-browser --env DEBUG=1
# ❌ Error: --env only works with --install
#    Usage: kaboom-agentic-browser --install --env KEY=VALUE

# Error: Invalid env format
kaboom-agentic-browser --install --env BADFORMAT
# ❌ Error: Invalid env format "BADFORMAT". Expected: KEY=VALUE
#    Example: kaboom-agentic-browser --install --env KABOOM_SERVER=http://localhost:7890

# Error: Permission denied
kaboom-agentic-browser --install
# ❌ Error: Permission denied writing ~/.cursor/mcp.json
#    Try: sudo kaboom-agentic-browser --install
#    Or: Check permissions with: ls -la ~/.cursor/

# Error: Invalid JSON in config
kaboom-agentic-browser --install
# ❌ Error: Invalid JSON in ~/.cursor/mcp.json at line 15
#    Unexpected token }
#
#    Fix options:
#    1. Manually edit: code ~/.cursor/mcp.json
#    2. Restore from backup and try --install again
#    3. Run: kaboom-agentic-browser --doctor (for more info)
```

## Notes

- **Distribution**: Both NPM (`npm/kaboom-agentic-browser/`) and PyPI (`pypi/kaboom-agentic-browser/`) wrappers must be updated with feature parity
- **NPM Entry Point**: `npm/kaboom-agentic-browser/bin/kaboom-agentic-browser` (Node.js)
- **PyPI Entry Point**: `pypi/kaboom-agentic-browser/kaboom_agentic_browser/__main__.py` (Python)
- **Related**: `.claude/docs/architecture.md` (5-tool MCP constraint)
- **Building on**: v5.2.0 CLI foundation (`--config`, `--install`)
- **Test reference**: `tests/extension/` (use as model for new tests)
