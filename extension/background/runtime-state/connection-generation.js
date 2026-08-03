/**
 * Purpose: Owns the daemon-authoritative generation for asynchronous extension work.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */
let currentGeneration = 0;
export function getConnectionGeneration() {
    return currentGeneration;
}
export function setConnectionGeneration(generation) {
    if (!Number.isSafeInteger(generation) || generation <= 0) {
        throw new Error('Connection generation must be a positive safe integer');
    }
    currentGeneration = generation;
}
export function isConnectionGenerationCurrent(generation) {
    return generation === currentGeneration;
}
//# sourceMappingURL=connection-generation.js.map