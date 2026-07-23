// @ts-nocheck
/**
 * @fileoverview chrome-platform-limits.test.js — One assertion per Chrome hard
 * limit, so the platform's cliffs fail a build instead of a user's browser.
 *
 * Why this file exists: every extension break this project has shipped was a
 * Chrome platform contract, and every one of them passed the full test suite.
 * Unit tests run against a `chrome` object we wrote ourselves, so they can only
 * confirm our beliefs about Chrome — they cannot falsify them. These assertions
 * read the manifest and the source directly and compare them against limits
 * Chrome actually enforces.
 *
 * The bar for inclusion is a **hard** limit: something Chrome rejects, silently
 * drops, or refuses to run. Soft ergonomics belong elsewhere. Each entry cites
 * the rule it encodes, and where one bit us, what it cost.
 *
 * Deliberately not here: the six-item action context menu limit
 * (`contextMenus.ACTION_MENU_TOP_LEVEL_LIMIT`). Our items are page-context and
 * Chrome auto-groups those under a parent, so the limit does not apply — pinning
 * it would assert a contract that is not real.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs'
import path from 'node:path'

const EXTENSION_DIR = 'extension'
const MANIFEST_PATH = path.join(EXTENSION_DIR, 'manifest.json')

function loadManifest() {
  return JSON.parse(readFileSync(MANIFEST_PATH, 'utf8'))
}

function listSourceFiles(dir = 'src') {
  const out = []
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry)
    if (statSync(full).isDirectory()) out.push(...listSourceFiles(full))
    else if (full.endsWith('.ts')) out.push(full)
  }
  return out
}

// =============================================================================
// COMMANDS
// =============================================================================

/**
 * Chrome allows at most four commands with a `suggested_key` and does not
 * degrade past it: it refuses the *entire* manifest with "Too many shortcuts
 * specified for 'commands': The maximum is 4" and the extension fails to load.
 * https://developer.chrome.com/docs/extensions/reference/api/commands
 *
 * Cost when we hit it: the extension stopped loading altogether. The manifest
 * was still valid JSON, so nothing in the build noticed.
 */
const MAX_SUGGESTED_KEYS = 4

describe('manifest commands', () => {
  test(`at most ${MAX_SUGGESTED_KEYS} commands may declare a suggested_key`, () => {
    const commands = loadManifest().commands ?? {}
    const withKeys = Object.entries(commands).filter(([, cmd]) => cmd?.suggested_key)
    assert.ok(
      withKeys.length <= MAX_SUGGESTED_KEYS,
      `${withKeys.length} commands declare suggested_key (max ${MAX_SUGGESTED_KEYS}). ` +
        'Chrome rejects the entire manifest, so the extension will not load at all. ' +
        'Ship extra commands unbound and let users assign keys at chrome://extensions/shortcuts. ' +
        `Offenders: ${withKeys.map(([name]) => name).join(', ')}`
    )
  })

  test('every command has a description so it is assignable in the shortcuts UI', () => {
    const commands = loadManifest().commands ?? {}
    for (const [name, cmd] of Object.entries(commands)) {
      assert.ok(
        typeof cmd?.description === 'string' && cmd.description.length > 0,
        `command "${name}" needs a description; without one users cannot identify it at chrome://extensions/shortcuts`
      )
    }
  })

  test('the terminal panel command is still registered', () => {
    // It may be unbound, but it must exist: it is the gesture-native path that
    // the in-page launcher button cannot reliably provide.
    const commands = loadManifest().commands ?? {}
    assert.ok(commands.open_terminal_panel, 'open_terminal_panel command is missing')
  })
})

// =============================================================================
// MANIFEST V3 SHAPE
// =============================================================================

/** Keys Chrome removed in MV3; their presence invalidates the manifest. */
const MV2_ONLY_KEYS = ['browser_action', 'page_action', 'content_security_policy_v2', 'persistent']

