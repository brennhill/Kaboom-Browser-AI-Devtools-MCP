/**
 * Purpose: Loads and renders the daemon's canonical System Doctor report.
 * Why: Gives users actionable readiness diagnostics without duplicating health rules in the extension.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
import type { PopupConnectionStatus } from './shell/types.js';
type DoctorFetch = (input: string) => Promise<{
    ok: boolean;
    status: number;
    json: () => Promise<unknown>;
}>;
export declare function refreshSystemDoctor(status: Pick<PopupConnectionStatus, 'connected' | 'serverUrl'>, fetchImpl?: DoctorFetch): Promise<void>;
export {};
//# sourceMappingURL=system-doctor.d.ts.map