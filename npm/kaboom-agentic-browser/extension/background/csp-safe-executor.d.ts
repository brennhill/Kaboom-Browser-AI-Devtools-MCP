/**
 * CSP-Safe Structured Command Executor
 *
 * This function is injected into the page's MAIN world via:
 *   chrome.scripting.executeScript({ world: "MAIN", func: cspSafeExecutor, args: [command] })
 *
 * It MUST be fully self-contained — no closures over module-level variables.
 * Chrome serializes the function source at injection time. Any external reference
 * will be undefined at runtime.
 *
 * WHY THIS BYPASSES CSP:
 * Chrome's extension API injects the function natively (same mechanism as content
 * scripts declared in manifest.json). The page's CSP governs scripts loaded BY
 * the page — it has no authority over Chrome's extension injection pipeline.
 * The command argument is JSON data, not code. The executor resolves property
 * paths via bracket notation and calls methods via .apply() — standard JS
 * operations that CSP cannot restrict.
 *
 * IMPORTANT: this binding
 * DOM methods require correct `this` (e.g., document.querySelector needs
 * this === document). The executor tracks the parent object through the chain
 * and uses fn.apply(parent, args) for call steps.
 */
interface ExecutorStep {
    op: 'access' | 'index' | 'call' | 'construct';
    key?: string;
    index?: number;
    args?: ExecutorValue[];
}
interface ExecutorValue {
    type: 'literal' | 'undefined' | 'global' | 'chain' | 'array' | 'object';
    value?: string | number | boolean | null;
    name?: string;
    root?: ExecutorValue;
    steps?: ExecutorStep[];
    elements?: ExecutorValue[];
    entries?: Array<{
        key: string;
        value: ExecutorValue;
    }>;
}
interface ExecutorCommand {
    expr: ExecutorValue;
    assign?: {
        target: ExecutorValue;
        steps: ExecutorStep[];
        key: string;
    };
}
export declare function cspSafeExecutor(command: ExecutorCommand): unknown;
export {};
//# sourceMappingURL=csp-safe-executor.d.ts.map