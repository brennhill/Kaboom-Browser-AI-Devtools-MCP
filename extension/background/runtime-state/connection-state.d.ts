/**
 * Purpose: Own connection lifecycle state for the background worker.
 * Why: Connection checks and health updates change together independently of user settings.
 */
import type { ConnectionStatus } from '../../types/runtime/state.js';
export type MutableConnectionStatus = {
    -readonly [Key in keyof ConnectionStatus]: ConnectionStatus[Key];
};
export declare function getConnectionStatus(): Readonly<MutableConnectionStatus>;
export declare function setConnectionStatus(patch: Partial<MutableConnectionStatus>): void;
export declare function isConnectionCheckRunning(): boolean;
export declare function setConnectionCheckRunning(running: boolean): void;
export declare function applyConnectionOverrides(overrides: Readonly<Record<string, string>>): void;
//# sourceMappingURL=connection-state.d.ts.map