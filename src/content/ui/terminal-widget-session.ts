/**
 * Purpose: Terminal session lifecycle — config persistence, session start/validate/persist.
 * Why: Isolates all daemon HTTP calls and chrome.storage I/O from UI and orchestrator logic.
 * Docs: docs/features/feature/terminal/index.md
 */

import { DEFAULT_SERVER_URL, StorageKey } from '../../lib/constants.js'
import { getDaemonStartHint } from '../../lib/brand.js'
import { persist } from '../../lib/storage/io.js'
import { getLocal, setLocal } from '../../lib/storage/local.js'
import { getSession, removeSessions, setSession } from '../../lib/storage/session.js'
import {
  state,
  resolveTerminalServerUrl,
  type TerminalConfig,
  type TerminalSessionState,
  type TerminalUIState
} from './terminal-widget-types.js'

/**
 * Why a terminal session failed to start — lets the UI choose the right surface:
 * - `unreachable`: the daemon did not answer (transport failure / "Failed to
 *   fetch"). A real, actionable failure that must be shown, even if no panel body
 *   is mounted yet (surface via toast).
 * - `unavailable`: the daemon answered with an error status (e.g. 500). Reachable
 *   but not ready — recoverable, so the UI falls through to the no-session state
 *   (Start + root folder) rather than a dead-end error.
 * - `sandbox`: the daemon refused the spawn under macOS sandbox restrictions. An
 *   actionable failure carrying a remedy command; always shown.
 */
export type TerminalStartFailureKind = 'unreachable' | 'unavailable' | 'sandbox'

export type TerminalSandboxErrorHandler = (
  message: string,
  instruction: string,
  command: string,
  kind?: TerminalStartFailureKind
) => void

// =============================================================================
// CONFIG HELPERS — read/write chrome.storage.local
// =============================================================================

export async function getServerUrl(): Promise<string> {
  try {
    const value = await getLocal(StorageKey.SERVER_URL)
    const url = (value as string) || DEFAULT_SERVER_URL
    state.serverUrl = url
    return url
  } catch {
    return DEFAULT_SERVER_URL // Extension context invalidated
  }
}

export async function getTerminalConfig(): Promise<TerminalConfig> {
  try {
    const value = await getLocal(StorageKey.TERMINAL_CONFIG)
    const config = (value as TerminalConfig) || {}
    return config
  } catch {
    return {} // Extension context invalidated
  }
}

export function saveTerminalConfig(config: TerminalConfig): void {
  try {
    persist(setLocal(StorageKey.TERMINAL_CONFIG, config), 'terminal-config')
  } catch {
    // Extension context invalidated — config won't persist but session still works
  }
}

async function getTerminalAICommand(): Promise<string> {
  try {
    const value = await getLocal(StorageKey.TERMINAL_AI_COMMAND)
    const cmd = (value as string) || 'claude'
    return cmd
  } catch {
    return 'claude'
  }
}

/**
 * Build the command entered into the login shell after it has loaded the user's
 * profile. Checking here (rather than in the daemon's own environment) catches
 * API credentials exported by .zprofile/.zshrc without ever reading or sending
 * their values to the extension.
 */
