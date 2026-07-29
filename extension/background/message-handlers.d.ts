import type { MessageHandlerOwner } from './message-routing/types.js';
export interface MessageRouterDependencies {
    handlers: readonly MessageHandlerOwner[];
    debugLog: (category: string, message: string, data?: unknown) => void;
}
export declare function installMessageListener(deps: MessageRouterDependencies): void;
//# sourceMappingURL=message-handlers.d.ts.map