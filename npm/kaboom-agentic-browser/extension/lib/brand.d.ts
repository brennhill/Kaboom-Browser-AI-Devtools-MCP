/**
 * Purpose: Shared Kaboom brand metadata and user-facing copy helpers for extension surfaces.
 * Docs: docs/features/feature/tab-tracking-ux/index.md, docs/features/feature/terminal/index.md
 */
export declare const KABOOM_DOCS_URL = "https://gokaboom.dev/docs";
export declare const KABOOM_REPOSITORY_URL = "https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP";
export declare const KABOOM_DAEMON_COMMAND = "npx kaboom-agentic-browser";
export declare const KABOOM_LOG_PREFIX = "[KaBOOM!]";
export declare const KABOOM_RECORDING_LOG_PREFIX = "[KaBOOM! REC]";
export declare const KABOOM_TELEMETRY_ENDPOINT = "https://t.gokaboom.dev/v1/event";
export declare const KABOOM_TELEMETRY_STORAGE_KEY = "kaboom_telemetry_off";
export declare const KABOOM_TELEMETRY_ENV_VAR = "KABOOM_TELEMETRY";
export declare function getTrackedTabLostToastDetail(): string;
export declare function getDaemonStartHint(): string;
export declare function getReloadedExtensionWarning(): string;
//# sourceMappingURL=brand.d.ts.map