/**
 * Purpose: Pure element-result collection, filtering, limiting, and page metadata shared by commands.
 */
export type CommandParams = Record<string, unknown>;
export declare function selectCommandElements(elements: unknown[], params: CommandParams): unknown[];
interface CommandElementResult {
    result?: unknown;
}
export declare function collectCommandElements(results: CommandElementResult[], limit: number): {
    elements: unknown[];
    firstError?: string;
};
interface CommandTabMetadata {
    url?: string;
    title?: string;
    status?: string;
    favIconUrl?: string;
    width?: number;
    height?: number;
}
export declare function commandPageMetadata(tab: CommandTabMetadata): Record<string, unknown>;
export {};
//# sourceMappingURL=element-results.d.ts.map