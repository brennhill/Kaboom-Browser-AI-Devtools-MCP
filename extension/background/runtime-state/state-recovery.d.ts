/**
 * Purpose: Own extension persisted-state recovery diagnostics sent to the daemon.
 * Why: Keeps raw state out of logs while making fallbacks visible in System Doctor.
 */
import type { StateRecoveryDiagnostic } from '../../types/runtime-messages.js';
export declare function reportStateRecovery(diagnostic: StateRecoveryDiagnostic): void;
//# sourceMappingURL=state-recovery.d.ts.map