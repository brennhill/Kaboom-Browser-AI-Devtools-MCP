/**
 * CSP-Safe Expression Parser
 *
 * WHY THIS EXISTS:
 * Page Content Security Policy (CSP) blocks eval() and new Function() — the two
 * mechanisms normally used by execute_js. But chrome.scripting.executeScript can
 * inject a PRE-COMPILED function reference into the page's MAIN world. Chrome's
 * native injection mechanism bypasses CSP because no string-to-code conversion
 * happens — the function was compiled at extension build time.
 *
 * HOW IT WORKS:
 * 1. This parser converts a JS expression string into a structured command
 *    (property paths, method calls, literal arguments — all JSON-serializable data).
 * 2. The structured command is passed as an ARGUMENT to a pre-compiled executor
 *    function via chrome.scripting.executeScript({func: executor, args: [command]}).
 * 3. The executor interprets the command using direct property access (obj[key])
 *    and Function.prototype.apply() — operations CSP does NOT restrict.
 *
 * CSP blocks CODE-FROM-STRINGS (eval, new Function, inline <script>).
 * CSP does NOT block PROPERTY ACCESS (obj.key), METHOD CALLS (obj.method()),
 * or the chrome.scripting.executeScript({func}) injection path.
 *
 * LIMITATIONS:
 * Only expression-level JS is supported (property chains, method calls, literals).
 * Control flow, closures, operators, and variable declarations cannot be represented
 * as structured commands and are rejected with guidance to use DOM primitives.
 */
import type { ParseResult } from './csp-safe-types.js';
export declare function parseExpression(input: string): ParseResult;
//# sourceMappingURL=csp-safe-parser.d.ts.map