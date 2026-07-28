/**
 * @fileoverview Global Type Declarations for Kaboom Extension
 *
 * Ambient type declarations for global objects, browser APIs, and third-party libraries.
 * These types augment the global namespace without requiring explicit imports.
 */

import type {
  ActionEntry,
  EnhancedAction,
  SelectorStrategies
} from './capture/actions.js'
import type { AiContextData } from './capture/ai-context.js'
import type { WaterfallEntry } from './capture/network.js'
import type {
  PerformanceMark,
  PerformanceMeasure
} from './capture/performance.js'

// =============================================================================
// KABOOM DEVELOPER API (window.__kaboom)
// =============================================================================

/**
 * Kaboom Developer API exposed on window for programmatic control
 */
interface KaboomAPI {
  /** API version string */
  readonly version: string

  // --- Context Annotations ---

  /**
   * Add a context annotation that will be included with errors
   * @param key Annotation key (e.g., 'checkout-flow', 'user')
   * @param value Annotation value
   */
  annotate(key: string, value: unknown): void

  /**
   * Remove a context annotation
   * @param key Annotation key to remove
   */
  removeAnnotation(key: string): void

  /** Clear all context annotations */
  clearAnnotations(): void

  /**
   * Get current context annotations
   * @returns Current annotations or null if none
   */
  getContext(): Record<string, unknown> | null

  // --- User Action Replay ---

  /**
   * Get the user action replay buffer
   * @returns Recent user actions
   */
  getActions(): readonly ActionEntry[]

  /** Clear the user action replay buffer */
  clearActions(): void

  /**
   * Enable or disable action capture
   * @param enabled Whether to capture user actions
   */
  setActionCapture(enabled: boolean): void

  // --- Enhanced Actions / Reproduction Scripts ---

  /**
   * Record an enhanced action (for testing)
   * @param type Action type
   * @param element Target element
   * @param opts Options
   */
  recordAction(type: string, element: Element, opts?: Record<string, unknown>): void

  /**
   * Get the enhanced action buffer
   */
  getEnhancedActions(): readonly EnhancedAction[]

  /** Clear the enhanced action buffer */
  clearEnhancedActions(): void

  /**
   * Generate a Playwright reproduction script
   * @param actions Actions to convert
   * @param opts Generation options
   * @returns Playwright test script
   */
  generateScript(actions?: readonly EnhancedAction[], opts?: { baseUrl?: string }): string

  /**
   * Compute multi-strategy selectors for an element
   * @param element Target element
   */
  getSelectors(element: Element): SelectorStrategies

  // --- Network Waterfall ---

  /**
   * Enable or disable network waterfall capture
   */
  setNetworkWaterfall(enabled: boolean): void

  /**
   * Get current network waterfall
   * @param opts Filter options
   */
  getNetworkWaterfall(opts?: { since?: number }): readonly WaterfallEntry[]

  // --- Performance Marks ---

  /**
   * Enable or disable performance marks capture
   */
  setPerformanceMarks(enabled: boolean): void

  /**
   * Get performance marks
   * @param opts Filter options
   */
  getMarks(opts?: { since?: number }): readonly PerformanceMark[]

  /**
   * Get performance measures
   * @param opts Filter options
   */
  getMeasures(opts?: { since?: number }): readonly PerformanceMeasure[]

  // --- AI Context ---

  /**
   * Enable or disable AI context enrichment
   */
  setAiContext(enabled: boolean): void

  /**
   * Enable or disable state snapshot in AI context
   */
  setStateSnapshot(enabled: boolean): void

  /**
   * Enrich an error entry with AI context
   * @param error Error entry to enrich
   * @returns Enriched error entry
   */
  enrichError(error: Error): Promise<AiContextData | null>
}

// =============================================================================
// WINDOW AUGMENTATION
// =============================================================================

declare global {
  /** Early-patch buffered WebSocket connection */
  interface EarlyWsConnection {
    ws: WebSocket
    url: string
    createdAt: number
    events: Array<{ type: string; ts: number; code?: number; reason?: string }>
  }

  /** Early-patch buffered network body (fetch/XHR response captured before Phase 2) */
  interface EarlyNetworkBody {
    url: string
    method: string
    status: number
    content_type: string
    response_body: string
    timestamp: number
  }

  interface Window {
    /** Kaboom developer API */
    __kaboom?: KaboomAPI

    /** WebSocket class (for monkey-patching) */
    WebSocket: typeof WebSocket

    /** Fetch function (for monkey-patching) */
    fetch: typeof fetch

    /** Early-patch: original WebSocket constructor saved before page scripts run */
    __KABOOM_ORIGINAL_WS__?: typeof WebSocket

    /** Early-patch: buffered WebSocket connections created before inject script loaded */
    __KABOOM_EARLY_WS__?: EarlyWsConnection[]

    /** Early-patch: original fetch function saved before page scripts run */
    __KABOOM_ORIGINAL_FETCH__?: typeof fetch

    /** Early-patch: original XMLHttpRequest.prototype.open saved before page scripts run */
    __KABOOM_ORIGINAL_XHR_OPEN__?: typeof XMLHttpRequest.prototype.open

    /** Early-patch: original XMLHttpRequest.prototype.send saved before page scripts run */
    __KABOOM_ORIGINAL_XHR_SEND__?: typeof XMLHttpRequest.prototype.send

    /** Early-patch: buffered network bodies captured before inject script loaded */
    __KABOOM_EARLY_BODIES__?: EarlyNetworkBody[]
  }
}

