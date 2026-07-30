/**
 * Purpose: Validate persisted extension state and apply explicit safe fallbacks.
 */
import type { StateRecoveryDiagnostic } from '../../types/runtime-messages.js';
type Reporter = (diagnostic: StateRecoveryDiagnostic) => void;
type Validator<T> = (value: unknown) => value is T;
interface ReadStateOptions<T> {
    key: string;
    fallback: T;
    validate: Validator<T>;
    diagnostic: StateRecoveryDiagnostic;
    report?: Reporter;
}
export declare function readLocalState<T>(options: ReadStateOptions<T>): Promise<T>;
export declare function readSessionState<T>(options: ReadStateOptions<T>): Promise<T>;
export {};
//# sourceMappingURL=validated.d.ts.map