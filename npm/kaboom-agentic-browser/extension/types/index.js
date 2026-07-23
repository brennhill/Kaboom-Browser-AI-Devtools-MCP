/**
 * Purpose: Exposes the canonical extension type barrel that aggregates runtime, telemetry, and utility contracts.
 * Why: Provides a stable import surface so cross-module typing remains consistent during refactors.
 * Docs: docs/features/feature/query-service/index.md
 */
// Re-export type guards from utils
export { isObject, isNonEmptyString, hasType, isJsonValue, createTypeGuard } from './utils.js';
//# sourceMappingURL=index.js.map