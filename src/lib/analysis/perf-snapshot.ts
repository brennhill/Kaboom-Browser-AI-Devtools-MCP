/**
 * Purpose: Observes web vitals (FCP, LCP, CLS, INP), long tasks, and resource timing to build comprehensive performance snapshots.
 * Docs: docs/features/feature/performance-audit/index.md
 */

/**
 * @fileoverview Performance snapshot capture.
 * Observes web vitals (FCP, LCP, CLS, INP), long tasks, and resource timing
 * to build comprehensive performance snapshots.
 */

import { MAX_LONG_TASKS, MAX_SLOWEST_REQUESTS, MAX_URL_LENGTH } from '../constants.js'
import { postAuthenticatedPageMessage } from '../page/channel.js'
import {
  getVitalsAttribution,
  recordCLSAttribution,
  recordINPAttribution,
  recordLCPAttribution,
  recordLongTaskAttribution,
  resetVitalsAttribution,
  type VitalsAttribution
} from './vitals-attribution.js'

interface ResourceByType {
  count: number
  size: number
}

interface SlowRequest {
  url: string
  duration: number
  size: number
}

interface ResourceTimingSummary {
  request_count: number
  transfer_size: number
  decoded_size: number
  by_type: Record<string, ResourceByType>
  slowest_requests: SlowRequest[]
}

interface LongTaskMetrics {
  count: number
  total_blocking_time: number
  longest: number
}

interface NetworkTiming {
  dom_content_loaded: number
  load: number
  first_contentful_paint: number | null
  largest_contentful_paint: number | null
  interaction_to_next_paint: number | null
  time_to_first_byte: number
  dom_interactive: number
}

interface UserTimingEntry {
  name: string
  start_time: number
  duration?: number
}

interface PerformanceSnapshotData {
  url: string
  timestamp: string
  timing: NetworkTiming
  network: ResourceTimingSummary
  long_tasks: LongTaskMetrics
  cumulative_layout_shift: number
  vitals_attribution: VitalsAttribution
  user_timing?: {
    marks: UserTimingEntry[]
    measures: UserTimingEntry[]
  }
}

// Performance snapshot state
let perfSnapshotEnabled = true
let longTaskEntries: PerformanceEntry[] = []
let longTaskObserver: PerformanceObserver | null = null
let paintObserver: PerformanceObserver | null = null
let lcpObserver: PerformanceObserver | null = null
let clsObserver: PerformanceObserver | null = null
let inpObserver: PerformanceObserver | null = null
let fcpValue: number | null = null
let lcpValue: number | null = null
let clsValue = 0
let inpValue: number | null = null

/**
 * Map resource initiator types to standard categories
 */
export function mapInitiatorType(type: string): string {
  switch (type) {
    case 'script':
      return 'script'
    case 'link':
    case 'css':
      return 'style'
    case 'img':
      return 'image'
    case 'fetch':
    case 'xmlhttprequest':
      return 'fetch'
    case 'font':
      return 'font'
    default:
      return 'other'
  }
}

/**
 * Aggregate resource timing entries into a network summary
 */
export function aggregateResourceTiming(): ResourceTimingSummary {
  const resources = (performance.getEntriesByType('resource') as PerformanceResourceTiming[]) || []
  const byType: Record<string, ResourceByType> = {}
  let transferSize = 0
  let decodedSize = 0

  for (const entry of resources) {
    const category = mapInitiatorType(entry.initiatorType)
    // eslint-disable-next-line security/detect-object-injection -- category from mapInitiatorType returns known resource type strings
    if (!byType[category]) {
      // eslint-disable-next-line security/detect-object-injection -- category from mapInitiatorType returns known resource type strings
      byType[category] = { count: 0, size: 0 }
    }
    // eslint-disable-next-line security/detect-object-injection -- category from mapInitiatorType returns known resource type strings
    byType[category].count++
    // eslint-disable-next-line security/detect-object-injection -- category from mapInitiatorType returns known resource type strings
    byType[category].size += entry.transferSize || 0
    transferSize += entry.transferSize || 0
    decodedSize += entry.decodedBodySize || 0
  }

  // Top N slowest requests
  const sorted = [...resources].sort((a, b) => b.duration - a.duration)
  const slowestRequests: SlowRequest[] = sorted.slice(0, MAX_SLOWEST_REQUESTS).map((r) => ({
    url: r.name.length > MAX_URL_LENGTH ? r.name.slice(0, MAX_URL_LENGTH) : r.name,
    duration: r.duration,
    size: r.transferSize || 0
  }))

  return {
    request_count: resources.length,
    transfer_size: transferSize,
    decoded_size: decodedSize,
    by_type: byType,
    slowest_requests: slowestRequests
  }
}

