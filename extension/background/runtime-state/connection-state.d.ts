/**
 * Purpose: Own connection lifecycle state for the background worker.
 * Why: Connection checks and health updates change together independently of user settings.
 */
import type { ConnectionStatus } from '../../types/runtime/state.js';
export type MutableConnectionStatus = {
    -readonly [Key in keyof ConnectionStatus]: ConnectionStatus[Key];
};
/** Called on each edge of the daemon sync connection, never on repeats of the same value. */
export type ExtensionConnectionListener = (connected: boolean) => void;
export declare function getConnectionStatus(): Readonly<MutableConnectionStatus>;
/**
 * Observe the daemon sync connection edge. This is the authoritative live signal
 * that an MCP client session started or ended — owners of session-scoped browser
 * state (the driven tab group) subscribe here instead of mirroring the state in
 * storage, which goes stale when the worker dies without flushing (rule 18).
 * Returns an unsubscribe function.
 */
export declare function subscribeExtensionConnection(listener: ExtensionConnectionListener): () => void;
export declare function setConnectionStatus(patch: Partial<MutableConnectionStatus>): void;
export declare function isConnectionCheckRunning(): boolean;
export declare function setConnectionCheckRunning(running: boolean): void;
export declare function applyConnectionOverrides(overrides: Readonly<Record<string, string>>): void;
//# sourceMappingURL=connection-state.d.ts.map