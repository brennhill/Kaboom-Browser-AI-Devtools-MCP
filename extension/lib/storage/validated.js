import { getLocal } from './local.js';
import { reportStateRecovery as reportAcrossContexts } from './recovery.js';
import { getSession } from './session.js';
async function readValidated(read, options) {
    const report = options.report ?? reportAcrossContexts;
    try {
        const value = await read(options.key);
        if (value === undefined || value === null)
            return options.fallback;
        if (options.validate(value))
            return value;
        report(options.diagnostic);
        return options.fallback;
    }
    catch {
        report(options.diagnostic);
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