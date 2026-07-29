/**
 * Purpose: Own the latest CSP probe result without coupling sync and execution modules.
 */
import type { CSPProbeResult } from '../exec/query-execution.js';
export declare function getLastCSPStatus(): CSPProbeResult;
export declare function setLastCSPStatus(status: CSPProbeResult): void;
//# sourceMappingURL=csp-state.d.ts.map