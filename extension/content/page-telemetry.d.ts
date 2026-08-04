/**
 * Purpose: Validates and bounds authenticated page telemetry before extension forwarding.
 * Why: The tracked page is an untrusted producer even after the injected channel is authenticated.
 */
export type PageTelemetryRejection = 'invalid_schema' | 'payload_too_large' | 'payload_too_deep';
export declare function validatePageTelemetry(messageType: string, payload: unknown): PageTelemetryRejection | null;
//# sourceMappingURL=page-telemetry.d.ts.map