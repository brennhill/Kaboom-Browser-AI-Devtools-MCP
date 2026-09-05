/**
 * Purpose: Composes the Chrome-backed environment transaction runtime.
 * Why: Keeps browser globals and registration side effects out of command policy modules.
 * Docs: docs/features/feature/environment-manipulation/index.md
 */
import { debugLog, DebugCategory } from '../debug.js';
import { createChromeEnvironmentStateDriver } from './chrome-state-adapter.js';
import { registerEnvironmentTransactionCommands } from './commands.js';
import { registerEnvironmentPinCommands } from './env-pin.js';
import { createPersistentEnvironmentSnapshotStore } from './snapshot-store.js';
import { reportStateRecovery } from '../runtime-state/state-recovery.js';
export function initializeEnvironmentTransactionRuntime() {
    const driver = createChromeEnvironmentStateDriver();
    const snapshots = createPersistentEnvironmentSnapshotStore({
        storage: chrome.storage.local,
        limit: 32,
        now: () => Date.now(),
        newID: () => crypto.randomUUID(),
        onNotice: (notice) => {
            debugLog(DebugCategory.LIFECYCLE, `${notice.code} (${notice.fault_kind})`);
            reportStateRecovery({
                name: 'environment_snapshot_state',
                detail: `Environment snapshot ${notice.fault_kind} failure (${notice.code}); private snapshot values were not logged.`,
                fix: 'Retry the environment action; if it recurs, clear QA environment snapshots and inspect System Doctor.'
            });
        }
    });
    registerEnvironmentTransactionCommands(driver, snapshots);
    registerEnvironmentPinCommands();
}
//# sourceMappingURL=runtime.js.map