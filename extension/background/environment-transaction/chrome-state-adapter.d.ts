/**
 * Purpose: Adapts Chrome tab, cookie, scripting, and window APIs to the environment transaction state driver.
 * Why: Keeps browser API details out of transaction policy and makes every I/O boundary replaceable in tests.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import { type EnvironmentStateDriver, type EnvironmentStateDriverDeps } from './browser-state-driver.js';
export declare function createChromeEnvironmentStateDriver(): EnvironmentStateDriver;
export declare function chromeDriverDeps(): EnvironmentStateDriverDeps;
//# sourceMappingURL=chrome-state-adapter.d.ts.map