// =============================================================================
// AXE-CORE ACCESSIBILITY TESTING LIBRARY
// =============================================================================

/**
 * axe-core accessibility testing library (loaded dynamically)
 */
declare namespace axe {
  interface RunOptions {
    runOnly?: {
      type: 'rule' | 'tag'
      values: string[]
    }
    rules?: Record<string, { enabled: boolean }>
    reporter?: 'v1' | 'v2' | 'raw'
    resultTypes?: Array<'violations' | 'passes' | 'incomplete' | 'inapplicable'>
    selectors?: boolean
    ancestry?: boolean
    xpath?: boolean
  }

  interface NodeResult {
    html: string
    target: string[]
    failureSummary?: string
    any?: CheckResult[]
    all?: CheckResult[]
    none?: CheckResult[]
  }

  interface CheckResult {
    id: string
    data: unknown
    relatedNodes?: NodeResult[]
    impact?: string
    message: string
  }

  interface RuleResult {
    id: string
    impact?: 'minor' | 'moderate' | 'serious' | 'critical'
    description: string
    help: string
    helpUrl: string
    tags: string[]
    nodes: NodeResult[]
  }

  interface Result {
    violations: RuleResult[]
    passes: RuleResult[]
    incomplete: RuleResult[]
    inapplicable: Array<{
      id: string
      description: string
      help: string
      helpUrl: string
      tags: string[]
    }>
    timestamp: string
    url: string
    testEngine: {
      name: string
      version: string
    }
    testRunner: {
      name: string
    }
    testEnvironment: {
      userAgent: string
      windowWidth: number
      windowHeight: number
    }
    toolOptions: RunOptions
  }

  /**
   * Run accessibility audit
   * @param context Element or document to audit
   * @param options Audit options
   */
  function run(context?: Element | Document, options?: RunOptions): Promise<Result>

  /**
   * Configure axe-core
   * @param options Configuration options
   */
  function configure(options: {
    rules?: Array<{
      id: string
      enabled?: boolean
      selector?: string
      tags?: string[]
    }>
    branding?: {
      brand?: string
      application?: string
    }
    reporter?: string
    locale?: Record<string, unknown>
  }): void

  /**
   * Reset axe-core configuration
   */
  function reset(): void
}

// =============================================================================
// PERFORMANCE OBSERVER ENTRY TYPES
// =============================================================================

/**
 * Layout Shift entry (for CLS measurement)
 */
interface PerformanceLayoutShift extends PerformanceEntry {
  readonly hadRecentInput: boolean
  readonly value: number
  readonly lastInputTime: number
  readonly sources?: ReadonlyArray<{
    readonly node?: Node
    readonly previousRect: DOMRectReadOnly
    readonly currentRect: DOMRectReadOnly
  }>
}

/**
 * Largest Contentful Paint entry (for LCP measurement)
 */
interface PerformanceLargestContentfulPaint extends PerformanceEntry {
  readonly element?: Element
  readonly renderTime: number
  readonly loadTime: number
  readonly size: number
  readonly id: string
  readonly url: string
}

/**
 * First Input entry (for FID measurement)
 */
interface PerformanceEventTiming extends PerformanceEntry {
  readonly processingStart: number
  readonly processingEnd: number
  readonly interactionId?: number
  readonly cancelable: boolean
  readonly target?: Element
}

/**
 * Long Task entry
 */
interface PerformanceLongTaskTiming extends PerformanceEntry {
  readonly attribution: ReadonlyArray<{
    readonly containerType: string
    readonly containerSrc: string
    readonly containerId: string
    readonly containerName: string
  }>
}

/**
 * Paint entry (for FCP measurement)
 */
interface PerformancePaintTiming extends PerformanceEntry {
  readonly name: 'first-paint' | 'first-contentful-paint'
}

// =============================================================================
// PERFORMANCE MEMORY API (Chrome-specific)
// =============================================================================

interface Performance {
  /** Chrome-specific memory info */
  readonly memory?: {
    readonly usedJSHeapSize: number
    readonly totalJSHeapSize: number
    readonly jsHeapSizeLimit: number
  }
}

// =============================================================================
// CHROME EXTENSION TYPES (supplementary to @types/chrome)
// =============================================================================

// Note: @types/chrome provides most Chrome API types.
// These are supplementary types for patterns not fully covered.

/**
 * Chrome storage change info
 */
interface ChromeStorageChanges {
  [key: string]: {
    oldValue?: unknown
    newValue?: unknown
  }
}

// Export empty object to make this a module
export {}
