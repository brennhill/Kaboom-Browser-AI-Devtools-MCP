/**
 * Purpose: Captures, applies, and restores the browser state touched by a QA fixture.
 * Why: Keeps private fixture state inside one extension boundary with deterministic mutation order.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */

import type { WireQACookie, WireQAFixture } from '../../types/wire/wire-qa-fixture.js'

export interface FixturePageState {
  readonly local_storage: Readonly<Record<string, string | null>>
  readonly session_storage: Readonly<Record<string, string | null>>
  readonly feature_flags: Readonly<Record<string, string | null>>
  readonly seed_data: Readonly<Record<string, string | null>>
}

export interface FixtureSnapshot {
  readonly tab_url: string
  readonly window_id: number
  readonly window_bounds?: { readonly width: number; readonly height: number }
  readonly page_state: FixturePageState
  readonly cookies: readonly WireQACookie[]
}

export interface FixtureMutationCounts {
  readonly cookies: number
  readonly local_storage: number
  readonly session_storage: number
  readonly feature_flags: number
  readonly seed_data: number
}

interface TabState {
  readonly url?: string
  readonly windowId: number
}

interface WindowBounds {
  readonly width?: number
  readonly height?: number
}

export interface BrowserStateDriverDeps {
  readonly getTab: (tabId: number) => Promise<TabState>
  readonly getWindow: (windowId: number) => Promise<WindowBounds>
  readonly capturePageState: (tabId: number, fixture: WireQAFixture) => Promise<FixturePageState>
  readonly getCookie: (url: string, name: string) => Promise<WireQACookie | null>
  readonly navigate: (tabId: number, url: string, timeoutMs: number) => Promise<void>
  readonly resizeViewport: (tabId: number, windowId: number, width: number, height: number) => Promise<void>
  readonly restoreWindow: (windowId: number, width: number, height: number) => Promise<void>
  readonly setCookie: (cookie: WireQACookie, url: string) => Promise<void>
  readonly removeCookie: (url: string, name: string) => Promise<void>
  readonly applyPageState: (tabId: number, fixture: WireQAFixture) => Promise<void>
  readonly restorePageState: (tabId: number, state: FixturePageState) => Promise<void>
}

export interface BrowserStateDriver {
  readonly snapshot: (tabId: number, fixture: WireQAFixture) => Promise<FixtureSnapshot>
  readonly apply: (tabId: number, fixture: WireQAFixture) => Promise<FixtureMutationCounts>
  readonly restore: (tabId: number, fixture: WireQAFixture, snapshot: FixtureSnapshot) => Promise<void>
}

const EMPTY_PAGE_STATE: FixturePageState = {
  local_storage: {},
  session_storage: {},
  feature_flags: {},
  seed_data: {}
}

export function unsupportedFixtureCapabilities(fixture: WireQAFixture): string[] {
  const unsupported: string[] = []
  if (fixture.locale) unsupported.push('locale')
  if (fixture.permissions && fixture.permissions.length > 0) unsupported.push('permissions')
  if (fixture.network?.profile) unsupported.push('network')
  return unsupported
}

