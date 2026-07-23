/**
 * Purpose: Defines shared utility and helper TypeScript types used across extension modules.
 * Why: Keeps common type patterns centralized to reduce duplication and improve type consistency.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
/**
 * @fileoverview Utility Types for Kaboom Extension
 *
 * Generic utility types, type guards, and helper types used across the extension.
 */
/**
 * Make all properties in T deeply readonly
 */
export type DeepReadonly<T> = T extends (infer R)[] ? DeepReadonlyArray<R> : T extends object ? DeepReadonlyObject<T> : T;
interface DeepReadonlyArray<T> extends ReadonlyArray<DeepReadonly<T>> {
}
type DeepReadonlyObject<T> = {
    readonly [P in keyof T]: DeepReadonly<T[P]>;
};
/**
 * Make specific keys of T optional
 */
export type PartialBy<T, K extends keyof T> = Omit<T, K> & Partial<Pick<T, K>>;
/**
 * Make specific keys of T required
 */
export type RequiredBy<T, K extends keyof T> = Omit<T, K> & Required<Pick<T, K>>;
/**
 * Extract the type of array elements
 */
export type ArrayElement<T> = T extends readonly (infer E)[] ? E : never;
/**
 * JSON-serializable value types
 */
export type JsonPrimitive = string | number | boolean | null;
export type JsonArray = JsonValue[];
export type JsonObject = {
    [key: string]: JsonValue;
};
export type JsonValue = JsonPrimitive | JsonArray | JsonObject;
/**
 * Serializable value (JSON-compatible)
 */
export type Serializable = JsonValue;
/**
 * Non-nullable version of T
 */
export type NonNullableFields<T> = {
    [P in keyof T]: NonNullable<T[P]>;
};
/**
 * Extract keys of T whose values are of type V
 */
export type KeysOfType<T, V> = {
    [K in keyof T]: T[K] extends V ? K : never;
}[keyof T];
/**
 * Omit keys whose values are of type V
 */
export type OmitByType<T, V> = Omit<T, KeysOfType<T, V>>;
/**
 * Pick keys whose values are of type V
 */
export type PickByType<T, V> = Pick<T, KeysOfType<T, V>>;
/**
 * Async function type
 */
export type AsyncFunction<TArgs extends unknown[] = unknown[], TReturn = unknown> = (...args: TArgs) => Promise<TReturn>;
/**
 * Callback function type
 */
export type Callback<T = void> = (result: T) => void;
/**
 * Error callback type
 */
export type ErrorCallback = (error: Error) => void;
/**
 * Event handler type
 */
export type EventHandler<T = Event> = (event: T) => void;
/**
 * Debounced function type
 */
export interface DebouncedFunction<T extends (...args: unknown[]) => unknown> {
    (...args: Parameters<T>): void;
    cancel: () => void;
    flush: () => void;
}
/**
 * Result type for operations that can fail
 */
export type Result<T, E = Error> = {
    readonly success: true;
    readonly value: T;
} | {
    readonly success: false;
    readonly error: E;
};
/**
 * Async result type
 */
export type AsyncResult<T, E = Error> = Promise<Result<T, E>>;
/**
 * Operation outcome with optional message
 */
export interface OperationResult {
    readonly success: boolean;
    readonly error?: string;
    readonly message?: string;
}
/**
 * Brand a type with a unique identifier to prevent accidental mixing
 */
declare const brand: unique symbol;
export type Brand<T, B extends string> = T & {
    readonly [brand]: B;
};
/**
 * Branded string types for type safety
 */
export type TabId = Brand<number, 'TabId'>;
export type QueryId = Brand<string, 'QueryId'>;
export type SessionId = Brand<string, 'SessionId'>;
export type CorrelationId = Brand<string, 'CorrelationId'>;
export type Timestamp = Brand<string, 'ISO8601Timestamp'>;
/**
 * Validated string with length constraints
 */
export interface ValidatedString<MinLength extends number = 0, MaxLength extends number = number> {
    readonly value: string;
    readonly length: number;
    readonly minLength: MinLength;
    readonly maxLength: MaxLength;
}
/**
 * URL validation result
 */
