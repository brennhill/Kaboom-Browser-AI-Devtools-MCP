/**
 * Purpose: Adapts Chrome tab, cookie, scripting, and window APIs to the QA fixture state driver.
 * Why: Keeps browser API details out of transaction policy and makes every I/O boundary replaceable in tests.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import { type BrowserStateDriver, type BrowserStateDriverDeps } from './browser-state-driver.js';
export declare function createChromeBrowserStateDriver(): BrowserStateDriver;
export declare function chromeDriverDeps(): BrowserStateDriverDeps;
//# sourceMappingURL=chrome-state-adapter.d.ts.map