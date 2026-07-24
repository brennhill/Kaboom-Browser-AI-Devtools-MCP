// Purpose: Detect an installed Chromium-family browser and open its extensions
// page, so the last install step ("Load unpacked") is one click away.
// Why: The extensions page uses a browser-internal scheme (chrome://, edge://…)
// that the OS file opener cannot handle — only the browser binary can open it.
// Detecting which browser is present lets us launch the right page directly.
// Docs: docs/features/feature/enhanced-cli-config/index.md

'use strict';

const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');
const { spawn } = require('node:child_process');
const { autoOpenDisabled } = require('./extension');
const { commandExistsOnPath } = require('./config');

// Ordered by preference. `scheme` is the browser-internal URL scheme for its
// extensions page. darwinApp is the .app bundle name; linuxBins are PATH names;
// winPaths are segment arrays joined onto each base dir (kept as segments so
// path.join builds a valid path on any host, e.g. for tests).
const KNOWN_BROWSERS = [
  {
    id: 'chrome', name: 'Google Chrome', scheme: 'chrome',
    darwinApp: 'Google Chrome',
    linuxBins: ['google-chrome', 'google-chrome-stable'],
    winPaths: [['Google', 'Chrome', 'Application', 'chrome.exe']],
  },
  {
    id: 'brave', name: 'Brave', scheme: 'brave',
    darwinApp: 'Brave Browser',
    linuxBins: ['brave-browser', 'brave'],
    winPaths: [['BraveSoftware', 'Brave-Browser', 'Application', 'brave.exe']],
  },
  {
    id: 'edge', name: 'Microsoft Edge', scheme: 'edge',
    darwinApp: 'Microsoft Edge',
    linuxBins: ['microsoft-edge', 'microsoft-edge-stable'],
    winPaths: [['Microsoft', 'Edge', 'Application', 'msedge.exe']],
  },
  {
    id: 'arc', name: 'Arc', scheme: 'chrome',
    darwinApp: 'Arc',
    linuxBins: [],
    winPaths: [],
  },
  {
    id: 'chromium', name: 'Chromium', scheme: 'chrome',
    darwinApp: 'Chromium',
    linuxBins: ['chromium', 'chromium-browser'],
    winPaths: [['Chromium', 'Application', 'chrome.exe']],
  },
];

/** The extensions-page URL for a browser scheme. */
function extensionsUrl(scheme) {
  return `${scheme}://extensions/`;
}

/** macOS: is `${appName}.app` present in a standard Applications folder? */
function defaultHasApp(appName) {
  for (const base of ['/Applications', path.join(os.homedir(), 'Applications')]) {
    try {
      if (fs.existsSync(path.join(base, `${appName}.app`))) return true;
    } catch {
      // Unreadable base dir — treat as absent.
    }
  }
  return false;
}

function defaultFileExists(p) {
  try {
    return fs.existsSync(p);
  } catch {
    return false;
  }
}

/** Windows base dirs where browsers install, in priority order. */
function winBaseDirs(env) {
  const dirs = [];
  for (const key of ['ProgramFiles', 'ProgramFiles(x86)', 'LOCALAPPDATA']) {
    const value = env[key];
    if (value && String(value).trim()) dirs.push(String(value));
  }
  return dirs;
}

function firstWinExe(browser, env, fileExists) {
  for (const base of winBaseDirs(env)) {
    for (const segments of browser.winPaths || []) {
      const full = path.join(base, ...segments);
      if (fileExists(full)) return full;
    }
  }
  return null;
}

/**
 * Find the first installed Chromium-family browser and how to open its
 * extensions page. Pure given its injected detectors.
 *
 * @param {Object} [deps]
 * @param {string} [deps.platform]  process.platform
 * @param {Object} [deps.env]       process.env (Windows path lookup)
 * @param {(appName: string) => boolean} [deps.hasApp]      macOS
 * @param {(cmd: string) => boolean} [deps.onPath]          Linux
 * @param {(absPath: string) => boolean} [deps.fileExists]  Windows
 * @returns {{id: string, name: string, url: string,
 *   launch: {command: string, args: string[]}}|null}
 */
function detectExtensionsTarget(deps = {}) {
  const platform = deps.platform || process.platform;
  const env = deps.env || process.env;
  const hasApp = deps.hasApp || defaultHasApp;
  const onPath = deps.onPath || commandExistsOnPath;
  const fileExists = deps.fileExists || defaultFileExists;

  for (const browser of KNOWN_BROWSERS) {
    const url = extensionsUrl(browser.scheme);

    if (platform === 'darwin') {
      if (browser.darwinApp && hasApp(browser.darwinApp)) {
        return { id: browser.id, name: browser.name, url, launch: { command: 'open', args: ['-a', browser.darwinApp, url] } };
      }
    } else if (platform === 'linux') {
      for (const bin of browser.linuxBins) {
        if (onPath(bin)) {
          return { id: browser.id, name: browser.name, url, launch: { command: bin, args: [url] } };
        }
      }
    } else if (platform === 'win32') {
      const exe = firstWinExe(browser, env, fileExists);
      if (exe) {
        return { id: browser.id, name: browser.name, url, launch: { command: exe, args: [url] } };
      }
    }
  }
  return null;
}

/**
 * Best-effort: open the detected browser's extensions page. Never throws; shares
 * the folder auto-open opt-out. Returns whether a launch was attempted.
 * @param {ReturnType<typeof detectExtensionsTarget>} target
 * @param {{env?: Object, spawnFn?: Function}} [opts]
 */
function openExtensionsPage(target, opts = {}) {
  const env = opts.env || process.env;
  const spawnFn = opts.spawnFn || spawn;
  if (!target || !target.launch || autoOpenDisabled(env)) return false;
  try {
    const child = spawnFn(target.launch.command, target.launch.args, { stdio: 'ignore', detached: true });
    if (child && typeof child.on === 'function') child.on('error', () => {});
    if (child && typeof child.unref === 'function') child.unref();
    return true;
  } catch {
    return false;
  }
}

module.exports = {
  KNOWN_BROWSERS,
  extensionsUrl,
  detectExtensionsTarget,
  openExtensionsPage,
};
