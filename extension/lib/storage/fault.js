/**
 * Purpose: Classify persisted-state failures consistently across extension owners.
 * Why: Doctor diagnostics need stable, redacted failure transitions without test hooks in production.
 */
export const STORAGE_FAULT_KINDS = [
    'read',
    'write',
    'sync',
    'rename',
    'directory_sync',
    'quota',
    'corruption',
    'partial_write',
    'cancellation',
    'restart'
];
export function classifyStorageFailure(error, operation) {
    const name = error instanceof Error ? error.name.toLowerCase() : '';
    const message = error instanceof Error ? error.message.toLowerCase() : '';
    if (name === 'aborterror')
        return 'cancellation';
    if (name.includes('quota') || message.includes('quota'))
        return 'quota';
    return operation;
}
export function storageFaultDetail(kind, consequence) {
    return `Extension state ${kind} failure; ${consequence}`;
}
//# sourceMappingURL=fault.js.map