describe('manifest v3 shape', () => {
  test('manifest_version is 3', () => {
    assert.strictEqual(loadManifest().manifest_version, 3)
  })

  test('no MV2-only keys survive', () => {
    const manifest = loadManifest()
    for (const key of MV2_ONLY_KEYS) {
      assert.strictEqual(manifest[key], undefined, `${key} is MV2-only and invalidates an MV3 manifest`)
    }
    assert.strictEqual(manifest.background?.scripts, undefined,
      'MV3 backgrounds are a service_worker, not a scripts array')
    assert.strictEqual(manifest.background?.persistent, undefined,
      'MV3 service workers cannot be persistent')
  })

  test('web_accessible_resources use the MV3 object form', () => {
    // MV3 replaced the flat string array with objects that must scope each
    // resource to matches. A string array is silently ignored, so injected
    // scripts fail to load with no error pointing at the manifest.
    for (const entry of loadManifest().web_accessible_resources ?? []) {
      assert.strictEqual(typeof entry, 'object',
        'MV3 requires { resources, matches } objects, not bare strings')
      assert.ok(Array.isArray(entry.resources) && entry.resources.length > 0)
      assert.ok(Array.isArray(entry.matches) && entry.matches.length > 0,
        'a resource with no matches is unreachable from any page')
    }
  })
})

// =============================================================================
// FILES THE MANIFEST PROMISES
// =============================================================================

/** Every path the manifest references, flattened. */
function manifestReferencedFiles(manifest) {
  const files = []
  const add = (value) => { if (typeof value === 'string') files.push(value) }

  add(manifest.background?.service_worker)
  add(manifest.action?.default_popup)
  add(manifest.side_panel?.default_path)
  add(manifest.options_ui?.page)
  for (const icon of Object.values(manifest.icons ?? {})) add(icon)
  for (const icon of Object.values(manifest.action?.default_icon ?? {})) add(icon)
  for (const script of manifest.content_scripts ?? []) {
    for (const js of script.js ?? []) add(js)
    for (const css of script.css ?? []) add(css)
  }
  for (const entry of manifest.web_accessible_resources ?? []) {
    for (const resource of entry.resources ?? []) add(resource)
  }
  return files
}

describe('manifest file references', () => {
  test('every file the manifest names exists in the packaged extension', () => {
    // A rename that misses the manifest produces no build error at all: Chrome
    // refuses to load the extension, or loads it with a silently dead script.
    // Wildcards are skipped — they cannot be resolved to a single path.
    const missing = manifestReferencedFiles(loadManifest())
      .filter((file) => !file.includes('*'))
      .filter((file) => !existsSync(path.join(EXTENSION_DIR, file)))

    assert.deepStrictEqual(missing, [], `manifest references files that do not exist: ${missing.join(', ')}`)
  })
})

// =============================================================================
// PERMISSIONS FOR THE APIS WE ACTUALLY CALL
// =============================================================================

/**
 * Chrome namespace → the permission it requires, or null when it needs none.
 *
 * An undeclared namespace is `undefined` at runtime — not an error, just
 * silently absent — so guarded code (`chrome.tabGroups?.update`) skips the
 * feature forever and looks like it simply does not work.
 * https://developer.chrome.com/docs/extensions/reference/permissions-list
 */
const API_PERMISSIONS = {
  runtime: null,
  action: null,
  commands: null,
  windows: null,
  i18n: null,
  extension: null,
  storage: 'storage',
  tabs: 'tabs',
  scripting: 'scripting',
  alarms: 'alarms',
  offscreen: 'offscreen',
  tabCapture: 'tabCapture',
  debugger: 'debugger',
  cookies: 'cookies',
  contextMenus: 'contextMenus',
  sidePanel: 'sidePanel',
  tabGroups: 'tabGroups',
  notifications: 'notifications',
  downloads: 'downloads',
  webRequest: 'webRequest',
  declarativeNetRequest: 'declarativeNetRequest',
  management: 'management',
  bookmarks: 'bookmarks',
  history: 'history',
  idle: 'idle',
  power: 'power',
  system: 'system.cpu'
}

/**
 * Namespaces we call without declaring their permission, and why that is
 * knowingly tolerated. Each entry is a live gap, not an exemption — the point of
 * naming them here is that they stay visible in the failure message rather than
 * disappearing into a passing test.
 */
const KNOWN_PERMISSION_GAPS = {}

