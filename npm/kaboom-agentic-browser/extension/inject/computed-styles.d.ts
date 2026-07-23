/**
 * Purpose: Queries elements by CSS selector and returns computed CSS properties, box model dimensions, and contrast ratios for the analyze tool.
 * Docs: docs/features/feature/analyze-tool/index.md
 */
interface ComputedStylesParams {
    selector: string;
    properties?: string[];
}
interface ElementStyleResult {
    selector: string;
    tag: string;
    computed_styles: Record<string, string>;
    box_model: {
        x: number;
        y: number;
        width: number;
        height: number;
        top: number;
        right: number;
        bottom: number;
        left: number;
    };
    contrast_ratio?: number;
}
/**
 * Query computed styles for all elements matching a CSS selector.
 */
export declare function queryComputedStyles(params: ComputedStylesParams): ElementStyleResult[];
export {};
//# sourceMappingURL=computed-styles.d.ts.map