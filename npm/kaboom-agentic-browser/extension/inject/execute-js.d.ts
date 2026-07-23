/**
 * Purpose: JavaScript execution sandbox for evaluating arbitrary scripts in page context with safe serialization and timeout support.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { ExecuteJsResult } from '../types/index.js';
export declare function safeSerializeForExecute(value: unknown, depth?: number, seen?: WeakSet<object>): unknown;
/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export declare function executeJavaScript(script: string, timeoutMs?: number): Promise<ExecuteJsResult>;
//# sourceMappingURL=execute-js.d.ts.map