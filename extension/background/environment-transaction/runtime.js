/**
 * Purpose: Composes the Chrome-backed environment transaction runtime.
 * Why: Keeps browser globals and registration side effects out of command policy modules.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import { debugLog, DebugCategory } from '../debug.js';
import { createChromeEnvironmentStateDriver } from './chrome-state-adapter.js';
import { registerEnvironmentTransactionCommands } from './commands.js';
import { createPersistentEnvironmentSnapshotStore } from './snapshot-store.js';
const driver = createChromeEnvironmentStateDriver();
const snapshots = createPersistentEnvironmentSnapshotStore({
    storage: chrome.storage.local,
    limit: 32,
    now: () => Date.now(),
    newID: () => crypto.randomUUID(),
    onNotice: (notice) => debugLog(DebugCategory.LIFECYCLE, notice)
});
registerEnvironmentTransactionCommands(driver, snapshots);
//# sourceMappingURL=runtime.js.map