/**
 * @fileoverview Wire types for extension-daemon synchronization — matches internal/capture/wire_sync.go
 *
 * Canonical TypeScript definitions for the complete /sync request and response graph.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */
import type { ExtensionLog } from './wire-extension-log.js';
/**
 * SyncRequest is the extension heartbeat and command-lifecycle payload.
 */
export interface SyncRequest {
    readonly ext_session_id: string;
    readonly connection_generation?: number;
    readonly extension_version?: string;
    readonly settings?: SyncSettings;
    readonly extension_logs?: readonly ExtensionLog[];
    readonly last_command_ack?: string;
    readonly command_results?: readonly SyncCommandResult[];
    readonly in_progress?: readonly SyncInProgress[];
    readonly features_used?: SyncFeaturesUsed;
}
/**
 * SyncSettings contains extension settings sent with a heartbeat.
 */
export interface SyncSettings {
    readonly pilot_enabled: boolean;
    readonly tracking_enabled: boolean;
    readonly tracked_tab_id: number;
    readonly tracked_tab_url: string;
    readonly tracked_tab_title: string;
    readonly tab_status?: 'loading' | 'complete';
    readonly tracked_tab_active?: boolean;
    readonly capture_logs: boolean;
    readonly capture_network: boolean;
    readonly capture_websocket: boolean;
    readonly capture_actions: boolean;
    readonly csp_restricted: boolean;
    readonly csp_level: string;
}
/**
 * SyncCommandResult is a terminal command outcome returned by the extension.
 */
export interface SyncCommandResult {
    readonly id: string;
    readonly correlation_id?: string;
    readonly connection_generation?: number;
    readonly status: 'complete' | 'error' | 'timeout' | 'cancelled';
    readonly result?: unknown;
    readonly error?: string;
}
/**
 * SyncInProgress is active extension command execution state.
 */
export interface SyncInProgress {
    readonly id: string;
    readonly correlation_id?: string;
    readonly connection_generation: number;
    readonly type?: string;
    readonly status?: 'running' | 'pending';
    readonly progress_pct?: number | null;
    readonly started_at?: string;
    readonly updated_at?: string;
}
/**
 * SyncFeaturesUsed is the bounded UI-originated feature telemetry schema.
 */
export interface SyncFeaturesUsed {
    readonly screenshot?: boolean;
    readonly annotations?: boolean;
    readonly video?: boolean;
    readonly dom_action?: boolean;
    readonly action_recording?: boolean;
}
export declare const SYNC_UI_FEATURES: readonly ["screenshot", "annotations", "video", "dom_action", "action_recording"];
export type SyncUIFeature = (typeof SYNC_UI_FEATURES)[number];
/**
 * SyncResponse is the daemon heartbeat response and command batch.
 */
export interface SyncResponse {
    readonly ack: boolean;
    readonly connection_generation: number;
    readonly commands: readonly SyncCommand[];
    readonly next_poll_ms: number;
    readonly server_time: string;
    readonly server_version?: string;
    readonly install_id?: string;
    readonly capture_overrides: Readonly<Record<string, string>>;
}
/**
 * SyncCommand is one daemon command delivered to the extension.
 */
export interface SyncCommand {
    readonly id: string;
    readonly type: string;
    readonly params: unknown;
    readonly tab_id?: number;
    readonly correlation_id?: string;
    readonly trace_id?: string;
    readonly connection_generation: number;
}
//# sourceMappingURL=wire-sync.d.ts.map