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
const { spawn, execFileSync } = require('node:child_process');
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

// --- OS default browser detection (maps native identifiers to KNOWN_BROWSERS ids) ---

const DARWIN_BUNDLE_TO_ID = {
  'com.google.chrome': 'chrome',
  'com.brave.browser': 'brave',
  'com.microsoft.edgemac': 'edge',
  'company.thebrowser.browser': 'arc',
  'org.chromium.chromium': 'chromium',
};
const LINUX_DESKTOP_TO_ID = {
  'google-chrome': 'chrome',
  'google-chrome-stable': 'chrome',
  'brave-browser': 'brave',
  brave: 'brave',
  'microsoft-edge': 'edge',
  'microsoft-edge-stable': 'edge',
  chromium: 'chromium',
  'chromium-browser': 'chromium',
};
// Windows ProgId substrings, most-specific first.
const WIN_PROGID_TO_ID = [
  [/brave/i, 'brave'],
  [/(msedge|edgehtm|edge)/i, 'edge'],
  [/chromium/i, 'chromium'],
  [/chrome/i, 'chrome'],
];

/** Default command runner: stdout string, or null on any failure. */
function defaultRunFn(command, args) {
  try {
    // maxBuffer is raised well above Node's 1 MB default: `plutil -convert json`
    // on a large com.apple.launchservices.secure.plist can exceed it, which would
    // throw ENOBUFS and silently disable default-browser detection.
    // nosemgrep: javascript.lang.security.detect-child-process.detect-child-process -- fixed OS query commands (plutil/xdg-settings/reg) with static args, no shell, no user input
    return String(execFileSync(command, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'], timeout: 3000, maxBuffer: 8 * 1024 * 1024 }));
  } catch {
    return null;
  }
}

function darwinDefaultBrowserId(runFn, homeDir) {
  const plist = path.join(homeDir, 'Library', 'Preferences', 'com.apple.LaunchServices', 'com.apple.launchservices.secure.plist');
  const out = runFn('plutil', ['-convert', 'json', '-o', '-', plist]);
  if (!out) return null;
  let data;
  try {
    data = JSON.parse(out);
  } catch {
    return null;
  }
  const handlers = data && Array.isArray(data.LSHandlers) ? data.LSHandlers : [];
  // Prefer the https handler, then fall back to http. Some macOS systems register
  // the default browser under http only, so an https-only scan would miss it.
  const handlerFor = (scheme) => handlers.find(
    (h) => h && h.LSHandlerURLScheme === scheme && typeof h.LSHandlerRoleAll === 'string'
  );
  const match = handlerFor('https') || handlerFor('http');
  if (!match) return null;
  return DARWIN_BUNDLE_TO_ID[match.LSHandlerRoleAll.toLowerCase()] || null;
}

function linuxDefaultBrowserId(runFn) {
  const out = runFn('xdg-settings', ['get', 'default-web-browser']);
  if (!out) return null;
  const desktop = out.trim().replace(/\.desktop$/i, '').toLowerCase();
  return LINUX_DESKTOP_TO_ID[desktop] || null;
}

function winDefaultBrowserId(runFn) {
  const out = runFn('reg', [
    'query',
    'HKCU\\Software\\Microsoft\\Windows\\Shell\\Associations\\UrlAssociations\\https\\UserChoice',
    '/v',
    'ProgId',
  ]);
  if (!out) return null;
  const match = /ProgId\s+REG_SZ\s+(\S+)/i.exec(out);
  const progId = match ? match[1] : out;
  for (const [re, id] of WIN_PROGID_TO_ID) {
    if (re.test(progId)) return id;
  }
  return null;
}

/**
 * The KNOWN_BROWSERS id of the OS default web browser, or null if it can't be
 * determined or isn't a Chromium-family browser we support. Best-effort and
 * injectable (runFn/homeDir) for tests.
 * @param {{platform?: string, runFn?: Function, homeDir?: string}} [deps]
 * @returns {string|null}
 */
function detectDefaultBrowserId(deps = {}) {
  const platform = deps.platform || process.platform;
  const runFn = deps.runFn || defaultRunFn;
  const homeDir = deps.homeDir || os.homedir();
  try {
    if (platform === 'darwin') return darwinDefaultBrowserId(runFn, homeDir);
    if (platform === 'linux') return linuxDefaultBrowserId(runFn);
    if (platform === 'win32') return winDefaultBrowserId(runFn);
  } catch {
    return null;
  }
  return null;
}

/**
 * Find the browser to open the extensions page in, and how. Prefers the OS
 * default browser when it's a supported Chromium-family one that's installed —
 * that's the browser the user will actually load the extension into — and falls
 * back to the fixed preference order otherwise. Pure given its injected detectors.
 *
 * @param {Object} [deps]
 * @param {string} [deps.platform]  process.platform
 * @param {Object} [deps.env]       process.env (Windows path lookup)
 * @param {(appName: string) => boolean} [deps.hasApp]      macOS
 * @param {(cmd: string) => boolean} [deps.onPath]          Linux
 * @param {(absPath: string) => boolean} [deps.fileExists]  Windows
 * @param {Function} [deps.runFn]     default-browser command runner (see detectDefaultBrowserId)
 * @param {string} [deps.homeDir]     override home dir (macOS default lookup)
 * @returns {{id: string, name: string, url: string,
 *   launch: {command: string, args: string[]}}|null}
 */
function detectExtensionsTarget(deps = {}) {
  const platform = deps.platform || process.platform;
  const env = deps.env || process.env;
  const hasApp = deps.hasApp || defaultHasApp;
  const onPath = deps.onPath || commandExistsOnPath;
  const fileExists = deps.fileExists || defaultFileExists;

  // Try the OS default browser first; fall back to the fixed preference order.
  const defaultId = detectDefaultBrowserId({ platform, runFn: deps.runFn, homeDir: deps.homeDir });
  let order = KNOWN_BROWSERS;
  if (defaultId) {
    const preferred = KNOWN_BROWSERS.find((b) => b.id === defaultId);
    if (preferred) order = [preferred, ...KNOWN_BROWSERS.filter((b) => b.id !== defaultId)];
  }

  for (const browser of order) {
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
  detectDefaultBrowserId,
  detectExtensionsTarget,
  openExtensionsPage,
};
