/**
 * Purpose: Handles file upload queries by fetching file data from the Go server and injecting it into DOM file inputs via DataTransfer or OS automation escalation.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { PendingQuery } from '../types/queries.js';
import type { SyncClient } from './sync-client.js';
import type { SendAsyncResultFn, ActionToastFn } from './pending-queries.js';
interface VerifyResult {
    has_file: boolean;
    file_name?: string;
    file_size?: number;
}
interface ClickResult {
    clicked: boolean;
    error?: string;
}
interface EscalationResult {
    success: boolean;
    stage: number;
    escalation_reason?: string;
    file_name?: string;
    error?: string;
}
/**
 * Verify whether a file persists on the input element after Stage 1 injection.
 * Sleeps BEFORE each check so frameworks with async onChange have time to clear.
 * If the file disappears at any check, returns has_file: false immediately.
 * If it survives all checks (~4.6s window), Stage 1 is confirmed.
 */
export declare function verifyFileOnInput(tabId: number, selector: string): Promise<VerifyResult>;
/**
 * Click a file input element to open the native file dialog.
 */
export declare function clickFileInput(tabId: number, selector: string): Promise<ClickResult>;
/**
 * Escalate to Stage 4 OS automation: click file input, call daemon, verify result.
 */
export declare function escalateToStage4(tabId: number, selector: string, filePath: string, serverUrl: string): Promise<EscalationResult>;
export declare function executeUpload(query: PendingQuery, tabId: number, syncClient: SyncClient, sendAsyncResult: SendAsyncResultFn, actionToast: ActionToastFn): Promise<void>;
export {};
//# sourceMappingURL=upload-handler.d.ts.map