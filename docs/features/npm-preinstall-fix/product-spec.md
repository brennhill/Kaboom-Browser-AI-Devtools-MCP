---
doc_type: product-spec
feature_id: npm-preinstall-fix
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: npm Preinstall Auto-Uninstall

## Problem

Users report that after running `npm install -g kaboom-mcp`, the old version continues to run until they manually uninstall with `npm uninstall -g kaboom-mcp`.

**Root cause**: npm's global package cache keeps the old binary in PATH even after new install completes.

## User Stories

1. **As a user upgrading kaboom-mcp**, I want `npm install -g` to automatically remove the old version, so I don't have to manually uninstall first.

2. **As a user**, I want a single command to upgrade to the latest version, without multi-step workarounds.

## Requirements

### Functional

1. **Auto-Uninstall**
   - Run `npm uninstall -g kaboom-mcp` before installing new version
   - Execute via npm `preinstall` hook
   - Must be silent (no error output if old version doesn't exist)
   - Must not block install if uninstall fails

2. **Installation Flow**
   - User runs: `npm install -g kaboom-mcp@latest`
   - npm runs: `preinstall` script → uninstalls old version
   - npm continues: installs new version
   - Result: Only new version in PATH

3. **First-Time Install**
   - Script must handle case where no previous version exists
   - Should not fail or warn when nothing to uninstall
   - Should complete silently

### Non-Functional

- Uninstall must complete in < 5 seconds
- Must work on all platforms (macOS, Linux, Windows)
- Must not interfere with CI/CD pipelines
- Must not create orphaned files or directories

## Edge Cases

1. **No previous install**: Script exits gracefully (no error)
2. **Multiple versions installed**: npm handles cleanup
3. **Permission denied**: Install continues (logged but not fatal)
4. **Network timeout during uninstall**: Install continues
5. **User has kaboom-mcp locally (not global)**: Local version unaffected

## Implementation

```json
{
  "scripts": {
    "preinstall": "node -e \"try{require('child_process').execSync('npm uninstall -g kaboom-mcp',{stdio:'ignore'})}catch(e){}\""
  }
}
```

### Why this works:
- `preinstall` hook runs before package extraction
- `execSync` blocks until uninstall completes
- `{stdio:'ignore'}` suppresses all output
- `try/catch` prevents errors from failing install
- Single-line Node.js avoids platform-specific shell scripts

## Out of Scope

- Migrating user settings from old version
- Backing up old binaries
- Prompting user before uninstall
- Uninstalling other kaboom packages (kaboom-cli, etc.)

## Success Metrics

- Zero user reports of "old version still running after install"
- Install time increase < 5 seconds
- Works on all supported platforms

## Testing

1. **Fresh install**: No errors, completes normally
2. **Upgrade from 5.6.0 → 5.7.0**: Old version removed, new version works
3. **Reinstall same version**: No errors, binary still works
4. **Install without network**: Continues even if uninstall fails
5. **CI pipeline**: No permission errors or unexpected failures

## Dependencies

- npm preinstall hook support (npm 5.0+)
- Node.js child_process module (always available)
- npm in PATH (always true during install)

## Status

✅ **Implemented** - Commit `498c741`
