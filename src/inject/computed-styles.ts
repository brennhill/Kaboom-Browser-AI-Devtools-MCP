/**
 * Purpose: Queries elements by CSS selector and returns computed CSS properties, box model dimensions, custom properties, and contrast ratios for the analyze tool.
 * Docs: docs/features/feature/analyze-tool/index.md
 *
 * CONTRACT: this file reports what the page renders and judges none of it. It
 * never decides which value is a token, what the norm is, or what counts as
 * drift — that arithmetic lives in Go (cmd/browser-agent/internal/toolanalyze/
 * designdrift) where it is table-tested. Content scripts are bundled and
 * awkward to test, so keep this a measurement.
 */

// computed-styles.ts — Computed styles query handler for inject context.

import type { WireStyleProbeElement, WireStyleProbeResult } from '../types/wire/wire-style-probe.js'

/**
 * Default CSS properties to return when no filter is specified.
 *
 * Longhands, not shorthands: `padding: 15px` has to expand into four separate
 * comparisons for token matching, and gap analysis needs the individual margin
 * edges. Asking getComputedStyle for `margin` returns a collapsed string that
 * cannot be compared against a spacing scale.
 */
const DEFAULT_PROPERTIES = [
  'color',
  'background-color',
  'font-size',
  'font-family',
  'font-weight',
  'line-height',
  'letter-spacing',
  'display',
  'position',
  'width',
  'height',
  'margin-top',
  'margin-right',
  'margin-bottom',
  'margin-left',
  'padding-top',
  'padding-right',
  'padding-bottom',
  'padding-left',
  'border-top-width',
  'border-radius',
  'border-color',
  'opacity',
  'visibility',
  'z-index',
  'overflow',
  'text-align',
  'text-decoration',
  'box-sizing'
]

/**
 * Ceiling on how many elements one probe may return.
 *
 * The cap exists to bound payload size, not to sample: exceeding it is reported
 * through match_count and truncated so a caller never computes a majority or a
 * spacing rhythm over a silently shortened set.
 */
const MAX_ELEMENTS_CEILING = 500
const DEFAULT_MAX_ELEMENTS = 50

interface ComputedStylesParams {
  selector: string
  properties?: string[]
  /** Element cap for this query; clamped to MAX_ELEMENTS_CEILING. */
  max_elements?: number
  /** Collect CSS custom properties (:root table and per-element scope). */
  include_custom_properties?: boolean
}

/**
 * Compute the relative luminance of an RGB color per WCAG 2.0.
 */
function relativeLuminance(r: number, g: number, b: number): number {
  const [rs, gs, bs] = [r / 255, g / 255, b / 255].map((c) =>
    c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
  )
  return 0.2126 * rs! + 0.7152 * gs! + 0.0722 * bs!
}

/**
 * Parse a CSS color string (rgb/rgba) into [r, g, b] components.
 */
function parseRGBColor(colorStr: string): [number, number, number] | null {
  const match = colorStr.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/)
  if (!match) return null
  return [parseInt(match[1]!, 10), parseInt(match[2]!, 10), parseInt(match[3]!, 10)]
}

/**
 * Compute contrast ratio between foreground and background colors.
 */
function computeContrastRatio(fgColor: string, bgColor: string): number | undefined {
  const fg = parseRGBColor(fgColor)
  const bg = parseRGBColor(bgColor)
  if (!fg || !bg) return undefined

  const fgLum = relativeLuminance(fg[0], fg[1], fg[2])
  const bgLum = relativeLuminance(bg[0], bg[1], bg[2])

  const lighter = Math.max(fgLum, bgLum)
  const darker = Math.min(fgLum, bgLum)

  return Math.round(((lighter + 0.05) / (darker + 0.05)) * 100) / 100
}

/**
 * Build a minimal CSS selector for an element.
 */
function buildSelector(el: Element): string {
  if (el.id) return `#${el.id}`
  const tag = el.tagName.toLowerCase()
  const classes = Array.from(el.classList)
    .slice(0, 3)
    .map((c) => `.${c}`)
    .join('')
  return tag + classes
}