export function createBrowserStateDriver(deps: BrowserStateDriverDeps): BrowserStateDriver {
  return {
    snapshot: async (tabId, fixture) => {
      assertStaticCapabilities(fixture)
      const tab = await deps.getTab(tabId)
      const tabUrl = requireHTTPURL(tab.url)
      const targetUrl = fixture.target?.url ?? tabUrl
      assertSameOriginForPageState(tabUrl, targetUrl, fixture)

      let windowBounds: FixtureSnapshot['window_bounds']
      if (hasViewport(fixture)) {
        const current = await deps.getWindow(tab.windowId)
        if (current.width === undefined || current.height === undefined) {
          throw new Error('window_bounds_unavailable')
        }
        windowBounds = { width: current.width, height: current.height }
      }

      const pageState = hasPageState(fixture) ? await deps.capturePageState(tabId, fixture) : EMPTY_PAGE_STATE
      const cookies: WireQACookie[] = []
      for (const cookie of fixture.cookies ?? []) {
        const existing = await deps.getCookie(targetUrl, cookie.name)
        if (existing) cookies.push(existing)
      }
      return {
        tab_url: tabUrl,
        window_id: tab.windowId,
        ...(windowBounds ? { window_bounds: windowBounds } : {}),
        page_state: pageState,
        cookies
      }
    },

    apply: async (tabId, fixture) => {
      assertStaticCapabilities(fixture)
      const targetUrl = fixture.target?.url
      if (targetUrl) await deps.navigate(tabId, targetUrl, fixture.setup_timeout_ms ?? 10_000)
      if (hasViewport(fixture)) {
        await deps.resizeViewport(
          tabId,
          (await deps.getTab(tabId)).windowId,
          fixture.viewport?.width ?? 0,
          fixture.viewport?.height ?? 0
        )
      }
      if ((fixture.cookies?.length ?? 0) > 0) {
        const cookieUrl = targetUrl ?? requireHTTPURL((await deps.getTab(tabId)).url)
        for (const cookie of fixture.cookies ?? []) await deps.setCookie(cookie, cookieUrl)
      }
      if (hasPageState(fixture)) await deps.applyPageState(tabId, fixture)
      return mutationCounts(fixture)
    },

    restore: async (tabId, fixture, snapshot) => {
      let failures = 0
      const attempt = async (operation: () => Promise<void>): Promise<void> => {
        try {
          await operation()
        } catch {
          // EXPECTED_AGGREGATION: every restore failure is surfaced through the
          // final stable error after all independent recovery steps are tried.
          failures++
        }
      }
      if (fixture.target?.url && fixture.target.url !== snapshot.tab_url) {
        await attempt(() => deps.navigate(tabId, snapshot.tab_url, fixture.setup_timeout_ms ?? 10_000))
      }
      if (snapshot.window_bounds) {
        const bounds = snapshot.window_bounds
        await attempt(() => deps.restoreWindow(snapshot.window_id, bounds.width, bounds.height))
      }
      const cookieUrl = fixture.target?.url ?? snapshot.tab_url
      for (const changed of fixture.cookies ?? []) await attempt(() => deps.removeCookie(cookieUrl, changed.name))
      for (const cookie of snapshot.cookies) await attempt(() => deps.setCookie(cookie, cookieUrl))
      if (hasPageState(fixture)) await attempt(() => deps.restorePageState(tabId, snapshot.page_state))
      if (failures > 0) throw new Error('fixture_restore_failed')
    }
  }
}

function assertStaticCapabilities(fixture: WireQAFixture): void {
  if (unsupportedFixtureCapabilities(fixture).length > 0) throw new Error('unsupported_fixture_capabilities')
}

function requireHTTPURL(value: string | undefined): string {
  if (!value) throw new Error('fixture_tab_url_unavailable')
  const parsed = new URL(value)
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') throw new Error('fixture_tab_url_unsupported')
  return parsed.href
}

function assertSameOriginForPageState(currentUrl: string, targetUrl: string, fixture: WireQAFixture): void {
  if (!hasPageState(fixture)) return
  if (new URL(currentUrl).origin !== new URL(targetUrl).origin) throw new Error('cross_origin_page_state_unsupported')
}

function hasPageState(fixture: WireQAFixture): boolean {
  return (
    Object.keys(fixture.local_storage ?? {}).length > 0 ||
    Object.keys(fixture.session_storage ?? {}).length > 0 ||
    Object.keys(fixture.feature_flags ?? {}).length > 0 ||
    Object.keys(fixture.seed_data ?? {}).length > 0
  )
}

function hasViewport(fixture: WireQAFixture): boolean {
  return (fixture.viewport?.width ?? 0) > 0 && (fixture.viewport?.height ?? 0) > 0
}

function mutationCounts(fixture: WireQAFixture): FixtureMutationCounts {
  return {
    cookies: fixture.cookies?.length ?? 0,
    local_storage: Object.keys(fixture.local_storage ?? {}).length,
    session_storage: Object.keys(fixture.session_storage ?? {}).length,
    feature_flags: Object.keys(fixture.feature_flags ?? {}).length,
    seed_data: Object.keys(fixture.seed_data ?? {}).length
  }
}
