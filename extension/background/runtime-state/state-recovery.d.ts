/**
 * Purpose: Own extension persisted-state recovery diagnostics sent to the daemon.
 * Why: Keeps raw state out of logs while making fallbacks visible in System Doctor.
 */
import type { StateRecoveryDiagnostic, StateRecoveryLifecycle } from '../../types/runtime-messages.js';
export declare function reportStateRecovery(diagnostic: StateRecoveryDiagnostic, lifecycle?: StateRecoveryLifecycle): void;
export declare function resolveStateRecovery(name: string): void;
//# sourceMappingURL=state-recovery.d.ts.map