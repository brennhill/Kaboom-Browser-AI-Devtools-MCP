/**
 * Purpose: Classify persisted-state failures consistently across extension owners.
 * Why: Doctor diagnostics need stable, redacted failure transitions without test hooks in production.
 */
export declare const STORAGE_FAULT_KINDS: readonly ["read", "write", "sync", "rename", "directory_sync", "quota", "corruption", "partial_write", "cancellation", "restart"];
export type StorageFaultKind = (typeof STORAGE_FAULT_KINDS)[number];
export declare function classifyStorageFailure(error: unknown, operation: 'read' | 'write'): StorageFaultKind;
export declare function storageFaultDetail(kind: StorageFaultKind, consequence: string): string;
//# sourceMappingURL=fault.d.ts.map