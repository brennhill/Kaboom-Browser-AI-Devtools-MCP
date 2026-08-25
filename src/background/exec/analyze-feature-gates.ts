/**
 * Purpose: Detects plan-gated, auth-gated, and usage-limited features on a page.
 * Why: Automates competitive analysis by identifying free vs. paid features.
 * Docs: docs/features/feature/analyze-tool/index.md
 */

// analyze-feature-gates.ts — Self-contained feature gate detection for chrome.scripting.executeScript.
// Scans the page for upgrade prompts, lock icons, auth-gate messages, and usage limits.
// MUST remain self-contained — Chrome serializes the function source only (no closures).

export function analyzeFeatureGates(): {
  plan_gates: Array<{
    feature: string
    required_plan?: string
    selector: string
    text: string
  }>
  auth_gates: Array<{
    feature: string
    provider?: string
    selector: string
    text: string
  }>
  usage_limits: Array<{
    feature: string
    text: string
    selector: string
  }>
  total_gates: number
} {
  const planGates: Array<{
    feature: string
    required_plan?: string
    selector: string
    text: string
  }> = []
  const authGates: Array<{
    feature: string
    provider?: string
    selector: string
    text: string
  }> = []
  const usageLimits: Array<{
    feature: string
    text: string
    selector: string
  }> = []

  // --- Helpers ---

  function buildSelector(el: Element): string {
    if (el.id) return `#${CSS.escape(el.id)}`
    const tag = el.tagName.toLowerCase()
    const rawClass = el.getAttribute('class') || ''
    const firstClass = rawClass.trim().split(/\s+/)[0] || ''
    const cls = firstClass ? `.${CSS.escape(firstClass)}` : ''
    return `${tag}${cls}`
  }

  function nearestFeatureName(el: Element): string {
    // Look for a heading or label near the gated element
    const parent = el.parentElement
    if (!parent) return ''
    const headings = parent.querySelectorAll('h1, h2, h3, h4, h5, h6, [role="heading"], label')
    for (let i = 0; i < headings.length; i++) {
      const text = (headings[i] as HTMLElement).textContent?.trim() || ''
      if (text.length > 0 && text.length < 80) return text
    }
    // Fallback: aria-label or title of parent
    return parent.getAttribute('aria-label') || parent.getAttribute('title') || ''
  }

  function isVisible(el: Element): boolean {
    const htmlEl = el as HTMLElement
    if (!htmlEl.getBoundingClientRect) return false
    const rect = htmlEl.getBoundingClientRect()
    return rect.width > 0 && rect.height > 0
  }

  // --- Shared scanner: walks visible elements, dedupes by text, applies per-category matching ---

  function scanGateElements(opts: {
    maxLength: number
    minLength: number
    count: () => number
    matches: (text: string, el: HTMLElement) => boolean
    onMatch: (el: HTMLElement, text: string) => void
  }): void {
    const seenTexts = new Set<string>()
    for (let i = 0; i < allElements.length && opts.count() < 20; i++) {
      const el = allElements[i] as HTMLElement
      if (!isVisible(el)) continue

      const text = (el.textContent || '').trim()
      if (text.length > opts.maxLength || text.length < opts.minLength) continue

      if (opts.matches(text, el) && !seenTexts.has(text)) {
        seenTexts.add(text)
        opts.onMatch(el, text)
      }
    }
  }

  // --- Plan gate detection ---

  const planPatterns =
    /\b(upgrade|unlock|premium|pro plan|enterprise|paid|subscribe|pricing|go pro|get started|start free trial)\b/i
  const planNamePattern = /\b(free|starter|basic|pro|premium|plus|business|enterprise|creator|growth|team|scale)\b/i

  // Scan for elements with gate-related text
  const allElements = document.body.querySelectorAll('*')

  scanGateElements({
    maxLength: 200,
    minLength: 3,
    count: () => planGates.length,
    matches: (text, el) => {
      // Check for gate-related CSS classes
      const className = String(el.className || '').toLowerCase()
      const hasGateClass = /\b(locked|gated|premium-only|pro-only|disabled|upgrade)\b/.test(className)

      // Check for lock icon SVGs or font-awesome icons
      const hasLockIcon = el.querySelector('svg[data-icon="lock"], .fa-lock, .icon-lock, [class*="lock"]') !== null

      return planPatterns.test(text) || hasGateClass || hasLockIcon
    },
    onMatch: (el, text) => {
      const planMatch = text.match(planNamePattern)
      planGates.push({
        feature: nearestFeatureName(el) || text.slice(0, 60),
        required_plan: planMatch ? planMatch[1] : undefined,
        selector: buildSelector(el),
        text: text.slice(0, 120)
      })
    }
  })

  // --- Auth gate detection ---

  const authPatterns =
    /\b(connect your|authenticate with|sign in with|log in with|link your|authorize|oauth|connect .+ account)\b/i
  const providerPattern =
    /\b(google|facebook|twitter|linkedin|github|slack|instagram|gumroad|stripe|shopify|zapier|hubspot|salesforce)\b/i

  scanGateElements({
    maxLength: 200,
    minLength: 5,
    count: () => authGates.length,
    matches: (text) => authPatterns.test(text),
    onMatch: (el, text) => {
      const providerMatch = text.match(providerPattern)
      authGates.push({
        feature: nearestFeatureName(el) || text.slice(0, 60),
        provider: providerMatch ? providerMatch[1] : undefined,
        selector: buildSelector(el),
        text: text.slice(0, 120)
      })
    }
  })

  // --- Usage limit detection ---

  const usagePatterns = /\b(\d+\s*\/\s*\d+|remaining|left this|quota|limit reached|usage|credits?\s+\d|allowance)\b/i

  scanGateElements({
    maxLength: 150,
    minLength: 3,
    count: () => usageLimits.length,
    matches: (text) => usagePatterns.test(text),
    onMatch: (el, text) => {
      usageLimits.push({
        feature: nearestFeatureName(el) || text.slice(0, 60),
        text: text.slice(0, 120),
        selector: buildSelector(el)
      })
    }
  })

  return {
    plan_gates: planGates,
    auth_gates: authGates,
    usage_limits: usageLimits,
    total_gates: planGates.length + authGates.length + usageLimits.length
  }
}
