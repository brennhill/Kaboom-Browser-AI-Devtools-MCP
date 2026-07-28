/**
 * Purpose: Pure element-result collection, filtering, limiting, and page metadata shared by commands.
 */

export type CommandParams = Record<string, unknown>

export function selectCommandElements(elements: unknown[], params: CommandParams): unknown[] {
  const visible =
    params.visible_only === true
      ? elements.filter((element) => (element as { visible?: boolean }).visible !== false)
      : elements
  const limit = typeof params.limit === 'number' && params.limit > 0 ? params.limit : visible.length
  return visible.slice(0, limit)
}

interface CommandElementResult {
  result?: unknown
}

interface CommandElementPayload {
  success?: boolean
  elements?: unknown[]
  error?: string
  message?: string
}

export function collectCommandElements(
  results: CommandElementResult[],
  limit: number
): { elements: unknown[]; firstError?: string } {
  const elements: unknown[] = []
  let firstError: string | undefined
  for (const item of results) {
    const result = item.result as CommandElementPayload | null | undefined
    if (result?.success === false) {
      firstError ||= result.error || result.message
      continue
    }
    if (result?.elements) {
      elements.push(...result.elements)
      if (elements.length >= limit) break
    }
  }
  return { elements: elements.slice(0, limit), ...(firstError ? { firstError } : {}) }
}

interface CommandTabMetadata {
  url?: string
  title?: string
  status?: string
  favIconUrl?: string
  width?: number
  height?: number
}

export function commandPageMetadata(tab: CommandTabMetadata): Record<string, unknown> {
  return {
    url: tab.url || '',
    title: tab.title || '',
    tab_status: tab.status || '',
    favicon: tab.favIconUrl || '',
    viewport: { width: tab.width, height: tab.height }
  }
}