/**
 * Capture a performance snapshot with navigation timing and network summary
 */
export function capturePerformanceSnapshot(): PerformanceSnapshotData | null {
  const navEntries = (performance.getEntriesByType('navigation') as PerformanceNavigationTiming[]) || []
  if (!navEntries || navEntries.length === 0) return null

  const nav = navEntries[0]
  if (!nav) return null

  const timing: NetworkTiming = {
    dom_content_loaded: nav.domContentLoadedEventEnd,
    load: nav.loadEventEnd,
    first_contentful_paint: getFCP(),
    largest_contentful_paint: getLCP(),
    interaction_to_next_paint: getINP(),
    time_to_first_byte: nav.responseStart - nav.requestStart,
    dom_interactive: nav.domInteractive
  }

  const network = aggregateResourceTiming()
  const longTasks = getLongTaskMetrics()

  // Capture user timing marks and measures
  const marks = (performance.getEntriesByType('mark') as PerformanceEntry[]) || []
  const measures = (performance.getEntriesByType('measure') as PerformanceEntry[]) || []
  const userTiming =
    marks.length > 0 || measures.length > 0
      ? {
          marks: marks.slice(-50).map((m) => ({ name: m.name, start_time: m.startTime })),
          measures: measures.slice(-50).map((m) => ({ name: m.name, start_time: m.startTime, duration: m.duration }))
        }
      : undefined

  return {
    url: window.location.pathname,
    timestamp: new Date().toISOString(),
    timing,
    network,
    long_tasks: longTasks,
    cumulative_layout_shift: getCLS(),
    vitals_attribution: getVitalsAttribution(nav.responseStart),
    user_timing: userTiming
  }
}

/**
 * Install performance observers for long tasks, paint, LCP, and CLS
 */
export function installPerfObservers(): void {
  longTaskEntries = []
  fcpValue = null
  lcpValue = null
  clsValue = 0
  inpValue = null
  resetVitalsAttribution()

  // Long task observer
  // #lizard forgives
  longTaskObserver = new PerformanceObserver((list: PerformanceObserverEntryList): void => {
    const entries = list.getEntries()
    for (const entry of entries) {
      if (longTaskEntries.length < MAX_LONG_TASKS) {
        longTaskEntries.push(entry)
        recordLongTaskAttribution(entry)
      }
    }
  })
  longTaskObserver.observe({ type: 'longtask' })

  // Paint observer (FCP)
  paintObserver = new PerformanceObserver((list: PerformanceObserverEntryList): void => {
    for (const entry of list.getEntries()) {
      if (entry.name === 'first-contentful-paint') {
        fcpValue = entry.startTime
      }
    }
  })
  paintObserver.observe({ type: 'paint', buffered: true })

  // LCP observer
  lcpObserver = new PerformanceObserver((list: PerformanceObserverEntryList): void => {
    const entries = list.getEntries()
    if (entries.length > 0) {
      const lastEntry = entries[entries.length - 1]
      if (lastEntry) {
        lcpValue = lastEntry.startTime
        recordLCPAttribution(lastEntry)
      }
    }
  })
  lcpObserver.observe({ type: 'largest-contentful-paint', buffered: true })

  // CLS observer
  // LayoutShift interface extends PerformanceEntry with hadRecentInput and value
  clsObserver = new PerformanceObserver((list: PerformanceObserverEntryList): void => {
    for (const entry of list.getEntries()) {
      const clsEntry = entry as PerformanceEntry & { hadRecentInput?: boolean; value?: number }
      if (!clsEntry.hadRecentInput) {
        clsValue += clsEntry.value || 0
        recordCLSAttribution(entry)
      }
    }
  })
  clsObserver.observe({ type: 'layout-shift', buffered: true })

  // INP observer (Interaction to Next Paint)
  // Event timing entries have interactionId and duration properties
  inpObserver = new PerformanceObserver((list: PerformanceObserverEntryList): void => {
    for (const entry of list.getEntries()) {
      const inpEntry = entry as PerformanceEntry & { interactionId?: number }
      if (inpEntry.interactionId) {
        if (inpValue === null || inpEntry.duration > inpValue) {
          inpValue = inpEntry.duration
          recordINPAttribution(entry)
        }
      }
    }
  })
  inpObserver.observe({ type: 'event', durationThreshold: 40, buffered: true } as PerformanceObserverInit)
}

