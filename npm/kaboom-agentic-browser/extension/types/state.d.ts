/**
 * Purpose: Defines extension runtime state contracts for snapshots, health, circuit-breaker, and memory-pressure reporting.
 * Why: Keeps status/state payloads consistent across background logic, popup rendering, and query responses.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
/**
 * @fileoverview State Management Types
 * Browser state snapshots, circuit breakers, and memory pressure
 */
/**
 * Browser state snapshot
 */
export interface BrowserStateSnapshot {
    readonly url: string;
    readonly timestamp: number;
    readonly localStorage: Readonly<Record<string, string>>;
    readonly sessionStorage: Readonly<Record<string, string>>;
    readonly cookies: string;
}
/**
 * Saved state snapshot with metadata
 */
export interface SavedStateSnapshot extends BrowserStateSnapshot {
    readonly name: string;
    readonly size_bytes: number;
}
/**
 * State action types
 */
export type StateAction = 'capture' | 'save' | 'load' | 'list' | 'delete' | 'restore';
/**
 * Circuit breaker states
 */
export type CircuitBreakerState = 'closed' | 'open' | 'half-open';
/**
 * Circuit breaker statistics
 */
export interface CircuitBreakerStats {
    readonly state: CircuitBreakerState;
    readonly consecutiveFailures: number;
    readonly totalFailures: number;
    readonly totalSuccesses: number;
    readonly currentBackoff: number;
}
/**
 * Memory pressure levels
 */
export type MemoryPressureLevel = 'normal' | 'soft' | 'hard';
/**
 * Memory pressure state
 */
export interface MemoryPressureState {
    readonly memoryPressureLevel: MemoryPressureLevel;
    readonly lastMemoryCheck: number;
    readonly networkBodyCaptureDisabled: boolean;
    readonly reducedCapacities: boolean;
}
/**
 * Connection status
 */
export interface ConnectionStatus {
    readonly connected: boolean;
    readonly entries: number;
    readonly maxEntries: number;
    readonly errorCount: number;
    readonly logFile: string;
    readonly logFileSize?: number;
    readonly serverVersion?: string;
    readonly extensionVersion?: string;
    readonly versionMismatch?: boolean;
    readonly securityMode?: 'normal' | 'insecure_proxy';
    readonly productionParity?: boolean;
    readonly insecureRewritesApplied?: readonly string[];
}
/**
 * Context annotation warning
 */
export interface ContextWarning {
    readonly sizeKB: number;
    readonly count: number;
    readonly triggeredAt: number;
}
/**
 * Error group for deduplication
 */
export interface ErrorGroup {
    readonly entry: import('./telemetry.js').LogEntry;
    readonly count: number;
    readonly firstSeen: number;
    readonly lastSeen: number;
}
/**
 * Rate limit check result
 */
export interface RateLimitResult {
    readonly allowed: boolean;
    readonly reason?: 'session_limit' | 'rate_limit';
    readonly nextAllowedIn?: number | null;
}
/**
 * Capture screenshot result
 */
export interface CaptureScreenshotResult {
    readonly success: boolean;
    readonly entry?: import('./telemetry.js').ScreenshotLogEntry;
    readonly error?: string;
    readonly nextAllowedIn?: number;
}
//# sourceMappingURL=state.d.ts.map