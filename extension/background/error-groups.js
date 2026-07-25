/**
 * Purpose: Deduplicates and groups identical errors within configurable time windows to reduce server traffic.
 * Docs: docs/features/feature/error-clustering/index.md
 */
import { StorageKey } from '../lib/constants.js';
import { getSession, setSession, persist } from '../lib/storage-utils.js';
// =============================================================================
// CONSTANTS
// =============================================================================
/** Error deduplication window in milliseconds */
const ERROR_DEDUP_WINDOW_MS = 5000;
/** Error group flush interval in milliseconds */
const ERROR_GROUP_FLUSH_MS = 10000;
/** Maximum tracked error groups */
const MAX_TRACKED_ERRORS = 100;
/** Error group max age - cleanup after 1 hour */
export const ERROR_GROUP_MAX_AGE_MS = 3600000;
// =============================================================================
// STATE
// =============================================================================
/** Error grouping state */
const errorGroups = new Map();
// =============================================================================
// SESSION STORAGE PERSISTENCE
// =============================================================================
/** Debounce timer for session storage writes */
let persistTimer = null;
/** Debounce interval for writing error groups to session storage (ms) */
const PERSIST_DEBOUNCE_MS = 2000;
/**
 * Debounced write of error group state to chrome.storage.session.
 * Only persists signatures and counts — full LogEntry details are not stored
 * since they rebuild naturally on next occurrence.
 */
function schedulePersist() {
    if (persistTimer !== null)
        clearTimeout(persistTimer);
    persistTimer = setTimeout(() => {
        persistTimer = null;
        const snapshot = [];
        for (const [signature, group] of errorGroups) {
            snapshot.push({
                signature,
                count: group.count,
                firstSeen: group.firstSeen,
                lastSeen: group.lastSeen
            });
        }
        persist(setSession(StorageKey.ERROR_GROUPS, snapshot), 'error-groups');
    }, PERSIST_DEBOUNCE_MS);
}
/**
 * Restore error groups from session storage after service worker restart.
 * Only restores dedup windows (signature + timing) — the original LogEntry
 * is not persisted, so the first new occurrence will populate it.
 */
async function restoreFromSession() {
    try {
        const raw = await getSession(StorageKey.ERROR_GROUPS);
        if (!Array.isArray(raw))
            return;
        const now = Date.now();
        for (const item of raw) {
            // Skip groups that have aged out
            if (now - item.lastSeen > ERROR_GROUP_MAX_AGE_MS)
                continue;
            // Restore with a placeholder entry — next real error will overwrite it
            errorGroups.set(item.signature, {
                entry: { level: 'error' },
                count: item.count,
                firstSeen: item.firstSeen,
                lastSeen: item.lastSeen
            });
        }
    }
    catch {
        // Session storage may not be available — degrade silently
    }
}
// Restore on module load (top-level await is fine in MV3 service worker modules)
void restoreFromSession();
const SIGNATURE_EXTRACTORS = {
    exception: (entry) => {
        const exEntry = entry;
        const parts = [exEntry.message || ''];
        if (exEntry.stack) {
            const firstFrame = exEntry.stack.split('\n')[1] || '';
            parts.push(firstFrame.trim());
        }
        return parts;
    },
    network: (entry) => {
        const netEntry = entry;
        const parts = [netEntry.method || 'GET'];
        try {
            const url = new URL(netEntry.url || '', 'http://localhost');
            parts.push(url.pathname);
        }
        catch {
            parts.push(netEntry.url || '');
        }
        parts.push(String(netEntry.status || 0));
        return parts;
    },
    console: (entry) => {
        const consEntry = entry;
        if (consEntry.args && consEntry.args.length > 0) {
            const firstArg = consEntry.args[0];
            return [typeof firstArg === 'string' ? firstArg.slice(0, 200) : JSON.stringify(firstArg).slice(0, 200)];
        }
        return [];
    }
};
export function createErrorSignature(entry) {
    const entryType = entry.type || 'unknown';
    const parts = [entryType, entry.level || 'error'];
    const extractor = SIGNATURE_EXTRACTORS[entryType];
    if (extractor)
        parts.push(...extractor(entry));
    return parts.join('|');
}
/**
 * Process an error through the grouping system
 */
function handleExistingGroup(group, entry, now) {
    if (now - group.lastSeen < ERROR_DEDUP_WINDOW_MS) {
        group.count++;
        group.lastSeen = now;
        group.entry = entry; // Keep entry fresh for flush
        schedulePersist();
        return { shouldSend: false };
    }
    const countToReport = group.count;
    group.count = 1;
    group.lastSeen = now;
    group.firstSeen = now;
    group.entry = entry;
    schedulePersist();
    if (countToReport > 1) {
        return {
            shouldSend: true,
            entry: { ...entry, _previousOccurrences: countToReport - 1 }
        };
    }
    return { shouldSend: true, entry };
}
function evictOldestGroup() {
    if (errorGroups.size < MAX_TRACKED_ERRORS)
        return;
    let oldestSig = null;
    let oldestTime = Infinity;
    for (const [sig, group] of errorGroups) {
        if (group.lastSeen < oldestTime) {
            oldestTime = group.lastSeen;
            oldestSig = sig;
        }
    }
    if (oldestSig)
        errorGroups.delete(oldestSig);
}
export function processErrorGroup(entry) {
    if (entry.level !== 'error' && entry.level !== 'warn') {
        return { shouldSend: true, entry };
    }
    const signature = createErrorSignature(entry);
    const now = Date.now();
    const existing = errorGroups.get(signature);
    if (existing)
        return handleExistingGroup(existing, entry, now);
    evictOldestGroup();
    errorGroups.set(signature, { entry, count: 1, firstSeen: now, lastSeen: now });
    schedulePersist();
    return { shouldSend: true, entry };
}
/**
 * Get current state of error groups (for testing)
 */
export function getErrorGroupsState() {
    return errorGroups;
}
/**
 * Clean up stale error groups older than ERROR_GROUP_MAX_AGE_MS
 */
export function cleanupStaleErrorGroups(debugLogFn) {
    const now = Date.now();
    let cleaned = false;
    for (const [signature, group] of errorGroups) {
        if (now - group.lastSeen > ERROR_GROUP_MAX_AGE_MS) {
            errorGroups.delete(signature);
            cleaned = true;
            if (debugLogFn) {
                debugLogFn('error', 'Cleaned up stale error group', {
                    signature: signature.slice(0, 50) + '...',
                    age: Math.round((now - group.lastSeen) / 60000) + ' min'
                });
            }
        }
    }
    if (cleaned)
        schedulePersist();
}
/**
 * Flush error groups - send any accumulated counts
 */
export function flushErrorGroups() {
    const now = Date.now();
    const entriesToSend = [];
    for (const [signature, group] of errorGroups) {
        if (group.count > 1) {
            // Spread the readonly entry and add our mutable properties
            const processedEntry = {
                ...group.entry,
                _aggregatedCount: group.count,
                _firstSeen: new Date(group.firstSeen).toISOString(),
                _lastSeen: new Date(group.lastSeen).toISOString()
            };
            processedEntry.ts = new Date().toISOString();
            entriesToSend.push(processedEntry);
            group.count = 0;
        }
        if (now - group.lastSeen > ERROR_GROUP_FLUSH_MS * 2) {
            errorGroups.delete(signature);
        }
    }
    if (entriesToSend.length > 0)
        schedulePersist();
    return entriesToSend;
}
//# sourceMappingURL=error-groups.js.map