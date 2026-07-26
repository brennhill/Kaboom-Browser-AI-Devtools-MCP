/**
 * Purpose: Defines extension-side accessibility audit result contracts and violation node shapes.
 * Why: Keeps a11y analysis payloads stable between inject execution and background/query consumers.
 * Docs: docs/features/feature/enhanced-wcag-audit/index.md
 */
/**
 * @fileoverview Accessibility Types
 * Accessibility audit results and violations
 */
/**
 * Accessibility violation node
 */
export interface A11yViolationNode {
    readonly html: string;
    readonly target: readonly string[];
    readonly failureSummary: string;
}
/**
 * Accessibility violation
 */
export interface A11yViolation {
    readonly id: string;
    readonly impact: string;
    readonly description: string;
    readonly help: string;
    readonly helpUrl: string;
    readonly nodes: readonly A11yViolationNode[];
}
/**
 * Accessibility audit result
 */
export interface A11yAuditResult {
    readonly violations: readonly A11yViolation[];
    readonly passes: readonly {
        readonly id: string;
        readonly description: string;
        readonly nodes: readonly {
            html: string;
            target: string[];
        }[];
    }[];
    readonly incomplete: readonly {
        readonly id: string;
        readonly description: string;
        readonly nodes: readonly {
            html: string;
            target: string[];
        }[];
    }[];
    readonly inapplicable: readonly {
        id: string;
        description: string;
    }[];
    readonly summary?: {
        readonly violations: number;
        readonly passes: number;
        readonly incomplete?: number;
        readonly inapplicable?: number;
        readonly violation_count?: number;
        readonly pass_count?: number;
        readonly incomplete_count?: number;
        readonly inapplicable_count?: number;
    };
    readonly error?: string;
}
//# sourceMappingURL=accessibility.d.ts.map