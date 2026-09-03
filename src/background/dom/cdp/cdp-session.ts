/**
 * Purpose: Owns every chrome.debugger attachment so input dispatch, performance tracing and
 *          full-page capture share one session per tab instead of competing for it.
 * Why: Chrome permits exactly ONE debugger attachment per (extension, tab). Kaboom had four
 *      independent attach owners. Per-action attach/detach also races a click's own navigation
 *      — Input.dispatchMouseEvent resolves on delivery, not on default action, so detaching in
 *      a finally tears the session down mid-navigation (kaboom-knms) — and makes Chrome's
 *      debugging banner flicker once per action.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// cdp-session.ts — Single-owner, ref-counted CDP session manager with detach recovery.

import { CDP_VERSION } from '../../../lib/constants.js'
import { errorMessage } from '../../../lib/error-utils.js'

export interface Debuggee {
  tabId: number
}

/** The chrome.debugger surface this manager owns. Injected so tests use no real browser. */
export interface DebuggerApi {
  attach(target: Debuggee, requiredVersion: string): Promise<void>
  detach(target: Debuggee): Promise<void>
  sendCommand(target: Debuggee, method: string, params?: Record<string, unknown>): Promise<unknown>
  onDetach: { addListener(listener: (source: Debuggee, reason: string) => void): void }
}

type TimerHandle = ReturnType<typeof setTimeout>

export interface CDPSessionDeps {
  debuggerApi: DebuggerApi
  setTimeout?: (fn: () => void, ms: number) => TimerHandle
  clearTimeout?: (handle: TimerHandle) => void
  /** How long a session stays warm at zero refs. Long enough that an action burst reuses it. */
  idleGraceMs?: number
  cdpVersion?: string
  log?: (event: string, fields: Record<string, unknown>) => void
}

export interface AcquireOptions {
  /** Take the session alone. Tracing needs this: concurrent input would pollute the trace. */
  exclusive?: boolean
  /** How long an exclusive acquire waits for shared leases to drain before failing. */
  drainTimeoutMs?: number
}

/** Named failures. Callers match on these instead of parsing free-text Chrome errors. */
export const CDP_SESSION_ERRORS = {
  INVALIDATED: 'cdp_session_invalidated',
  EXCLUSIVE_HELD: 'cdp_session_exclusive_held',
  DRAIN_TIMEOUT: 'cdp_session_drain_timeout',
  ATTACH_FAILED: 'cdp_session_attach_failed'
} as const

export const DEFAULT_IDLE_GRACE_MS = 30_000
export const DEFAULT_DRAIN_TIMEOUT_MS = 10_000

/** Read-only probe used to decide whether Chrome already has us attached. */
const ADOPTION_PROBE = 'Page.getLayoutMetrics'

/** A held reference to a tab's CDP session. Release exactly once; repeat calls are no-ops. */
export interface Lease {
  readonly tabId: number
  readonly valid: boolean
  send(method: string, params?: Record<string, unknown>): Promise<unknown>
  /** Send `<domain>.enable` once for the lifetime of this session. */
  ensureDomain(domain: string): Promise<void>
  release(): void
}

interface Waiter {
  resolve: (lease: Lease) => void
  reject: (err: Error) => void
  timer: TimerHandle | null
}

interface Session {
  tabId: number
  /** Bumped on every teardown so leases from a dead session can never send. */
  generation: number
  refs: number
  attached: boolean
  exclusive: boolean
  enabledDomains: Set<string>
  idleTimer: TimerHandle | null
  waiters: Waiter[]
}

export class CDPSessionManager {
  private readonly deps: Required<Omit<CDPSessionDeps, 'log'>> & Pick<CDPSessionDeps, 'log'>
  private readonly sessions = new Map<number, Session>()

  constructor(deps: CDPSessionDeps) {
    this.deps = {
      debuggerApi: deps.debuggerApi,
      setTimeout: deps.setTimeout ?? ((fn, ms) => setTimeout(fn, ms)),
      clearTimeout: deps.clearTimeout ?? ((handle) => clearTimeout(handle)),
      idleGraceMs: deps.idleGraceMs ?? DEFAULT_IDLE_GRACE_MS,
      cdpVersion: deps.cdpVersion ?? CDP_VERSION,
      log: deps.log
    }
    this.deps.debuggerApi.onDetach.addListener((source, reason) => this.onExternalDetach(source, reason))
  }

  /** Acquire a reference to the tab's session, attaching (or adopting) if it is cold. */
  async acquire(tabId: number, options: AcquireOptions = {}): Promise<Lease> {
    const session = this.sessionFor(tabId)

    if (options.exclusive) return this.acquireExclusive(session, options)

    if (session.exclusive) {
      throw new Error(
        `${CDP_SESSION_ERRORS.EXCLUSIVE_HELD}: tab ${tabId} is held exclusively (performance trace or equivalent)`
      )
    }
    return this.grant(session, false)
  }

