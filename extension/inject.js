/**
 * Purpose: Installs page-context browser telemetry and automation hooks.
 * Why: Keeps the injected runtime entrypoint limited to deterministic startup orchestration.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
import { sendPerformanceSnapshot } from './lib/analysis/perf-snapshot.js';
import { installKaboomAPI } from './inject/api.js';
import { installMessageListener } from './inject/message-handlers.js';
import { installPhase1 } from './inject/observers.js';
import { captureState, restoreState } from './inject/state.js';
if (typeof window !== 'undefined' &&
    typeof document !== 'undefined' &&
    typeof globalThis.process === 'undefined') {
    installPhase1();
    installMessageListener(captureState, restoreState);
    installKaboomAPI();
    window.addEventListener('load', () => {
        setTimeout(() => {
            sendPerformanceSnapshot();
        }, 2000);
    });
}
//# sourceMappingURL=inject.js.map