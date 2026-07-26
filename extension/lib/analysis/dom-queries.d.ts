/**
 * Purpose: Structured DOM querying, page info extraction, and accessibility auditing via axe-core for the inject context.
 * Docs: docs/features/feature/query-dom/index.md
 */
export interface DOMQueryParams {
    selector: string;
    include_styles?: boolean;
    properties?: string[];
    include_children?: boolean;
    max_depth?: number;
}
interface BoundingBox {
    x: number;
    y: number;
    width: number;
    height: number;
}
interface DOMElementEntry {
    tag: string;
    text: string;
    visible: boolean;
    attributes?: Record<string, string>;
    boundingBox?: BoundingBox;
    styles?: Record<string, string>;
    children?: DOMElementEntry[];
}
interface DOMQueryResult {
    url: string;
    title: string;
    matchCount: number;
    returnedCount: number;
    matches: DOMElementEntry[];
}
interface PageInfoResult {
    url: string;
    title: string;
    viewport: {
        width: number;
        height: number;
    };
    scroll: {
        x: number;
        y: number;
    };
    documentHeight: number;
    headings: string[];
    links: number;
    images: number;
    interactiveElements: number;
    forms: FormInfo[];
}
interface FormInfo {
    id?: string;
    action?: string;
    fields: string[];
}
interface AxeAuditParams {
    scope?: string;
    tags?: string[];
    include_passes?: boolean;
}
interface FormattedAxeNode {
    selector: string;
    html: string;
    failureSummary?: string;
}
interface FormattedAxeViolation {
    id: string;
    impact?: string;
    description: string;
    helpUrl: string;
    wcag?: string[];
    nodes: FormattedAxeNode[];
    nodeCount?: number;
}
interface FormattedAxeResults {
    violations: FormattedAxeViolation[];
    passes?: FormattedAxeViolation[];
    incomplete?: FormattedAxeViolation[];
    inapplicable?: FormattedAxeViolation[];
    summary: {
        violations: number;
        passes: number;
        incomplete: number;
        inapplicable: number;
    };
    partial?: boolean;
    error?: string;
}
interface AxeNode {
    target: string[] | string;
    html?: string;
    failureSummary?: string;
}
interface AxeViolation {
    id: string;
    impact?: string;
    description: string;
    helpUrl: string;
    tags?: string[];
    nodes?: AxeNode[];
}
interface AxeResults {
    violations?: AxeViolation[];
    passes?: AxeViolation[];
    incomplete?: AxeViolation[];
    inapplicable?: AxeViolation[];
}
interface AxeRunConfig {
    runOnly?: string[];
    resultTypes?: string[];
}
declare global {
    interface Window {
        axe?: {
            run(context: Element | Document | {
                include: string[];
            }, config?: AxeRunConfig): Promise<AxeResults>;
        };
    }
}
/**
 * Execute a DOM query and return structured results
 */
export declare function executeDOMQuery(params: DOMQueryParams): Promise<DOMQueryResult>;
/**
 * Get comprehensive page info
 */
export declare function getPageInfo(): Promise<PageInfoResult>;
/**
 * Run an accessibility audit using axe-core
 */
export declare function runAxeAudit(params: AxeAuditParams): Promise<FormattedAxeResults>;
/**
 * Run axe audit with a timeout.
 * Issue #276: Returns partial results on timeout or conflict instead of throwing.
 */
export declare function runAxeAuditWithTimeout(params: AxeAuditParams, timeoutMs?: number): Promise<FormattedAxeResults>;
/**
 * Format axe-core results into a compact representation
 */
export declare function formatAxeResults(axeResult: AxeResults): FormattedAxeResults;
export {};
//# sourceMappingURL=dom-queries.d.ts.map