  /**
   * Tear the tab's session down NOW, invalidating every outstanding lease.
   *
   * This is what makes the supervision Stop button real: the interrupted action's next
   * lease.send fails loud with cdp_session_invalidated instead of running to completion
   * behind an overlay the user has already dismissed. Unlike a release, this does not wait
   * out the idle grace — the point is to interrupt.
   *
   * Inert for a tab with no live session, so a stop that races an action finishing cannot
   * tear down a session a later action already owns.
   */
  abort(tabId: number, reason: string): void {
    const session = this.sessions.get(tabId)
    if (!session || !session.attached) return
    this.cancelIdleTimer(session)
    session.refs = 0
    session.exclusive = false
    for (const waiter of session.waiters.splice(0)) {
      if (waiter.timer !== null) this.deps.clearTimeout(waiter.timer)
      waiter.reject(new Error(`${CDP_SESSION_ERRORS.INVALIDATED}: tab ${tabId} aborted (${reason})`))
    }
    void this.teardown(session, reason)
  }

  /** True when a live attachment exists for the tab. Diagnostics only. */
  isAttached(tabId: number): boolean {
    return this.sessions.get(tabId)?.attached === true
  }

  private sessionFor(tabId: number): Session {
    let session = this.sessions.get(tabId)
    if (!session) {
      session = {
        tabId,
        generation: 0,
        refs: 0,
        attached: false,
        exclusive: false,
        enabledDomains: new Set(),
        idleTimer: null,
        waiters: []
      }
      this.sessions.set(tabId, session)
    }
    return session
  }

  private async acquireExclusive(session: Session, options: AcquireOptions): Promise<Lease> {
    if (session.refs === 0 && !session.exclusive) return this.grant(session, true)

    const timeoutMs = options.drainTimeoutMs ?? DEFAULT_DRAIN_TIMEOUT_MS
    return new Promise<Lease>((resolve, reject) => {
      const waiter: Waiter = { resolve, reject, timer: null }
      waiter.timer = this.deps.setTimeout(() => {
        session.waiters = session.waiters.filter((w) => w !== waiter)
        reject(
          new Error(
            `${CDP_SESSION_ERRORS.DRAIN_TIMEOUT}: tab ${session.tabId} still had ${session.refs} open lease(s) after ${timeoutMs}ms`
          )
        )
      }, timeoutMs)
      session.waiters.push(waiter)
    })
  }

  private async grant(session: Session, exclusive: boolean): Promise<Lease> {
    this.cancelIdleTimer(session)
    session.refs += 1
    session.exclusive = exclusive
    try {
      await this.ensureAttached(session)
    } catch (err) {
      session.refs -= 1
      session.exclusive = false
      this.scheduleIdleDetach(session)
      throw err
    }
    return this.makeLease(session)
  }

  /**
   * Attach, or adopt an attachment that outlived this service worker.
   *
   * MV3 terminates the worker without detaching, so in-memory state can say "cold" while
   * Chrome still has us attached — and a second attach then throws. A read-only probe is
   * the only way to tell "we are already attached" from "someone else is": chrome.debugger
   * reports both as the same error string, and getTargets() reports that a debugger is
   * attached without saying whose it is (rule 18: reconcile against the live signal).
   */
  private async ensureAttached(session: Session): Promise<void> {
    if (session.attached) return
    const target: Debuggee = { tabId: session.tabId }

    if (await this.probeAdoptable(target)) {
      session.attached = true
      this.deps.log?.('cdp_session_adopted', { tab_id: session.tabId })
      return
    }

    try {
      await this.deps.debuggerApi.attach(target, this.deps.cdpVersion)
    } catch (err) {
      throw new Error(`${CDP_SESSION_ERRORS.ATTACH_FAILED}: ${errorMessage(err, 'unknown_error')}`)
    }
    session.attached = true
    session.enabledDomains.clear()
    this.deps.log?.('cdp_session_attached', { tab_id: session.tabId })
  }

  private async probeAdoptable(target: Debuggee): Promise<boolean> {
    try {
      await this.deps.debuggerApi.sendCommand(target, ADOPTION_PROBE, {})
      return true
    } catch {
      // EXPECTED_ABSENCE: a cold tab rejecting the probe is the normal path, and is exactly
      // the signal that we must attach. Logging it would report every first action as a failure.
      return false
    }
  }

