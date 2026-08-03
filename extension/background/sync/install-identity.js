/**
 * Purpose: Canonically owns the daemon install identity and its durable storage.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { StorageKey } from '../../lib/constants.js';
import { classifyStorageFailure, storageFaultDetail } from '../../lib/storage/fault.js';
import { setLocal } from '../../lib/storage/local.js';
import { readLocalState } from '../../lib/storage/validated.js';
import { reportStateRecovery, resolveStateRecovery } from '../runtime-state/state-recovery.js';
let serverInstallId;
const SERVER_INSTALL_ID = /^[0-9a-f]{12}$/;
function isServerInstallId(value) {
    return typeof value === 'string' && SERVER_INSTALL_ID.test(value);
}
export function getServerInstallId() {
    return serverInstallId;
}
export async function loadServerInstallId() {
    const stored = await readLocalState({
        key: StorageKey.SERVER_INSTALL_ID,
        fallback: undefined,
        validate: isServerInstallId,
        diagnostic: {
            name: 'extension_install_identity_state',
            detail: 'Saved daemon identity was invalid or unreadable; live synchronization will refresh it.',
            fix: 'Keep the extension connected to Kaboom until the next successful sync.'
        },
        report: reportStateRecovery,
        resolve: resolveStateRecovery
    });
    if (stored && !serverInstallId) {
        serverInstallId = stored;
    }
}
export function updateServerInstallId(id) {
    if (!isServerInstallId(id)) {
        reportStateRecovery({
            name: 'extension_install_identity_state',
            detail: storageFaultDetail('corruption', 'The live daemon identity was invalid and was not cached.'),
            fix: 'Restart the Kaboom daemon and reconnect the extension.'
        });
        return;
    }
    if (id === serverInstallId)
        return;
    serverInstallId = id;
    void setLocal(StorageKey.SERVER_INSTALL_ID, id).catch((error) => {
        reportStateRecovery({
            name: 'extension_install_identity_state',
            detail: storageFaultDetail(classifyStorageFailure(error, 'write'), 'Daemon identity could not be saved; the live identity remains active for this worker.'),
            fix: 'Check extension storage permissions, then reload the extension.'
        });
    });
}
//# sourceMappingURL=install-identity.js.map