export function buildAIInitCommand(aiCommand: string): string {
  if (!aiCommand) return ''
  const billingOverrides = [
    'ANTHROPIC_API_KEY',
    'ANTHROPIC_AUTH_TOKEN',
    'ANTHROPIC_BASE_URL',
    'CLAUDE_CODE_USE_BEDROCK',
    'CLAUDE_CODE_USE_VERTEX',
    'CLAUDE_CODE_USE_FOUNDRY',
    'OPENAI_API_KEY',
    'OPENAI_BASE_URL',
    'CODEX_ACCESS_TOKEN',
    'AWS_BEARER_TOKEN_BEDROCK'
  ]
  const presenceCheck = billingOverrides.map((name) => `\${${name}:-}`).join('')
  const firstCommand = aiCommand.trimStart().split(/\s+/, 1)[0] ?? ''
  const isCodex = /(^|\/)codex$/.test(firstCommand)
  const isClaude = /(^|\/)claude$/.test(firstCommand)
  const tool = isCodex ? 'codex' : isClaude ? 'claude' : 'other'
  const authCheck = isCodex
    ? ` codex_auth_status="$(codex login status 2>&1)"; case "$codex_auth_status" in *"Logged in using ChatGPT"*) kaboom_execution_provider=subscription;; *"API key"*|*"access token"*) kaboom_execution_provider=api;; esac;`
    : isClaude
      ? ` claude_auth_status="$(claude auth status 2>&1)"; case "$claude_auth_status" in *'"authMethod": "claude.ai"'*|*'"authMethod":"claude.ai"'*) kaboom_execution_provider=subscription;; *'"authMethod"'*) kaboom_execution_provider=api;; esac;`
      : ''
  const providerMarkers =
    ` case "$kaboom_execution_provider" in` +
    ` api) printf '\\033]1337;KABOOM_EXECUTION_PROVIDER=api:${tool}\\007';;` +
    ` subscription) printf '\\033]1337;KABOOM_EXECUTION_PROVIDER=subscription:${tool}\\007';;` +
    ` *) printf '\\033]1337;KABOOM_EXECUTION_PROVIDER=unknown:${tool}\\007';; esac;`
  const apiPrompt =
    ` if [ "$kaboom_execution_provider" = api ]; then` +
    ` printf '\\033[1;33m⚠ API billing credentials detected. This will not use your subscription.\\033[0m\\nContinue with API billing? [y/N] ';` +
    ` read -r kaboom_api_confirm; case "$kaboom_api_confirm" in y|Y|yes|YES) ${aiCommand};; *) printf 'API launch cancelled.\\n';; esac;` +
    ` else ${aiCommand}; fi;`
  return (
    `kaboom_execution_provider=unknown;` +
    ` if [ -n "${presenceCheck}" ]; then kaboom_execution_provider=api; fi;` +
    authCheck +
    ` if [ -n "${presenceCheck}" ]; then kaboom_execution_provider=api; fi;` +
    providerMarkers +
    apiPrompt +
    ` unset kaboom_execution_provider kaboom_api_confirm claude_auth_status codex_auth_status`
  )
}

export async function getTerminalDevRoot(): Promise<string> {
  try {
    const value = await getLocal(StorageKey.TERMINAL_DEV_ROOT)
    return (value as string) || ''
  } catch {
    return ''
  }
}

// =============================================================================
// SESSION PERSISTENCE — survives page refresh via chrome.storage.session
// =============================================================================

function persistSession(ss: TerminalSessionState): void {
  try {
    persist(setSession(StorageKey.TERMINAL_SESSION, ss), 'terminal-session')
  } catch {
    /* extension context invalidated */
  }
}

export function clearPersistedSession(): void {
  try {
    persist(removeSessions([StorageKey.TERMINAL_SESSION, StorageKey.TERMINAL_UI_STATE]), 'terminal-session-clear')
  } catch {
    /* extension context invalidated */
  }
}

export function persistUIState(uiState: TerminalUIState): void {
  try {
    persist(setSession(StorageKey.TERMINAL_UI_STATE, uiState), 'terminal-ui-state')
  } catch {
    /* extension context invalidated */
  }
}

export async function loadPersistedSession(): Promise<{
  session: TerminalSessionState | null
  uiState: TerminalUIState
}> {
  try {
    const sessionValue = await getSession(StorageKey.TERMINAL_SESSION)
    const uiValue = await getSession(StorageKey.TERMINAL_UI_STATE)
    const session = sessionValue as TerminalSessionState | undefined
    const uiState = (uiValue as TerminalUIState) || 'closed'
    return { session: session || null, uiState }
  } catch {
    return { session: null, uiState: 'closed' }
  }
}

// =============================================================================
// SESSION LIFECYCLE — start, validate
// =============================================================================

/** Validate that a persisted token is still alive on the daemon. */
export async function validateSession(token: string): Promise<boolean> {
  try {
    const base = await getServerUrl()
    const termUrl = await resolveTerminalServerUrl(base)
    const resp = await fetch(`${termUrl}/terminal/validate?token=${encodeURIComponent(token)}`, {
      signal: AbortSignal.timeout(2000)
    })
    if (!resp.ok) return false
    const data = (await resp.json()) as { valid?: boolean }
    return data.valid === true
  } catch {
    return false
  }
}

/** One selectable directory from the daemon's listing. */
export interface TerminalDirEntry {
  name: string
  path: string
}

/** A directory and its immediate sub-directories. */
export interface TerminalDirListing {
  path: string
  parent: string
  entries: TerminalDirEntry[]
  truncated: boolean
}

