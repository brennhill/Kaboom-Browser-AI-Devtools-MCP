import type { MessageHandlerOwner } from './types.js';
export interface SettingsHandlerDependencies {
    getServerUrl: () => string;
    setServerUrl: (url: string) => void;
    setLogLevel: (level: string) => void;
    setScreenshotOnError: (enabled: boolean) => void;
    setSourceMapEnabled: (enabled: boolean) => void;
    setDebugMode: (enabled: boolean) => void;
    clearSourceMapCache: () => void;
    saveSetting: (key: string, value: unknown) => void;
    forwardToContentScripts: (message: {
        type: string;
        [key: string]: unknown;
    }) => void;
    checkConnection: () => Promise<void>;
    debugLog: (category: string, message: string, data?: unknown) => void;
}
export declare function createSettingsMessageHandler(deps: SettingsHandlerDependencies): MessageHandlerOwner;
//# sourceMappingURL=settings-handler.d.ts.map