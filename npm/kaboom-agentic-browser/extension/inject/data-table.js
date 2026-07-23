/**
 * Purpose: Extract structured table data from page HTML tables for robust agent consumption.
 * Docs: docs/features/feature/analyze-tool/index.md
 */
const MAX_TABLES = 20;
const DEFAULT_MAX_ROWS = 100;
const DEFAULT_MAX_COLS = 30;
function normalizeText(input) {
    return (input || '').replace(/\s+/g, ' ').trim();
}
function buildTableSelector(table, index) {
    if (table.id)
        return `table#${table.id}`;
    const name = table.getAttribute('name');
    if (name)
        return `table[name="${name}"]`;
    const cls = normalizeText(table.className || '');
    if (cls)
        return `table.${cls.split(/\s+/)[0]}`;
    return `table:nth-of-type(${index + 1})`;
}
function collectHeaders(table, maxCols) {
    const headerCells = table.querySelectorAll('thead th');
    const headers = [];
    const seen = new Set();
    if (headerCells.length > 0) {
        for (let i = 0; i < headerCells.length && headers.length < maxCols; i++) {
            const label = normalizeText(headerCells[i]?.textContent || '') || `col_${headers.length + 1}`;
            if (!seen.has(label)) {
                seen.add(label);
                headers.push(label);
            }
        }
        return headers;
    }
    const firstRow = table.querySelector('tr');
    if (!firstRow)
        return headers;
    const firstRowCells = firstRow.querySelectorAll('th,td');
    for (let i = 0; i < firstRowCells.length && headers.length < maxCols; i++) {
        const cell = firstRowCells[i];
        const isHeader = cell?.tagName === 'TH';
        const label = isHeader
            ? normalizeText(cell?.textContent || '') || `col_${headers.length + 1}`
            : `col_${headers.length + 1}`;
        if (!seen.has(label)) {
            seen.add(label);
            headers.push(label);
        }
    }
    return headers;
}
function collectRows(table, headers, maxRows, maxCols) {
    const rows = table.querySelectorAll('tbody tr, tr');
    const out = [];
    let skippedHeaderLikeRow = false;
    for (let i = 0; i < rows.length && out.length < maxRows; i++) {
        const row = rows[i];
        if (!row)
            continue;
        const cells = row.querySelectorAll('th,td');
        if (cells.length === 0)
            continue;
        const allHeaders = Array.from(cells).every((cell) => cell?.tagName === 'TH');
        if (!skippedHeaderLikeRow && allHeaders) {
            skippedHeaderLikeRow = true;
            continue;
        }
        const record = {};
        let hasValue = false;
        for (let col = 0; col < cells.length && col < maxCols; col++) {
            const header = headers[col] || `col_${col + 1}`;
            const value = normalizeText(cells[col]?.textContent || '');
            record[header] = value;
            if (value)
                hasValue = true;
        }
        if (hasValue)
            out.push(record);
    }
    return out;
}
export function extractDataTables(params = {}) {
    const selector = params.selector || 'table';
    const maxRows = Math.max(1, Math.min(params.max_rows || DEFAULT_MAX_ROWS, DEFAULT_MAX_ROWS));
    const maxCols = Math.max(1, Math.min(params.max_cols || DEFAULT_MAX_COLS, DEFAULT_MAX_COLS));
    const tables = document.querySelectorAll(selector);
    const result = [];
    for (let i = 0; i < tables.length && result.length < MAX_TABLES; i++) {
        const el = tables[i];
        if (!(el instanceof HTMLTableElement))
            continue;
        const headers = collectHeaders(el, maxCols);
        const rows = collectRows(el, headers, maxRows, maxCols);
        const caption = normalizeText(el.caption?.textContent || '');
        result.push({
            selector: buildTableSelector(el, i),
            ...(caption ? { caption } : {}),
            headers,
            rows,
            row_count: rows.length,
            column_count: headers.length
        });
    }
    return { tables: result, count: result.length };
}
//# sourceMappingURL=data-table.js.map