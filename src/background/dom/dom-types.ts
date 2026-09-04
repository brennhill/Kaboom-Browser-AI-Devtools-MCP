/**
 * Purpose: Shared type definitions for DOM action parameters and results used by dispatch and injected primitives.
 * Docs: docs/features/feature/interact-explore/index.md
 */

export interface DOMMutationEntry {
  type: 'added' | 'removed' | 'attribute'
  tag?: string
  id?: string
  class?: string
  text_preview?: string
  attribute?: string
  old_value?: string
  new_value?: string
}

export interface ScopeRect {
  x: number
  y: number
  width: number
  height: number
}

export interface BoundingBox {
  x: number
  y: number
  width: number
  height: number
}

export interface DOMResult {
  success: boolean
  action: string
  selector: string
  value?: unknown
  candidate_count?: number
  scope_rect_used?: ScopeRect
  match_count?: number
  match_strategy?: string
  matched?: {
    tag?: string
    role?: string
    aria_label?: string
    text_preview?: string
    classes?: string[]
    selector?: string
    element_id?: string
    bbox?: BoundingBox
    scope_selector_used?: string
    scope_rect_used?: ScopeRect
    frame_id?: number
  }
  candidates?: Array<{
    tag?: string
    role?: string
    aria_label?: string
    text_preview?: string
    selector?: string
    element_id?: string
    bbox?: BoundingBox
    visible?: boolean
  }>
  auto_scrolled?: boolean
  ambiguous_matches?: {
    total_count: number
    warning: string
    candidates: Array<{
      tag: string
      element_id: string
      text_preview?: string
    }>
  }
  reason?: string
  error?: string
  message?: string
  dom_summary?: string
  timing?: { total_ms: number }
  dom_changes?: { added: number; removed: number; modified: number; summary: string }
  dom_mutations?: DOMMutationEntry[]
  viewport?: {
    scroll_x: number
    scroll_y: number
    viewport_width: number
    viewport_height: number
    page_height: number
  }
  analysis?: string
  insertion_strategy?: string
  ranked_candidates?: Array<{
    element_id: string
    tag: string
    text_preview?: string
    score: number
  }>
  suggested_element_id?: string
  strategy?: string
  selector_used?: string
  overlay_type?: string
  overlay_selector?: string
  overlay_text_preview?: string
  overlay_warning?: string
  // #445: extension vs page overlay source
  overlay_source?: 'extension' | 'page'
  // wait_for_stable fields (#344)
  stable?: boolean
  timed_out?: boolean
  waited_ms?: number
  mutations_observed?: number
  stability_ms?: number
  // wait_for enhanced fields (#371)
  matched_text?: string
  absent?: boolean
  // auto_dismiss_overlays fields (#342)
  dismissed_count?: number
  // get_text structured mode fields (#390)
  sections?: Array<{
    header?: string
    content: string
    expanded?: boolean
    tag: string
  }>
  section_count?: number
  // Pointer gesture evidence (kaboom-05ue.5). These say what was actually dispatched, so a
  // gesture that reports success can be checked against what the page could have seen.
  x?: number
  y?: number
  button?: string
  click_count?: number
  modifiers?: number
  /** right_click only: whether the contextmenu event reached the page. */
  context_menu?: boolean
  /** drag only: caller-supplied path points, and the moves actually dispatched. */
  path_points?: number
  move_events?: number
  /** drag only: false when Chrome refused Input.dispatchDragEvent and only the pointer path ran. */
  html5_drag?: boolean
  delta_x?: number
  delta_y?: number
}

export interface DOMPrimitiveOptions {
  text?: string
  key?: string
  value?: string
  direction?: string
  clear?: boolean
  checked?: boolean
  name?: string
  timeout_ms?: number
  stability_ms?: number
  analyze?: boolean
  observe_mutations?: boolean
  element_id?: string
  scope_selector?: string
  scope_rect?: ScopeRect
  nth?: number
  new_tab?: boolean
  url_contains?: string
  absent?: boolean
  structured?: boolean
  // Pointer gesture inputs (kaboom-05ue.5). x/y address a viewport coordinate directly;
  // `drag_path` is a route, not a pair of endpoints — HTML5 drag-and-drop and canvas apps
  // both start their drag on the first intermediate move, so a two-point jump drags nothing.
  x?: number
  y?: number
  /** The drag route. Named drag_path because `path` is already the cookie path string. */
  drag_path?: Array<{ x: number; y: number }>
  /** ctrl | shift | alt | cmd (meta), combinable. Folded into Chrome's modifier bitmask. */
  modifiers?: string[]
  delta_x?: number
  delta_y?: number
  width?: number
  height?: number
  scale?: number
}

export interface DOMActionParams extends DOMPrimitiveOptions {
  action?: string
  selector?: string
  reason?: string
  frame?: string | number
  // list_interactive filters (#369)
  text_contains?: string
  role?: string
  exclude_nav?: boolean
  visible_only?: boolean
  // query action (#370)
  query_type?: string
  attribute_names?: string[]
  // #599: input dispatch strategy. "auto" (default) tries CDP hardware events first
  // (isTrusted:true) and falls back to DOM primitives; "dom" skips CDP and drives
  // click/type through in-page element.click() + native-setter input events, which
  // reliably fires React/Vue/Svelte controlled-input onChange and delegated onClick.
  dispatch?: 'auto' | 'dom'
}
