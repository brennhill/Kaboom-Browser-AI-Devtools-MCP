/**
 * Purpose: Own persistent Chrome local-storage reads and mutations.
 * Why: Keep durable state operations separate from session lifecycle state.
 */
export declare function getLocal(key: string): Promise<unknown>;
export declare function getLocals(keys: string[]): Promise<Record<string, unknown>>;
export declare function setLocal(key: string, value: unknown): Promise<void>;
export declare function setLocals(items: Record<string, unknown>): Promise<void>;
export declare function removeLocal(key: string): Promise<void>;
export declare function removeLocals(keys: string[]): Promise<void>;
//# sourceMappingURL=local.d.ts.map