/**
 * Disconnect all performance observers
 */
export function uninstallPerfObservers(): void {
  if (longTaskObserver) {
    longTaskObserver.disconnect()
    longTaskObserver = null
  }
  if (paintObserver) {
    paintObserver.disconnect()
    paintObserver = null
  }
  if (lcpObserver) {
    lcpObserver.disconnect()
    lcpObserver = null
  }
  if (clsObserver) {
    clsObserver.disconnect()
    clsObserver = null
  }
  if (inpObserver) {
    inpObserver.disconnect()
    inpObserver = null
  }
  longTaskEntries = []
}

/**
 * Get accumulated long task metrics
 */
export function getLongTaskMetrics(): LongTaskMetrics {
  let totalBlockingTime = 0
  let longest = 0

  for (const entry of longTaskEntries) {
    const blocking = entry.duration - 50
    if (blocking > 0) totalBlockingTime += blocking
    if (entry.duration > longest) longest = entry.duration
  }

  return {
    count: longTaskEntries.length,
    total_blocking_time: totalBlockingTime,
    longest
  }
}

/**
 * Get First Contentful Paint value
 */
export function getFCP(): number | null {
  return fcpValue
}

/**
 * Get Largest Contentful Paint value
 */
export function getLCP(): number | null {
  return lcpValue
}

/**
 * Get Cumulative Layout Shift value
 */
export function getCLS(): number {
  return clsValue
}

/**
 * Get Interaction to Next Paint value
 */
export function getINP(): number | null {
  return inpValue
}

export { getVitalsAttribution }

/**
 * Send performance snapshot via postMessage to content script
 */
export function sendPerformanceSnapshot(): void {
  if (!perfSnapshotEnabled) return

  const snapshot = capturePerformanceSnapshot()
  if (!snapshot) return

  postAuthenticatedPageMessage({ type: 'kaboom_performance_snapshot', payload: snapshot })
}

// Debounce timer for snapshot re-sends triggered by user timing changes
let snapshotResendTimer: ReturnType<typeof setTimeout> | null = null

/**
 * Schedule a debounced re-send of the performance snapshot.
 * Called when user timing marks/measures are created to keep server data fresh.
 */
export function scheduleSnapshotResend(): void {
  if (!perfSnapshotEnabled) return
  if (snapshotResendTimer) clearTimeout(snapshotResendTimer)
  snapshotResendTimer = setTimeout(() => {
    snapshotResendTimer = null
    sendPerformanceSnapshot()
  }, 500)
}

/**
 * Check if performance snapshot capture is enabled
 */
export function isPerformanceSnapshotEnabled(): boolean {
  return perfSnapshotEnabled
}

/**
 * Enable or disable performance snapshot capture
 */
export function setPerformanceSnapshotEnabled(enabled: boolean): void {
  perfSnapshotEnabled = enabled
}