  private makeLease(session: Session): Lease {
    const generation = session.generation
    let released = false
    const manager = this

    const assertLive = (): void => {
      if (released || session.generation !== generation || !session.attached) {
        throw new Error(
          `${CDP_SESSION_ERRORS.INVALIDATED}: the CDP session for tab ${session.tabId} was detached`
        )
      }
    }

    return {
      tabId: session.tabId,
      get valid(): boolean {
        return !released && session.generation === generation && session.attached
      },
      async send(method: string, params: Record<string, unknown> = {}): Promise<unknown> {
        assertLive()
        return manager.deps.debuggerApi.sendCommand({ tabId: session.tabId }, method, params)
      },
      async ensureDomain(domain: string): Promise<void> {
        assertLive()
        if (session.enabledDomains.has(domain)) return
        await manager.deps.debuggerApi.sendCommand({ tabId: session.tabId }, `${domain}.enable`, {})
        session.enabledDomains.add(domain)
      },
      release(): void {
        if (released) return
        released = true
        if (session.generation !== generation) return
        manager.releaseRef(session)
      }
    }
  }

  private releaseRef(session: Session): void {
    session.refs = Math.max(0, session.refs - 1)
    if (session.refs > 0) return
    session.exclusive = false
    if (this.promoteWaiter(session)) return
    this.scheduleIdleDetach(session)
  }

  /** Hand a drained session to the longest-waiting exclusive acquirer, if any. */
  private promoteWaiter(session: Session): boolean {
    const waiter = session.waiters.shift()
    if (!waiter) return false
    if (waiter.timer !== null) this.deps.clearTimeout(waiter.timer)
    this.grant(session, true).then(waiter.resolve, waiter.reject)
    return true
  }

  private scheduleIdleDetach(session: Session): void {
    if (!session.attached || session.idleTimer !== null) return
    session.idleTimer = this.deps.setTimeout(() => {
      session.idleTimer = null
      if (session.refs > 0) return
      void this.teardown(session, 'idle')
    }, this.deps.idleGraceMs)
  }

  private cancelIdleTimer(session: Session): void {
    if (session.idleTimer === null) return
    this.deps.clearTimeout(session.idleTimer)
    session.idleTimer = null
  }

  private async teardown(session: Session, reason: string): Promise<void> {
    if (!session.attached) return
    session.attached = false
    session.generation += 1
    session.enabledDomains.clear()
    try {
      await this.deps.debuggerApi.detach({ tabId: session.tabId })
      this.deps.log?.('cdp_session_detached', { tab_id: session.tabId, reason })
    } catch (err) {
      // A detach that fails leaves nothing to clean up — Chrome drops the session when the
      // tab or worker dies — but it is still unexpected, so it is reported (rule 27).
      this.deps.log?.('cdp_session_detach_failed', {
        tab_id: session.tabId,
        reason,
        error: errorMessage(err, 'unknown_error')
      })
    }
  }

  /**
   * Chrome detached us: DevTools opened, the tab closed, or the user clicked Cancel on the
   * debugging banner. The detach is authoritative — every outstanding lease is dead and must
   * fail loud on its next send rather than silently no-op against a session that is gone.
   */
  private onExternalDetach(source: Debuggee, reason: string): void {
    const session = this.sessions.get(source.tabId)
    if (!session || !session.attached) return
    this.cancelIdleTimer(session)
    session.attached = false
    session.generation += 1
    session.refs = 0
    session.exclusive = false
    session.enabledDomains.clear()
    this.deps.log?.('cdp_session_external_detach', { tab_id: source.tabId, reason })
    for (const waiter of session.waiters.splice(0)) {
      if (waiter.timer !== null) this.deps.clearTimeout(waiter.timer)
      waiter.reject(
        new Error(`${CDP_SESSION_ERRORS.INVALIDATED}: tab ${source.tabId} detached (${reason})`)
      )
    }
  }
}

// =============================================================================
// PROCESS-WIDE INSTANCE
// =============================================================================

let sharedManager: CDPSessionManager | null = null

/**
 * The one manager every CDP consumer must go through.
 *
 * Returns null when chrome.debugger is unavailable (tests, constrained extension contexts)
 * so callers fall back to their non-CDP path instead of throwing. This is the single
 * authoritative owner of debugger attachment — do not call chrome.debugger.attach directly
 * anywhere else (repo rule 18).
 */
export function cdpSessions(): CDPSessionManager | null {
  if (sharedManager) return sharedManager
  const api = globalThis.chrome?.debugger as unknown as DebuggerApi | undefined
  if (!api?.attach || !api?.detach || !api?.sendCommand || !api?.onDetach) return null
  sharedManager = new CDPSessionManager({ debuggerApi: api })
  return sharedManager
}