/**
 * Why a directory listing could not be produced.
 * - `unreachable`: the daemon did not answer (down, refused, timed out).
 * - `outdated`: the daemon answered 404 with no error body — it predates
 *   `/terminal/dirs`. A version problem, not a connectivity or path one.
 * - `not_found`: the daemon answered 404 *with* a `not_found` error body — it has
 *   the endpoint, but the requested folder does not exist (e.g. a saved root that
 *   was since deleted). A current daemon, so telling the user to update it is wrong.
 * - `denied`: the daemon answered 403 — the folder exists but cannot be read
 *   (permissions). Also a reachable, current daemon.
 */
export type TerminalDirsFailure = 'unreachable' | 'outdated' | 'not_found' | 'denied'

/** The listing, or the reason it could not be fetched. */
export type TerminalDirsResult = { ok: true; listing: TerminalDirListing } | { ok: false; reason: TerminalDirsFailure }

/**
 * List the sub-directories of `path`, or of the user's home when empty.
 *
 * The browser cannot resolve an absolute path by itself — `webkitdirectory` and
 * showDirectoryPicker() both withhold it — so picking a working directory has to
 * go through the daemon, which is already running shells in these directories.
 *
 * Distinguishes a daemon that is down from one that is merely too old to have the
 * endpoint: a 404 is a reachable daemon, and telling the user it is unreachable
 * sends them debugging a connection that is fine.
 */
export async function listTerminalDirs(path: string): Promise<TerminalDirsResult> {
  let resp: Response
  try {
    const base = await getServerUrl()
    const termUrl = await resolveTerminalServerUrl(base)
    resp = await fetch(`${termUrl}/terminal/dirs?path=${encodeURIComponent(path)}`, {
      signal: AbortSignal.timeout(3000)
    })
  } catch {
    return { ok: false, reason: 'unreachable' } // No answer at all.
  }

  if (resp.status === 404) {
    // 404 is ambiguous: a daemon that predates /terminal/dirs 404s the whole
    // route with Chrome's plain-text ServeMux default (no error body), while a
    // current daemon 404s a *missing directory* with {"error":"not_found"}.
    // Telling a user whose folder was deleted to update a current daemon sends
    // them fixing the wrong thing, so distinguish by the presence of our body.
    const daemonError = await readDaemonError(resp)
    return { ok: false, reason: daemonError === 'not_found' ? 'not_found' : 'outdated' }
  }
  // 403 is a reachable, current daemon that cannot read the folder (permissions),
  // which is a different problem — and message — from a daemon that is down.
  if (resp.status === 403) return { ok: false, reason: 'denied' }
  if (!resp.ok) return { ok: false, reason: 'unreachable' }

  try {
    const data = (await resp.json()) as Partial<TerminalDirListing>
    return {
      ok: true,
      listing: {
        path: data.path ?? path,
        parent: data.parent ?? '',
        entries: Array.isArray(data.entries) ? data.entries : [],
        truncated: data.truncated === true
      }
    }
  } catch {
    return { ok: false, reason: 'unreachable' } // Reached it, but the body was unusable.
  }
}

/**
 * Read the daemon's structured `error` code from a response body, or '' when the
 * body is not our JSON shape — which is exactly how an old daemon's plain-text
 * 404 (no `/terminal/dirs` route) is told apart from a current daemon's
 * `{"error":"not_found"}` for a directory that does not exist.
 */
async function readDaemonError(resp: Response): Promise<string> {
  try {
    const body = (await resp.json()) as { error?: unknown }
    return typeof body.error === 'string' ? body.error : ''
  } catch {
    return '' // Plain-text / empty body — not one of our structured errors.
  }
}

/** Persist the terminal root folder (the cwd new sessions spawn in). */
export async function setTerminalDevRoot(root: string): Promise<void> {
  try {
    await setLocal(StorageKey.TERMINAL_DEV_ROOT, root)
  } catch {
    // Extension context invalidated — nothing to persist into.
  }
}

/**
 * Poll `/terminal/validate` until the token no longer maps to a live session, or
 * a bounded number of attempts elapse. Used to CONFIRM a stop actually landed.
 */
async function waitForSessionTornDown(token: string, attempts = 5, delayMs = 200): Promise<boolean> {
  for (let i = 0; i < attempts; i++) {
    if (!(await validateSession(token))) return true // gone
    await new Promise((resolve) => setTimeout(resolve, delayMs))
  }
  return false // still alive after retries — the daemon is wedged
}

