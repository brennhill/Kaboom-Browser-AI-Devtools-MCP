import { classifyStorageFailure, storageFaultDetail } from './fault.js';
import { getLocal } from './local.js';
import { reportStateRecovery as reportAcrossContexts, resolveStateRecovery as resolveAcrossContexts } from './recovery.js';
import { getSession } from './session.js';
async function readValidated(read, options) {
    const report = options.report ?? reportAcrossContexts;
    const resolve = options.resolve ?? resolveAcrossContexts;
    try {
        const value = await read(options.key);
        if (value === undefined || value === null)
            return options.fallback;
        if (options.validate(value)) {
            resolve(options.diagnostic.name);
            return value;
        }
        report({
            ...options.diagnostic,
            detail: storageFaultDetail('corruption', options.diagnostic.detail)
        });
        return options.fallback;
    }
    catch (error) {
        report({
            ...options.diagnostic,
            detail: storageFaultDetail(classifyStorageFailure(error, 'read'), options.diagnostic.detail)
        });
        return options.fallback;
    }
}
export function readLocalState(options) {
    return readValidated(getLocal, options);
}
export function readSessionState(options) {
    return readValidated(getSession, options);
}
//# sourceMappingURL=validated.js.map