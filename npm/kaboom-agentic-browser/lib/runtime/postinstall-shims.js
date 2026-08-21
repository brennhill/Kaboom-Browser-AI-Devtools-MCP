// postinstall-shims.js — Point the Windows command shims at our batch launchers.
// Purpose: On win32, replace npm's generated .bin shims for kaboom-agentic-browser
//   and kaboom-hooks so they invoke bin/*.cmd directly.
// Why: The bin entries are POSIX `sh` exec shims — that is what removes the extra
//   launcher process on macOS/Linux. npm's Windows shim reads that `#!/bin/sh`
//   shebang and routes through `sh`, which only exists when Git Bash is installed.
//   Rewriting the shim keeps Windows working with no `sh` dependency and no Node
//   runtime between the client and the Go binary.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const path = require('path');
const fs = require('fs');

const COMMANDS = ['kaboom-agentic-browser', 'kaboom-hooks'];

// Forwarders live in node_modules/.bin, one level above the package directory.
function cmdForwarder(pkgDirName, command) {
  return [
    '@echo off',
    `"%~dp0..\\${pkgDirName}\\bin\\${command}.cmd" %*`,
    'exit /b %ERRORLEVEL%',
    '',
  ].join('\r\n');
}

function ps1Forwarder(pkgDirName, command) {
  return [
    '#!/usr/bin/env pwsh',
    `& "$PSScriptRoot/../${pkgDirName}/bin/${command}.cmd" @args`,
    'exit $LASTEXITCODE',
    '',
  ].join('\r\n');
}

/**
 * @returns {{skipped: boolean, reason?: string, written: string[], failures: Array<{file: string, error: string}>}}
 */
function wireWindowsShims({
  platform = process.platform,
  packageRoot = path.resolve(__dirname, '..', '..'),
  fsImpl = fs,
} = {}) {
  const result = { skipped: false, written: [], failures: [] };

  if (platform !== 'win32') {
    // EXPECTED_ABSENCE: POSIX uses the sh exec shim directly; there is nothing to rewire.
    return { ...result, skipped: true, reason: 'not win32' };
  }

  const binDir = path.resolve(packageRoot, '..', '.bin');
  if (!fsImpl.existsSync(binDir)) {
    // EXPECTED_ABSENCE: a global install or a bare tarball extract has no .bin
    // directory; npm did not create shims, so there are none to correct.
    return { ...result, skipped: true, reason: `no .bin directory at ${binDir}` };
  }

  const pkgDirName = path.basename(packageRoot);

  for (const command of COMMANDS) {
    for (const [file, body] of [
      [`${command}.cmd`, cmdForwarder(pkgDirName, command)],
      [`${command}.ps1`, ps1Forwarder(pkgDirName, command)],
    ]) {
      const target = path.join(binDir, file);
      try {
        fsImpl.writeFileSync(target, body);
        result.written.push(target);
      } catch (e) {
        // Never fail the install, but never swallow it either: a stale shim means
        // the CLI silently depends on `sh` being present.
        result.failures.push({ file: target, error: e.message });
      }
    }
  }

  return result;
}

module.exports = { COMMANDS, cmdForwarder, ps1Forwarder, wireWindowsShims };

// Run as an npm postinstall step. Never fails the install, but never hides a
// failure either — a stale shim silently reintroduces the `sh` dependency.
if (require.main === module) {
  const result = wireWindowsShims();
  if (result.failures.length > 0) {
    console.error(
      `[kaboom] Could not update ${result.failures.length} Windows command shim(s). ` +
        'The kaboom-agentic-browser CLI will fall back to npm\'s shim, which requires `sh` (Git Bash) on PATH. ' +
        result.failures.map((f) => `${f.file}: ${f.error}`).join('; ')
    );
  }
}
