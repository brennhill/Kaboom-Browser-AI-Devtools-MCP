/**
 * Purpose: Canonically owns the daemon install identity and its durable storage.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { StorageKey } from '../../lib/constants.js';
import { setLocal } from '../../lib/storage/local.js';
import { readLocalState } from '../../lib/storage/validated.js';
import { reportStateRecovery } from '../runtime-state/state-recovery.js';
let serverInstallId;
export function getServerInstallId() {
    return serverInstallId;
}
export async function loadServerInstallId() {
    const stored = await readLocalState({
        key: StorageKey.SERVER_INSTALL_ID,
        fallback: undefined,
        validate: (value) => typeof value === 'string' && value.length > 0,
        diagnostic: {
            name: 'extension_install_identity_state',
            detail: 'Saved daemon identity was invalid or unreadable; live synchronization will refresh it.',
            fix: 'Keep the extension connected to Kaboom until the next successful sync.'
        },
        report: reportStateRecovery
    });
    if (stored && !serverInstallId) {
        serverInstallId = stored;
    }
}
export function updateServerInstallId(id) {
    if (!id || id === serverInstallId)
        return;
    serverInstallId = id;
    void setLocal(StorageKey.SERVER_INSTALL_ID, id).catch(() => {
        reportStateRecovery({
            name: 'extension_install_identity_state',
            detail: 'Daemon identity could not be saved; the live identity remains active for this worker.',
            fix: 'Check extension storage permissions, then reload the extension.'
        });
    });
}
//# sourceMappingURL=install-identity.js.map