/**
 * Stop the active PTY and forget it locally.
 *
 * Used when a setting that is fixed at spawn time changes (the working
 * directory), and by the explicit end-session control.
 *
 * The stop must be CONFIRMED, not fire-and-forget: the session id is a fixed
 * "default", so if the stop times out but the old session survives, the following
 * /terminal/start returns 409 and the client silently reconnects to the OLD
 * working directory while the UI shows the newly-picked one. A 200 (or a 404 =
 * already gone) confirms teardown; otherwise poll validate before returning so a
 * fresh start cannot 409-reattach to a stale cwd.
 */
export async function stopActiveSession(): Promise<void> {
  const persisted = await loadPersistedSession()
  const sessionId = persisted.session?.sessionId
  const token = persisted.session?.token
  clearPersistedSession()
  if (!sessionId) return
  try {
    const base = await getServerUrl()
    const termUrl = await resolveTerminalServerUrl(base)
    const resp = await fetch(`${termUrl}/terminal/stop`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: sessionId }),
      signal: AbortSignal.timeout(3000)
    })
    // Stop is synchronous server-side: 200 = torn down, 404 = already gone.
    if (resp.ok || resp.status === 404) return
  } catch {
    // Timed out / unreachable — the stop is unconfirmed. Verify below.
  }
  if (token) await waitForSessionTornDown(token)
}

export async function startSession(
  config: TerminalConfig,
  onSandboxError?: TerminalSandboxErrorHandler
): Promise<TerminalSessionState | null> {
  const base = await getServerUrl()
  const termUrl = await resolveTerminalServerUrl(base)
  const aiCommand = await getTerminalAICommand()
  const devRoot = await getTerminalDevRoot()
  try {
    // Build init_command: unset CLAUDECODE to avoid nesting detection, then launch the AI tool.
    const launchCommand = buildAIInitCommand(aiCommand)
    const initCommand = launchCommand ? `unset CLAUDECODE 2>/dev/null; ${launchCommand}` : ''
    const resp = await fetch(`${termUrl}/terminal/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        cmd: config.cmd || '',
        args: config.args || [],
        dir: config.dir || devRoot || '',
        init_command: initCommand
      })
    })
    if (!resp.ok) {
      const body = (await resp.json()) as {
        error?: string
        message?: string
        instruction?: string
        command?: string
        detail?: string
        session_id?: string
        token?: string
      }
      // Session already exists — reconnect using the returned token.
      if (resp.status === 409 && body.token) {
        const ss = { sessionId: body.session_id ?? 'default', token: body.token }
        persistSession(ss)
        return ss
      }
      // Sandbox restriction — the daemon's message is its diagnosis, `detail` is
      // the underlying error. Show both: the diagnosis can be wrong, the error can't.
      if (resp.status === 503 && body.error === 'sandbox_restricted') {
        const message = body.detail
          ? `${body.message ?? 'Terminal start was refused.'} (${body.detail})`
          : (body.message ?? 'Terminal start was refused.')
        reportStartFailure(message, body.instruction ?? '', body.command ?? '', 'sandbox', onSandboxError)
        return null
      }
      // Any other rejection from a reachable daemon. This used to only
      // console.warn, so the side panel rendered nothing at all and the terminal
      // looked simply broken. Classified `unavailable` (reachable but not ready)
      // so the UI shows the recoverable no-session state, not a dead-end error.
      reportStartFailure(
        `Terminal start was refused (HTTP ${resp.status}): ${body.error ?? 'unknown error'}.`,
        '',
        '',
        'unavailable',
        onSandboxError
      )
      return null
    }
    const data = (await resp.json()) as { session_id: string; token: string; pid: number }
    const ss = { sessionId: data.session_id, token: data.token }
    persistSession(ss)
    return ss
  } catch (err) {
    // Transport failure — the daemon did not answer at all. This is `unreachable`:
    // a real failure the user must see even when no panel body is mounted yet.
    reportStartFailure(
      'Terminal session start failed: ' + (err instanceof Error ? err.message : String(err)) + '.',
      getDaemonStartHint(),
      '',
      'unreachable',
      onSandboxError
    )
    return null
  }
}

/**
 * Route a start failure to the panel when a handler is available, and always log
 * it. A failure that only reaches the console leaves the panel blank, which reads
 * as "the terminal is broken" rather than "here is what went wrong".
 */
function reportStartFailure(
  message: string,
  instruction: string,
  command: string,
  kind: TerminalStartFailureKind,
  onError?: TerminalSandboxErrorHandler
): void {
  console.warn(`[KaBOOM!] (${kind}) ${message} ${instruction} ${command}`.trimEnd())
  onError?.(message, instruction, command, kind)
}
