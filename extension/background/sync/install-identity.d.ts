/**
 * Purpose: Canonically owns the daemon install identity and its durable storage.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
export declare function getServerInstallId(): string | undefined;
export declare function loadServerInstallId(): Promise<void>;
export declare function updateServerInstallId(id: string): void;
//# sourceMappingURL=install-identity.d.ts.map