/** Namespaces referenced in source, ignoring import paths like './chrome.js'. */
function usedChromeNamespaces() {
  const used = new Map()
  for (const file of listSourceFiles()) {
    const text = readFileSync(file, 'utf8')
    for (const match of text.matchAll(/(?<![./\w])chrome\.([a-zA-Z]+)/g)) {
      if (!used.has(match[1])) used.set(match[1], file)
    }
  }
  return used
}

describe('permissions match the APIs we call', () => {
  test('every chrome namespace used in src/ is a known one', () => {
    // Forces the map above to grow with the code. An unmapped namespace is not
    // a failure of the code — it is this test admitting it cannot judge it.
    const unknown = [...usedChromeNamespaces()]
      .filter(([ns]) => !(ns in API_PERMISSIONS))
      .map(([ns, file]) => `${ns} (${file})`)

    assert.deepStrictEqual(unknown, [],
      `add these to API_PERMISSIONS with the permission they need, or null: ${unknown.join(', ')}`)
  })

  test('every API we call has its permission declared, or a documented gap', () => {
    const declared = new Set(loadManifest().permissions ?? [])
    const undeclared = [...usedChromeNamespaces()]
      .filter(([ns]) => ns in API_PERMISSIONS)
      .filter(([ns]) => API_PERMISSIONS[ns] && !declared.has(API_PERMISSIONS[ns]))
      .map(([ns, file]) => ({ ns, file }))

    const undocumented = undeclared.filter(({ ns }) => !(ns in KNOWN_PERMISSION_GAPS))
    assert.deepStrictEqual(
      undocumented.map(({ ns, file }) => `${ns} (used in ${file}, needs "${API_PERMISSIONS[ns]}")`),
      [],
      'these APIs are undefined at runtime without their permission, so the code using them ' +
        'silently does nothing. Declare the permission, or record it in KNOWN_PERMISSION_GAPS ' +
        'with the reason.'
    )
  })

  test('documented permission gaps are still real', () => {
    // Keeps the exemption list from outliving its reason: once a permission is
    // declared, its entry must go, or the next gap hides behind stale text.
    const declared = new Set(loadManifest().permissions ?? [])
    for (const ns of Object.keys(KNOWN_PERMISSION_GAPS)) {
      assert.ok(
        !declared.has(API_PERMISSIONS[ns]),
        `"${API_PERMISSIONS[ns]}" is now declared — remove ${ns} from KNOWN_PERMISSION_GAPS`
      )
    }
  })

  test('no permission is declared that nothing uses', () => {
    // Every permission is a line in the install prompt. Unused ones cost user
    // trust and Web Store review time for nothing.
    const used = new Set(
      [...usedChromeNamespaces().keys()].map((ns) => API_PERMISSIONS[ns]).filter(Boolean)
    )
    // activeTab is granted by user action rather than by calling an API.
    const notApiBacked = new Set(['activeTab'])
    const unused = (loadManifest().permissions ?? [])
      .filter((permission) => !notApiBacked.has(permission) && !used.has(permission))

    assert.deepStrictEqual(unused, [], `declared but never used: ${unused.join(', ')}`)
  })
})

// =============================================================================
// STORAGE ACCESS LEVEL
// =============================================================================

describe('session storage reachability', () => {
  test('content scripts can only read session storage if the worker opens it up', () => {
    // chrome.storage.session defaults to TRUSTED_CONTEXTS: content scripts get
    // an empty object, not an error. Anything in a content script that reads it
    // would silently see nothing.
    const contentReaders = listSourceFiles('src/content')
      .filter((file) => readFileSync(file, 'utf8').includes('getSession('))

    if (contentReaders.length === 0) return

    const backgroundOpensIt = listSourceFiles('src/background')
      .some((file) => readFileSync(file, 'utf8').includes('TRUSTED_AND_UNTRUSTED_CONTEXTS'))

    assert.ok(
      backgroundOpensIt,
      `${contentReaders.length} content-script file(s) read chrome.storage.session, but no background ` +
        'file calls setAccessLevel(TRUSTED_AND_UNTRUSTED_CONTEXTS) — those reads return nothing. ' +
        `First reader: ${contentReaders[0]}`
    )
  })
})
