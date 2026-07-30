/**
 * Purpose: Defines user-action telemetry contracts and selector strategy shapes used for replay/test generation.
 * Why: Keeps action capture payloads aligned with replay and generated-test consumers.
 * Docs: docs/features/feature/test-generation/index.md
 */

/**
 * @fileoverview User Action Types
 * User interactions for action replay and test generation
 */

/**
 * Action types for user action replay
 */
export type ActionType = 'click' | 'input' | 'scroll' | 'keydown' | 'change' | 'navigate'

/**
 * Basic action entry
 */
export interface ActionEntry {
  readonly type: ActionType
  readonly target: string
  readonly timestamp: string
  readonly value?: string
}

/**
 * Multi-strategy selector result
 */
export interface SelectorStrategies {
  readonly testId?: string
  readonly aria?: string
  readonly role?: string
  readonly cssPath?: string
  readonly xpath?: string
  readonly text?: string
}
