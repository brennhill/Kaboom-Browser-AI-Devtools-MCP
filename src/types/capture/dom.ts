/**
 * Purpose: Defines DOM query result and page metadata contracts returned by extension query handlers.
 * Why: Keeps DOM inspection responses deterministic for tool consumers and serialization boundaries.
 * Docs: docs/features/feature/query-dom/index.md
 */

/**
 * @fileoverview DOM Query Types
 * DOM element queries and page information
 */

/**
 * DOM element info from query
 */
export interface DomElementInfo {
  readonly tag: string
  readonly id?: string
  readonly classes?: readonly string[]
  readonly text?: string
  readonly html?: string
  readonly attributes?: Readonly<Record<string, string>>
  readonly boundingRect?: {
    readonly x: number
    readonly y: number
    readonly width: number
    readonly height: number
  }
}

/**
 * DOM query result
 */
export interface DomQueryResult {
  readonly elements: readonly DomElementInfo[]
  readonly count: number
  readonly truncated: boolean
  readonly error?: string
}

/**
 * Page info result
 */
export interface PageInfo {
  readonly url: string
  readonly title: string
  readonly favicon?: string
  readonly status: string
  readonly viewport?: {
    readonly width: number
    readonly height: number
  }
}
