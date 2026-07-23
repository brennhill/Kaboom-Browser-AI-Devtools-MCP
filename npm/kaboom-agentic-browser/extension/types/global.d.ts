/**
 * Global type declarations for Kaboom extension
 */

// Kaboom Developer API exposed on window
interface KaboomAPI {
  version: string
  annotate(key: string, value: unknown): void
  removeAnnotation(key: string): void
  clearAnnotations(): void
  getContext(): Record<string, unknown>
  getActions(): Array<{
    type: string
    target: string
    timestamp: string
    value?: string
  }>
  clearActions(): void
  setActionCapture(enabled: boolean): void
  recordAction(type: string, element: Element, opts?: Record<string, unknown>): void
  getEnhancedActions(): Array<Record<string, unknown>>
  clearEnhancedActions(): void
  generateScript(actions: Array<Record<string, unknown>>, opts?: { baseUrl?: string }): string
  getSelectors(element: Element): {
    testId?: string
    aria?: string
    role?: string
    cssPath?: string
    xpath?: string
  }
  setNetworkWaterfall(enabled: boolean): void
  getNetworkWaterfall(opts?: { since?: number }): Array<Record<string, unknown>>
  setPerformanceMarks(enabled: boolean): void
  getMarks(opts?: { since?: number }): Array<Record<string, unknown>>
  getMeasures(): Array<Record<string, unknown>>
  setAiContext(enabled: boolean): void
  setStateSnapshot(enabled: boolean): void
  enrichError(error: Error): Record<string, unknown>
}

interface Window {
  __gasoline?: KaboomAPI
}

// axe-core accessibility testing library (loaded dynamically)
declare namespace axe {
  interface RunOptions {
    runOnly?: {
      type: string
      values: string[]
    }
    rules?: Record<string, { enabled: boolean }>
  }

  interface Result {
    violations: Array<{
      id: string
      impact: string
      description: string
      help: string
      helpUrl: string
      nodes: Array<{
        html: string
        target: string[]
        failureSummary: string
      }>
    }>
    passes: Array<{
      id: string
      description: string
      nodes: Array<{ html: string; target: string[] }>
    }>
    incomplete: Array<{
      id: string
      description: string
      nodes: Array<{ html: string; target: string[] }>
    }>
    inapplicable: Array<{
      id: string
      description: string
    }>
  }

  function run(context?: Element | Document, options?: RunOptions): Promise<Result>
}

// PerformanceObserver entry types used in Web Vitals
interface PerformanceLayoutShift extends PerformanceEntry {
  hadRecentInput: boolean
  value: number
}

interface PerformanceLargestContentfulPaint extends PerformanceEntry {
  element?: Element
  size?: number
}

// Chrome extension messaging types
interface ChromeMessage {
  type: string
  payload?: unknown
  url?: string
  level?: string
  queryId?: string
  params?: Record<string, unknown>
}