export interface ValidatedUrl {
    readonly href: string;
    readonly protocol: string;
    readonly hostname: string;
    readonly port: string;
    readonly pathname: string;
    readonly origin: string;
}
/**
 * Extract a specific variant from a discriminated union by its type field
 */
export type ExtractByType<TUnion, TType> = TUnion extends {
    type: TType;
} ? TUnion : never;
/**
 * Get all type values from a discriminated union
 */
export type TypesOf<TUnion extends {
    type: string;
}> = TUnion['type'];
/**
 * Create a handler map for discriminated unions
 */
export type MessageHandlerMap<TUnion extends {
    type: string;
}> = {
    [TType in TypesOf<TUnion>]: (message: ExtractByType<TUnion, TType>) => void | Promise<void>;
};
/**
 * Serialized element info (safe for messaging)
 */
export interface SerializedElementInfo {
    readonly tagName: string;
    readonly id?: string;
    readonly className?: string;
    readonly textContent?: string;
    readonly innerHTML?: string;
    readonly attributes?: Readonly<Record<string, string>>;
    readonly boundingClientRect?: {
        readonly x: number;
        readonly y: number;
        readonly width: number;
        readonly height: number;
        readonly top: number;
        readonly right: number;
        readonly bottom: number;
        readonly left: number;
    };
}
/**
 * Element selector with multiple strategies
 */
export interface ElementSelector {
    readonly css?: string;
    readonly xpath?: string;
    readonly testId?: string;
    readonly aria?: string;
    readonly text?: string;
}
/**
 * Extension settings that can be persisted
 */
export interface ExtensionSettings {
    readonly serverUrl: string;
    readonly logLevel: string;
    readonly screenshotOnError: boolean;
    readonly sourceMapEnabled: boolean;
    readonly debugMode: boolean;
    readonly aiWebPilotEnabled: boolean;
    readonly webSocketCaptureEnabled: boolean;
    readonly networkWaterfallEnabled: boolean;
    readonly performanceMarksEnabled: boolean;
    readonly actionReplayEnabled: boolean;
    readonly networkBodyCaptureEnabled: boolean;
    readonly performanceSnapshotEnabled: boolean;
}
/**
 * Partial settings for updates
 */
export type PartialSettings = Partial<ExtensionSettings>;
/**
 * Rate limit configuration
 */
export interface RateLimitConfig {
    readonly maxFailures: number;
    readonly resetTimeout: number;
    readonly backoffSchedule: readonly number[];
    readonly retryBudget: number;
}
/**
 * Batcher configuration
 */
export interface BatcherConfig {
    readonly debounceMs: number;
    readonly maxBatchSize: number;
    readonly retryBudget?: number;
    readonly maxFailures?: number;
    readonly resetTimeout?: number;
}
/**
 * Check if a value is a non-null object
 */
export declare function isObject(value: unknown): value is Record<string, unknown>;
/**
 * Check if a value is a non-empty string
 */
export declare function isNonEmptyString(value: unknown): value is string;
/**
 * Check if a value has a specific type property
 */
export declare function hasType<T extends string>(value: unknown, type: T): value is {
    type: T;
} & Record<string, unknown>;
/**
 * Check if value is a valid JSON value
 */
export declare function isJsonValue(value: unknown): value is JsonValue;
/**
 * Check if a message has a specific type (type guard factory)
 */
export declare function createTypeGuard<T extends {
    type: string;
}>(type: T['type']): (value: unknown) => value is T;
/**
 * Timer ID types for cleanup tracking
 */
export type TimeoutId = ReturnType<typeof setTimeout>;
export type IntervalId = ReturnType<typeof setInterval>;
/**
 * Timer cleanup function
 */
export type TimerCleanup = () => void;
/**
 * Buffer state for memory tracking
 */
export interface BufferState {
    readonly logEntries: readonly unknown[];
    readonly wsEvents: readonly unknown[];
    readonly networkBodies: readonly unknown[];
    readonly enhancedActions: readonly unknown[];
}
/**
 * Memory estimation result
 */
export interface MemoryEstimate {
    readonly totalBytes: number;
    readonly breakdown: {
        readonly logEntries: number;
        readonly wsEvents: number;
        readonly networkBodies: number;
        readonly enhancedActions: number;
    };
}
export {};
//# sourceMappingURL=utils.d.ts.map