/**
 * Read every CSS custom property declared on a style rule set.
 *
 * getComputedStyle exposes custom properties by index in modern Chrome, which
 * is the only way to enumerate them — there is no `--*` wildcard getter.
 */
function collectCustomProperties(style: CSSStyleDeclaration): Record<string, string> {
  const props: Record<string, string> = {}
  for (let i = 0; i < style.length; i++) {
    const name = style.item(i)
    if (name && name.startsWith('--')) {
      props[name] = style.getPropertyValue(name).trim()
    }
  }
  return props
}

/**
 * Read the document's :root token table.
 */
function collectRootTokens(): Record<string, string> {
  const root = document.documentElement
  if (!root) return {}
  return collectCustomProperties(window.getComputedStyle(root))
}

/**
 * Report whether an element participates in normal flow.
 *
 * Out-of-flow siblings are not part of a vertical rhythm; including one lets a
 * hidden or absolutely-positioned node manufacture a phantom gap.
 */
function isInFlow(style: CSSStyleDeclaration, rect: DOMRect): boolean {
  const position = style.getPropertyValue('position')
  if (position === 'absolute' || position === 'fixed') return false
  if (style.getPropertyValue('display') === 'none') return false
  if (style.getPropertyValue('visibility') === 'hidden') return false
  if (style.getPropertyValue('float') !== 'none') return false
  return rect.width > 0 && rect.height > 0
}

/**
 * Resolve the element cap, clamped to the documented ceiling.
 */
function resolveMaxElements(requested: number | undefined): number {
  if (typeof requested !== 'number' || !Number.isFinite(requested) || requested <= 0) {
    return DEFAULT_MAX_ELEMENTS
  }
  return Math.min(Math.floor(requested), MAX_ELEMENTS_CEILING)
}

/**
 * Query computed styles for all elements matching a CSS selector.
 */
export function queryComputedStyles(params: ComputedStylesParams): WireStyleProbeResult {
  const elements = document.querySelectorAll(params.selector)
  const propList = params.properties && params.properties.length > 0 ? params.properties : DEFAULT_PROPERTIES
  const results: WireStyleProbeElement[] = []
  const maxElements = resolveMaxElements(params.max_elements)
  const wantCustomProperties = params.include_custom_properties === true

  for (let i = 0; i < elements.length && results.length < maxElements; i++) {
    const el = elements[i]!
    const style = window.getComputedStyle(el)
    const rect = el.getBoundingClientRect()

    const computedStyles: Record<string, string> = {}
    for (const prop of propList) {
      computedStyles[prop] = style.getPropertyValue(prop)
    }

    const parent = el.parentElement
    const parentStyle = parent ? window.getComputedStyle(parent) : null

    const result: WireStyleProbeElement = {
      selector: buildSelector(el),
      tag: el.tagName.toLowerCase(),
      computed_styles: computedStyles,
      box_model: {
        x: Math.round(rect.x),
        y: Math.round(rect.y),
        width: Math.round(rect.width),
        height: Math.round(rect.height),
        top: Math.round(rect.top),
        right: Math.round(rect.right),
        bottom: Math.round(rect.bottom),
        left: Math.round(rect.left)
      },
      index: i,
      in_flow: isInFlow(style, rect),
      ...(parentStyle
        ? {
            parent_display: parentStyle.getPropertyValue('display'),
            parent_gap: parentStyle.getPropertyValue('gap')
          }
        : {}),
      ...(wantCustomProperties ? { custom_properties: collectCustomProperties(style) } : {})
    }

    // Compute contrast ratio for elements that likely contain text
    const color = style.getPropertyValue('color')
    const bgColor = style.getPropertyValue('background-color')
    if (color && bgColor && bgColor !== 'rgba(0, 0, 0, 0)') {
      const ratio = computeContrastRatio(color, bgColor)
      if (ratio !== undefined) {
        ;(result as { contrast_ratio?: number }).contrast_ratio = ratio
      }
    }

    results.push(result)
  }

  return {
    elements: results,
    count: results.length,
    match_count: elements.length,
    truncated: elements.length > results.length,
    ...(wantCustomProperties ? { root_tokens: collectRootTokens() } : {})
  }
}
