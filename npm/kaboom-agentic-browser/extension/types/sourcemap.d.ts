/**
 * Purpose: Defines source-map parsing and original-location type contracts used in stack/frame resolution.
 * Why: Keeps source-map resolution payloads explicit for error enrichment and code-location attribution.
 * Docs: docs/features/feature/code-navigation-modification/index.md
 */
/**
 * @fileoverview Source Map Types
 * Source map parsing and original location resolution
 */
/**
 * Parsed source map
 */
export interface ParsedSourceMap {
    readonly sources: readonly string[];
    readonly names: readonly string[];
    readonly sourceRoot: string;
    readonly mappings: readonly (readonly (readonly number[])[])[];
    readonly sourcesContent: readonly string[];
}
/**
 * Original location from source map
 */
export interface OriginalLocation {
    readonly source: string;
    readonly line: number;
    readonly column: number;
    readonly name: string | null;
}
//# sourceMappingURL=sourcemap.d.ts.map