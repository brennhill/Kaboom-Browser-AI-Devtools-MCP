/**
 * Purpose: Queries elements by CSS selector and returns computed CSS properties, box model dimensions, custom properties, and contrast ratios for the analyze tool.
 * Docs: docs/features/feature/analyze-tool/index.md
 *
 * CONTRACT: this file reports what the page renders and judges none of it. It
 * never decides which value is a token, what the norm is, or what counts as
 * drift — that arithmetic lives in Go (cmd/browser-agent/internal/toolanalyze/
 * designdrift) where it is table-tested. Content scripts are bundled and
 * awkward to test, so keep this a measurement.
 */
import type { WireStyleProbeResult } from '../types/wire/wire-style-probe.js';
interface ComputedStylesParams {
    selector: string;
    properties?: string[];
    /** Element cap for this query; clamped to MAX_ELEMENTS_CEILING. */
    max_elements?: number;
    /** Collect CSS custom properties (:root table and per-element scope). */
    include_custom_properties?: boolean;
}
/**
 * Query computed styles for all elements matching a CSS selector.
 */
export declare function queryComputedStyles(params: ComputedStylesParams): WireStyleProbeResult;
export {};
//# sourceMappingURL=computed-styles.d.ts.map