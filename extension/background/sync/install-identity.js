/**
 * Purpose: Canonically owns the daemon install identity and its durable storage.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
import { StorageKey } from '../../lib/constants.js';
import { getLocal, setLocal } from '../../lib/storage/local.js';
let serverInstallId;
export function getServerInstallId() {
    return serverInstallId;
}
export async function loadServerInstallId() {
    try {
        const stored = await getLocal(StorageKey.SERVER_INSTALL_ID);
        if (typeof stored === 'string' && stored && !serverInstallId) {
            serverInstallId = stored;
        }
    }
    catch {
        // Identity persistence is best-effort and must not block worker startup.
    }
}
export function updateServerInstallId(id) {
    if (!id || id === serverInstallId)
        return;
    serverInstallId = id;
    void setLocal(StorageKey.SERVER_INSTALL_ID, id).catch(() => {
        // A live identity remains usable when durable storage is unavailable.
    });
}
//# sourceMappingURL=install-identity.js.map