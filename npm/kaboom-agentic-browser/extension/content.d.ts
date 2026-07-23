/**
 * Purpose: Implements the content-script bridge that forwards page telemetry to the extension background worker.
 * Why: Provides the safe boundary between page-context capture hooks and extension runtime message handling.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { getPendingRequestStats, clearPendingRequests, cleanupRequestTracking } from './content/request-tracking.js';
export { getPendingRequestStats, clearPendingRequests, cleanupRequestTracking };
//# sourceMappingURL=content.d.ts.map