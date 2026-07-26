/**
 * Purpose: Exposes the window.__kaboom developer API for programmatic access to capture capabilities, context annotations, and event buffers.
 * Docs: docs/features/feature/custom-event-api/index.md
 */

/**
 * @fileoverview Kaboom API - Exposes window.__kaboom interface for developers
 * to interact with Kaboom capture capabilities.
 */

declare const __KABOOM_VERSION__: string

import type {
  LogEntry,
  ActionEntry,
  EnhancedAction,
  SelectorStrategies,
  WaterfallEntry,
  PerformanceMark,
  PerformanceMeasure
} from '../types/index.js'

import {
  setContextAnnotation,
  removeContextAnnotation,
  clearContextAnnotations,
  getContextAnnotations
} from '../lib/page/context.js'
import {
  computeSelectors,
  recordEnhancedAction,
  getEnhancedActionBuffer,
  clearEnhancedActionBuffer,
  generatePlaywrightScript
} from '../lib/page/reproduction.js'
import { getActionBuffer, clearActionBuffer, setActionCaptureEnabled } from '../lib/page/actions.js'
import { getNetworkWaterfall, setNetworkWaterfallEnabled } from '../lib/network.js'
import { getPerformanceMarks, getPerformanceMeasures, setPerformanceMarksEnabled } from '../lib/performance.js'
import { enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot } from '../lib/ai-context/ai-context.js'

/**
 * KaboomAPI interface exposed on window.__kaboom
 */
export interface KaboomAPI {
  annotate(key: string, value: unknown): void
  removeAnnotation(key: string): void
  clearAnnotations(): void
  getContext(): Record<string, unknown> | null
  getActions(): ActionEntry[]
  clearActions(): void
  setActionCapture(enabled: boolean): void
  setNetworkWaterfall(enabled: boolean): void
  getNetworkWaterfall(options?: { since?: number; initiatorTypes?: string[] }): WaterfallEntry[]
  setPerformanceMarks(enabled: boolean): void
  getMarks(options?: { since?: number }): PerformanceMark[]
  getMeasures(options?: { since?: number }): PerformanceMeasure[]
  enrichError(error: LogEntry): Promise<LogEntry>
  setAiContext(enabled: boolean): void
  setStateSnapshot(enabled: boolean): void
  recordAction(type: string, element: Element, opts?: Record<string, unknown>): void
  getEnhancedActions(): EnhancedAction[]
  clearEnhancedActions(): void
  generateScript(opts?: Record<string, unknown>): string
  getSelectors(element: Element): SelectorStrategies
  setInputValue(selector: string, value: string | boolean): boolean
  version: string
}

// Extend Window interface for __kaboom
declare global {
  interface Window {
    __kaboom?: KaboomAPI
  }
}

function setWithNativeSetter<T extends HTMLElement>(
  element: T,
  proto: { prototype: T },
  prop: string,
  val: string | boolean
): void {
  const setter = Object.getOwnPropertyDescriptor(proto.prototype, prop)?.set
  if (setter) setter.call(element, val)
  else (element as unknown as Record<string, string | boolean>)[prop] = val
}

/** Use native property setter to set value on form elements, bypassing framework interception */
function setNativeValue(element: Element, value: string | boolean): boolean {
  if (element instanceof HTMLInputElement) {
    if (element.type === 'checkbox' || element.type === 'radio') {
      setWithNativeSetter(element, HTMLInputElement, 'checked', Boolean(value))
    } else {
      setWithNativeSetter(element, HTMLInputElement, 'value', String(value))
    }
    return true
  }
  if (element instanceof HTMLTextAreaElement) {
    setWithNativeSetter(element, HTMLTextAreaElement, 'value', String(value))
    return true
  }
  if (element instanceof HTMLSelectElement) {
    setWithNativeSetter(element, HTMLSelectElement, 'value', String(value))
    return true
  }
  return false
}

/**
 * Install the window.__kaboom API for developers to interact with Kaboom
 */
