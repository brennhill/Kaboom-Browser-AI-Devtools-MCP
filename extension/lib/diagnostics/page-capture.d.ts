/**
 * Purpose: Surface redacted injected-world capture failures to local extension diagnostics.
 * Docs: docs/features/feature/system-doctor/index.md
 */
export interface PageCaptureDiagnostic {
    category: string;
    message: string;
    error_type: string;
}
export declare function reportPageCaptureFailure(category: string, error: unknown): void;
//# sourceMappingURL=page-capture.d.ts.map