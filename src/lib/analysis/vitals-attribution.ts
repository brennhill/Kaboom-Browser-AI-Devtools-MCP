/**
 * Purpose: Build bounded, content-free attribution for browser Web Vitals entries.
 * Docs: docs/features/feature/web-vitals/index.md
 */

const MAX_CLASSES = 4
const MAX_SHIFTS = 10
const MAX_SHIFT_NODES = 5
const MAX_LONG_TASKS = 20

export interface ElementDescriptor {
  tag: string
  id?: string
  classes?: string[]
  role?: string
}

export interface LCPAttribution {
  element?: ElementDescriptor
  time_to_first_byte_ms: number
  resource_load_delay_ms: number
  resource_load_duration_ms: number
  element_render_delay_ms: number
  attribution_status: 'available' | 'element_unavailable'
}

export interface INPAttribution {
  event_type: string
  target?: ElementDescriptor
  input_delay_ms: number
  processing_ms: number
  presentation_delay_ms: number
  interaction_id: number
}

export interface CLSAttribution {
  shifts: Array<{ value: number; start_time: number; nodes: ElementDescriptor[] }>
  attribution_status: 'available' | 'nodes_unavailable'
}

export interface LongTaskAttribution {
  name: string
  start_time: number
  duration: number
  source_stack?: string[]
  source_stack_status: 'available' | 'unavailable'
}

export interface VitalsAttribution {
  lcp?: LCPAttribution
  inp?: INPAttribution
  cls: CLSAttribution
  long_tasks: LongTaskAttribution[]
}

interface ElementLike {
  tagName?: string
  id?: string
  classList?: Iterable<string>
  getAttribute?: (name: string) => string | null
}

interface LCPEntryLike extends PerformanceEntry {
  element?: ElementLike | null
  loadTime?: number
  renderTime?: number
}

interface LayoutShiftEntryLike extends PerformanceEntry {
  value?: number
  sources?: Array<{ node?: ElementLike | null }>
}

interface EventTimingEntryLike extends PerformanceEntry {
  interactionId?: number
  processingStart?: number
  processingEnd?: number
  target?: ElementLike | null
}

interface LongTaskEntryLike extends PerformanceEntry {
  attribution?: Array<{ name?: string; containerType?: string }>
}

let lcpEntry: LCPEntryLike | null = null
let inpEntry: EventTimingEntryLike | null = null
const clsShifts: CLSAttribution['shifts'] = []
const longTasks: LongTaskAttribution[] = []

export function resetVitalsAttribution(): void {
  lcpEntry = null
  inpEntry = null
  clsShifts.length = 0
  longTasks.length = 0
}

export function recordLCPAttribution(entry: PerformanceEntry): void {
  lcpEntry = entry as LCPEntryLike
}

export function recordINPAttribution(entry: PerformanceEntry): void {
  inpEntry = entry as EventTimingEntryLike
}

export function recordCLSAttribution(entry: PerformanceEntry): void {
  const shift = entry as LayoutShiftEntryLike
  if (clsShifts.length >= MAX_SHIFTS) return
  const nodes = (shift.sources ?? [])
    .slice(0, MAX_SHIFT_NODES)
    .map((source) => describeElement(source.node))
    .filter((node): node is ElementDescriptor => node !== undefined)
  clsShifts.push({ value: finite(shift.value), start_time: finite(shift.startTime), nodes })
}

export function recordLongTaskAttribution(entry: PerformanceEntry): void {
  if (longTasks.length >= MAX_LONG_TASKS) return
  const task = entry as LongTaskEntryLike
  longTasks.push({
    name: bounded(task.name || 'longtask', 64),
    start_time: finite(task.startTime),
    duration: finite(task.duration),
    source_stack_status: 'unavailable'
  })
}

export function getVitalsAttribution(responseStart = 0): VitalsAttribution {
  return {
    ...(lcpEntry ? { lcp: buildLCPAttribution(lcpEntry, responseStart) } : {}),
    ...(inpEntry ? { inp: buildINPAttribution(inpEntry) } : {}),
    cls: {
      shifts: clsShifts.map((shift) => ({ ...shift, nodes: shift.nodes.map((node) => ({ ...node })) })),
      attribution_status: clsShifts.some((shift) => shift.nodes.length > 0) ? 'available' : 'nodes_unavailable'
    },
    long_tasks: longTasks.map((task) => ({ ...task }))
  }
}

function buildLCPAttribution(entry: LCPEntryLike, responseStart: number): LCPAttribution {
  const loadTime = finite(entry.loadTime)
  const renderTime = finite(entry.renderTime || entry.startTime)
  const resourceStart = loadTime > 0 ? loadTime : renderTime
  const element = describeElement(entry.element)
  return {
    ...(element ? { element } : {}),
    time_to_first_byte_ms: finite(responseStart),
    resource_load_delay_ms: Math.max(0, resourceStart - finite(responseStart)),
    resource_load_duration_ms: Math.max(0, loadTime - resourceStart),
    element_render_delay_ms: Math.max(0, renderTime - (loadTime || resourceStart)),
    attribution_status: element ? 'available' : 'element_unavailable'
  }
}

function buildINPAttribution(entry: EventTimingEntryLike): INPAttribution {
  const processingStart = finite(entry.processingStart)
  const processingEnd = finite(entry.processingEnd)
  const startTime = finite(entry.startTime)
  const duration = finite(entry.duration)
  const target = describeElement(entry.target)
  return {
    event_type: bounded(entry.name || 'event', 64),
    ...(target ? { target } : {}),
    input_delay_ms: Math.max(0, processingStart - startTime),
    processing_ms: Math.max(0, processingEnd - processingStart),
    presentation_delay_ms: Math.max(0, startTime + duration - processingEnd),
    interaction_id: finite(entry.interactionId)
  }
}

function describeElement(element: ElementLike | null | undefined): ElementDescriptor | undefined {
  if (!element) return undefined
  const tag = element.tagName?.toLowerCase()
  if (!tag) return undefined
  const id = bounded(element.id ?? '', 128)
  const classes = Array.from(element.classList ?? [])
    .filter((name) => typeof name === 'string' && name.length > 0)
    .slice(0, MAX_CLASSES)
    .map((name) => bounded(name, 64))
  const role = bounded(element.getAttribute?.('role') ?? '', 64)
  return {
    tag: bounded(tag, 32),
    ...(id ? { id } : {}),
    ...(classes.length > 0 ? { classes } : {}),
    ...(role ? { role } : {})
  }
}

function finite(value: number | undefined): number {
  return Number.isFinite(value) ? Number(value) : 0
}

function bounded(value: string, max: number): string {
  return value.slice(0, max)
}
