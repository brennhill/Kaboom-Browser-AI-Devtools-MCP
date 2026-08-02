/**
 * Purpose: Captures, applies, and restores the browser state touched by a QA fixture.
 * Why: Keeps private fixture state inside one extension boundary with deterministic mutation order.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import type { WireQACookie, WireQAFixture } from '../../types/wire/wire-qa-fixture.js';
export interface FixturePageState {
    readonly local_storage: Readonly<Record<string, string | null>>;
    readonly session_storage: Readonly<Record<string, string | null>>;
    readonly feature_flags: Readonly<Record<string, string | null>>;
    readonly seed_data: Readonly<Record<string, string | null>>;
}
export interface FixtureSnapshot {
    readonly tab_url: string;
    readonly window_id: number;
    readonly window_bounds?: {
        readonly width: number;
        readonly height: number;
    };
    readonly page_state: FixturePageState;
    readonly cookies: readonly WireQACookie[];
}
export interface FixtureMutationCounts {
    readonly cookies: number;
    readonly local_storage: number;
    readonly session_storage: number;
    readonly feature_flags: number;
    readonly seed_data: number;
}
interface TabState {
    readonly url?: string;
    readonly windowId: number;
}
interface WindowBounds {
    readonly width?: number;
    readonly height?: number;
}
export interface BrowserStateDriverDeps {
    readonly getTab: (tabId: number) => Promise<TabState>;
    readonly getWindow: (windowId: number) => Promise<WindowBounds>;
    readonly capturePageState: (tabId: number, fixture: WireQAFixture) => Promise<FixturePageState>;
    readonly getCookie: (url: string, name: string) => Promise<WireQACookie | null>;
    readonly navigate: (tabId: number, url: string, timeoutMs: number) => Promise<void>;
    readonly resizeViewport: (tabId: number, windowId: number, width: number, height: number) => Promise<void>;
    readonly restoreWindow: (windowId: number, width: number, height: number) => Promise<void>;
    readonly setCookie: (cookie: WireQACookie, url: string) => Promise<void>;
    readonly removeCookie: (url: string, name: string) => Promise<void>;
    readonly applyPageState: (tabId: number, fixture: WireQAFixture) => Promise<void>;
    readonly restorePageState: (tabId: number, state: FixturePageState) => Promise<void>;
}
export interface BrowserStateDriver {
    readonly snapshot: (tabId: number, fixture: WireQAFixture) => Promise<FixtureSnapshot>;
    readonly apply: (tabId: number, fixture: WireQAFixture) => Promise<FixtureMutationCounts>;
    readonly restore: (tabId: number, fixture: WireQAFixture, snapshot: FixtureSnapshot) => Promise<void>;
}
export declare function unsupportedFixtureCapabilities(fixture: WireQAFixture): string[];
export declare function createBrowserStateDriver(deps: BrowserStateDriverDeps): BrowserStateDriver;
export {};
//# sourceMappingURL=browser-state-driver.d.ts.map