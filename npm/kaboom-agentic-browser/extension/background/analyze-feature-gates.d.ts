/**
 * Purpose: Detects plan-gated, auth-gated, and usage-limited features on a page.
 * Why: Automates competitive analysis by identifying free vs. paid features.
 * Docs: docs/features/feature/analyze-tool/index.md
 */
export declare function analyzeFeatureGates(): {
    plan_gates: Array<{
        feature: string;
        required_plan?: string;
        selector: string;
        text: string;
    }>;
    auth_gates: Array<{
        feature: string;
        provider?: string;
        selector: string;
        text: string;
    }>;
    usage_limits: Array<{
        feature: string;
        text: string;
        selector: string;
    }>;
    total_gates: number;
};
//# sourceMappingURL=analyze-feature-gates.d.ts.map