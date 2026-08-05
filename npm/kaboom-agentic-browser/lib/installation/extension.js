// Purpose: Resolve where the unpacked browser extension lives and reveal it in
// the desktop file manager so "Load unpacked" is one selection away.
// Why: npm bundles the extension inside this package; the curl|sh installer
// stages it to ~/KaboomAgenticDevtoolExtension. Either way the user must load an
// EXACT folder, and opening it for them removes the most-missed onboarding step.

'use strict';

const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');
const { spawn } = require('node:child_process');
const { isEnvFlagSet } = require('../config/config');

// The folder the curl|sh / PowerShell installers stage the extension into.
const STAGED_DIR_NAME = 'KaboomAgenticDevtoolExtension';

/** True when dir looks like an unpacked extension (has a manifest.json). */
function isExtensionDir(dir) {
  try {
    return !!dir && fs.existsSync(path.join(dir, 'manifest.json'));
  } catch {
    return false;
  }
}

/**
 * Resolve the extension directory to load, preferring the first candidate that
 * actually contains a manifest:
 *   1. $KABOOM_EXTENSION_DIR   — explicit override, honored verbatim
 *   2. the copy bundled in this npm package (../extension, relative to lib/)
 *   3. ~/KaboomAgenticDevtoolExtension — staged by the curl|sh installer
 * When none exist yet, returns the override (if set) or the bundled default so a
 * concrete path is always shown; `exists` says whether it is actually there.
 *
 * @param {object} [env] defaults to process.env — injectable for tests
 * @param {string} [homeDir] defaults to os.homedir() — injectable for tests
 * @returns {{dir: string, exists: boolean, source: 'env'|'bundled'|'staged'}}
 */
function resolveExtensionDir(env = process.env, homeDir = os.homedir()) {
  // An explicit override is honored verbatim — the user said "use this path", so
  // never silently fall through to a different one, even when it is not there yet.
  const override = String(env.KABOOM_EXTENSION_DIR || '').trim();
  if (override) {
    return { dir: override, exists: isExtensionDir(override), source: 'env' };
  }

  const bundled = path.join(__dirname, '..', '..', 'extension');
  const staged = path.join(homeDir, STAGED_DIR_NAME);
  for (const candidate of [{ dir: bundled, source: 'bundled' }, { dir: staged, source: 'staged' }]) {
    if (isExtensionDir(candidate.dir)) {
      return { dir: candidate.dir, exists: true, source: candidate.source };
    }
  }
  // Nothing staged yet — still name the bundled location so the user has a path.
  return { dir: bundled, exists: false, source: 'bundled' };
}

/** True when the user opted out of the install-time auto-open. */
function autoOpenDisabled(env = process.env) {
  return isEnvFlagSet(env, ['KABOOM_NO_OPEN', 'KABOOM_INSTALL_NO_OPEN']);
}

/**
 * The command that reveals dir in the platform's file manager, or null when the
 * platform has none. Pure — the caller runs it.
 * @returns {{command: string, args: string[]}|null}
 */
function revealCommand(platform, dir) {
  switch (platform) {
    case 'darwin':
      return { command: 'open', args: [dir] };
    case 'win32':
      return { command: 'explorer', args: [dir] };
    case 'linux':
      return { command: 'xdg-open', args: [dir] };
    default:
      return null;
  }
}

/**
 * Best-effort: reveal the extension dir in the file manager so Load unpacked is
 * one selection away. Never throws; returns whether an opener was launched.
 * @param {string} dir
 * @param {{platform?: string, env?: object, spawnFn?: Function}} [opts]
 */
function openExtensionDir(dir, opts = {}) {
  const { platform = process.platform, env = process.env, spawnFn = spawn } = opts;
  if (!dir || autoOpenDisabled(env) || !isExtensionDir(dir)) return false;
  const spec = revealCommand(platform, dir);
  if (!spec) return false;
  try {
    const child = spawnFn(spec.command, spec.args, { stdio: 'ignore', detached: true });
    // A missing opener (e.g. no xdg-open) surfaces async — swallow it; the path
    // is always printed too, so the user still has the fallback.
    if (child && typeof child.on === 'function') child.on('error', () => {});
    if (child && typeof child.unref === 'function') child.unref();
    return true;
  } catch {
    return false;
  }
}

module.exports = {
  STAGED_DIR_NAME,
  isExtensionDir,
  resolveExtensionDir,
  autoOpenDisabled,
  revealCommand,
  openExtensionDir,
};
