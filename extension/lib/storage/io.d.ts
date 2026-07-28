/**
 * Purpose: Normalize callback- and Promise-based Chrome storage I/O.
 * Why: Keep error handling and fire-and-forget write reporting consistent.
 */
export type StorageReadResult = Record<string, unknown>;
export type StorageReadCallback = (result: StorageReadResult) => void;
export type StorageVoidCallback = () => void;
export type StorageGetMethod = (keys: string | string[], callback?: StorageReadCallback) => Promise<StorageReadResult> | void;
export type StorageSetMethod = (items: Record<string, unknown>, callback?: StorageVoidCallback) => Promise<void> | void;
export type StorageRemoveMethod = (keys: string | string[], callback?: StorageVoidCallback) => Promise<void> | void;
export type StorageAccessLevelMethod = (options: {
    accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS';
}, callback?: StorageVoidCallback) => Promise<void> | void;
export declare function persist(write: Promise<void>, context: string): void;
export declare function readStorage(method: StorageGetMethod, keys: string | string[]): Promise<StorageReadResult>;
export declare function writeStorage(method: StorageSetMethod, items: Record<string, unknown>): Promise<void>;
export declare function removeFromStorage(method: StorageRemoveMethod, keys: string | string[]): Promise<void>;
export declare function setStorageAccessLevel(method: StorageAccessLevelMethod, accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS'): Promise<void>;
//# sourceMappingURL=io.d.ts.map