import type { MessageHandlerOwner } from './types.js';
export interface UtilityHandlerDependencies {
    getServerUrl: () => string;
}
export declare function createUtilityMessageHandler(deps: UtilityHandlerDependencies): MessageHandlerOwner;
//# sourceMappingURL=utility-handler.d.ts.map