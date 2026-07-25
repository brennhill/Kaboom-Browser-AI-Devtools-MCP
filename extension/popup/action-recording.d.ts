/**
 * Purpose: Popup UI module for action workflow (event) recording — start/stop via daemon HTTP API.
 * Why: Separates event recording controls from screen recording, keeping each feature self-contained.
 * Docs: docs/features/feature/flow-recording/index.md
 */
/**
 * Decide whether a persisted "recording in progress" is still real (F5).
 *
 * An event recording lives only in the daemon's memory, so a daemon restart
 * discards it and leaves the ACTION_RECORDING mirror as a phantom — a running
 * timer over a recording that no longer exists (Class 2: stale mirror as source
 * of truth). We detect a restart by comparing the daemon PID captured when the
 * recording started against the daemon's current PID.
 *
 * DESTRUCTIVE-SAFE / FAIL-OPEN: the caller deletes the mirror on `false`, so we
 * only return `false` (stale) when CONFIDENT the daemon restarted — both PIDs
 * known and different. Any uncertainty (no captured baseline, daemon unreachable,
 * non-2xx, unparseable health) returns `true` (keep the mirror), so a transient
 * blip never orphans a live recording. An earlier uptime-vs-wall-clock check
 * deleted live recordings after laptop sleep or an NTP/DST jump, because the
 * daemon's uptime is a MONOTONIC clock that diverges from the browser wall clock.
 */
export declare function isActionRecordingStillLive(startedPid: number | null | undefined): Promise<boolean>;
export declare function setupActionRecordingUI(): void;
//# sourceMappingURL=action-recording.d.ts.map