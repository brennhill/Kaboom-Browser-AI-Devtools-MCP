/**
 * Purpose: Extract structured table data from page HTML tables for robust agent consumption.
 * Docs: docs/features/feature/analyze-tool/index.md
 */
interface DataTableParams {
    selector?: string;
    max_rows?: number;
    max_cols?: number;
}
interface DataTable {
    selector: string;
    caption?: string;
    headers: string[];
    rows: Array<Record<string, string>>;
    row_count: number;
    column_count: number;
}
export declare function extractDataTables(params?: DataTableParams): {
    tables: DataTable[];
    count: number;
};
export {};
//# sourceMappingURL=data-table.d.ts.map