// #lizard forgives
export function installKaboomAPI(): void {
  if (typeof window === 'undefined') return

  window.__kaboom = {
    /**
     * Add a context annotation that will be included with errors
     * @param key - Annotation key (e.g., 'checkout-flow', 'user')
     * @param value - Annotation value
     * @example
     * window.__kaboom.annotate('checkout-flow', { step: 'payment', items: 3 })
     */
    annotate(key: string, value: unknown): boolean {
      return setContextAnnotation(key, value)
    },

    /**
     * Remove a context annotation
     * @param key - Annotation key to remove
     */
    removeAnnotation(key: string): boolean {
      return removeContextAnnotation(key)
    },

    /**
     * Clear all context annotations
     */
    clearAnnotations(): void {
      clearContextAnnotations()
    },

    /**
     * Get current context annotations
     * @returns Current annotations or null if none
     */
    getContext(): Record<string, unknown> | null {
      return getContextAnnotations()
    },

    /**
     * Get the user action replay buffer
     * @returns Recent user actions
     */
    getActions(): ActionEntry[] {
      return getActionBuffer() as unknown as ActionEntry[]
    },

    /**
     * Clear the user action replay buffer
     */
    clearActions(): void {
      clearActionBuffer()
    },

    /**
     * Enable or disable action capture
     * @param enabled - Whether to capture user actions
     */
    setActionCapture(enabled: boolean): void {
      setActionCaptureEnabled(enabled)
    },

    /**
     * Enable or disable network waterfall capture
     * @param enabled - Whether to capture network waterfall
     */
    setNetworkWaterfall(enabled: boolean): void {
      setNetworkWaterfallEnabled(enabled)
    },

    /**
     * Get current network waterfall
     * @param options - Filter options
     * @returns Network waterfall entries
     */
    getNetworkWaterfall(options?: { since?: number; initiatorTypes?: string[] }): WaterfallEntry[] {
      return getNetworkWaterfall(options)
    },

    /**
     * Enable or disable performance marks capture
     * @param enabled - Whether to capture performance marks
     */
    setPerformanceMarks(enabled: boolean): void {
      setPerformanceMarksEnabled(enabled)
    },

    /**
     * Get performance marks
     * @param options - Filter options
     * @returns Performance mark entries
     */
    getMarks(options?: { since?: number }): PerformanceMark[] {
      return getPerformanceMarks(options) as unknown as PerformanceMark[]
    },

    /**
     * Get performance measures
     * @param options - Filter options
     * @returns Performance measure entries
     */
    getMeasures(options?: { since?: number }): PerformanceMeasure[] {
      return getPerformanceMeasures(options) as unknown as PerformanceMeasure[]
    },

    // === AI Context ===

    /**
     * Enrich an error entry with AI context
     * @param error - Error entry to enrich
     * @returns Enriched error entry
     */
    enrichError(error: LogEntry): Promise<LogEntry> {
      // enrichErrorWithAiContext expects ErrorEntryForEnrichment which is compatible with LogEntry
      // The return type EnrichedErrorEntry extends LogEntry, so we can safely cast
      return enrichErrorWithAiContext(error as { stack?: string; message?: string }) as Promise<LogEntry>
    },

    /**
     * Enable or disable AI context enrichment
     * @param enabled
     */
    setAiContext(enabled: boolean): void {
      setAiContextEnabled(enabled)
    },

    /**
     * Enable or disable state snapshot in AI context
     * @param enabled
     */
    setStateSnapshot(enabled: boolean): void {
      setAiContextStateSnapshot(enabled)
    },

    // === Reproduction Scripts ===

    /**
     * Record an enhanced action (for testing)
     * @param type - Action type (click, input, keypress, navigate, select, scroll)
     * @param element - Target element
     * @param opts - Options
     */
    recordAction(
      type: 'click' | 'input' | 'keypress' | 'navigate' | 'select' | 'scroll',
      element: Element,
      opts?: Record<string, unknown>
    ): void {
      recordEnhancedAction(type, element, opts)
    },

    /**
     * Get the enhanced action buffer
     * @returns
     */
    getEnhancedActions(): EnhancedAction[] {
      return getEnhancedActionBuffer() as unknown as EnhancedAction[]
    },

    /**
     * Clear the enhanced action buffer
     */
    clearEnhancedActions(): void {
      clearEnhancedActionBuffer()
    },

    /**
     * Generate a Playwright reproduction script
     * @param opts - Generation options
     * @returns Playwright test script
     */
    generateScript(opts?: Record<string, unknown>): string {
      // Uses the internal enhanced action buffer which is populated by recordEnhancedAction
      return generatePlaywrightScript(getEnhancedActionBuffer(), opts)
    },

    /**
     * Compute multi-strategy selectors for an element
     * @param element
     * @returns
     */
    getSelectors(element: Element): SelectorStrategies {
      return computeSelectors(element) as unknown as SelectorStrategies
    },

    /**
     * Set input value and trigger React/Vue/Svelte change events
     * Works with frameworks that track form state internally by dispatching
     * the events that frameworks listen for.
     *
     * @param selector - CSS selector for the input element
     * @param value - Value to set (string for text inputs, boolean for checkboxes)
     * @returns true if successful, false if element not found
     *
     * @example
     * // Text input
     * window.__kaboom.setInputValue('input[name="email"]', 'test@example.com')
     *
     * // Checkbox
     * window.__kaboom.setInputValue('input[type="checkbox"]', true)
     *
     * // Select dropdown
     * window.__kaboom.setInputValue('select[name="country"]', 'US')
     */
    setInputValue(selector: string, value: string | boolean): boolean {
      const element = document.querySelector(selector)
      if (!element) {
        console.error('[KaBOOM!] Element not found:', selector)
        return false
      }

      try {
        if (!setNativeValue(element, value)) {
          console.error('[KaBOOM!] Element is not a form input:', selector)
          return false
        }

        // Dispatch events that React/Vue/Svelte listen for
        element.dispatchEvent(new Event('input', { bubbles: true }))
        element.dispatchEvent(new Event('change', { bubbles: true }))
        element.dispatchEvent(new Event('blur', { bubbles: true }))
        return true
      } catch (err) {
        console.error('[KaBOOM!] Failed to set input value:', err)
        return false
      }
    },

    /**
     * Version of the Kaboom API
     */
    version: __KABOOM_VERSION__
  }
}

/**
 * Uninstall the window.__kaboom API
 */
export function uninstallKaboomAPI(): void {
  if (typeof window !== 'undefined' && window.__kaboom) {
    delete window.__kaboom
  }
}
