/**
 * Purpose: Pure stateless serialization utilities -- safe value serialization with circular reference detection, DOM element selector generation, and sensitive input detection.
 */

/**
 * @fileoverview Serialization utilities for safe value handling.
 * Provides safe serialization with circular reference detection, DOM element
 * selector generation, and sensitive input detection.
 *
 * NOTE: This module has NO mutable state. All functions are pure and stateless.
 * No resetForTesting() function is needed.
 */

import { MAX_STRING_LENGTH, MAX_DEPTH, SENSITIVE_INPUT_TYPES } from '../constants.js'
import type { JsonValue } from '../../types/index.js'

// Extended input element interface for type checking
interface InputLikeElement extends Element {
  type?: string
  autocomplete?: string
  name?: string
  value?: string
}

// DOM element with extended properties
interface DOMElementLike {
  nodeType?: number
  tagName?: string
  id?: string
  className?: string | SVGAnimatedString
  getAttribute?(name: string): string | null
  textContent?: string | null
}

function serializePrimitive(value: unknown, type: string): JsonValue | undefined {
  if (type === 'string') {
    const s = value as string
    return s.length > MAX_STRING_LENGTH ? s.slice(0, MAX_STRING_LENGTH) + '... [truncated]' : s
  }
  if (type === 'number') return value as number
  if (type === 'boolean') return value as boolean
  if (type === 'function') return `[Function: ${(value as { name?: string }).name || 'anonymous'}]` // nosemgrep: missing-template-string-indicator
  return undefined
}

function serializeDOMNode(value: DOMElementLike): string {
  const tag = value.tagName ? value.tagName.toLowerCase() : 'node'
  const id = value.id ? `#${value.id}` : ''
  const cn = value.className
  const className = typeof cn === 'string' && cn ? `.${cn.split(' ').join('.')}` : ''
  return `[${tag}${id}${className}]`
}

function serializeObject(value: object, depth: number, seen: WeakSet<object>): JsonValue {
  if (seen.has(value)) return '[Circular]'
  seen.add(value)

  if ((value as DOMElementLike).nodeType) return serializeDOMNode(value as DOMElementLike)
  if (Array.isArray(value)) return value.slice(0, 100).map((item) => safeSerialize(item, depth + 1, seen))

  const result: Record<string, JsonValue> = {}
  for (const key of Object.keys(value).slice(0, 50)) {
    try {
      result[key] = safeSerialize((value as Record<string, unknown>)[key], depth + 1, seen)
    } catch {
      result[key] = '[Unserializable]'
    }
  }
  return result
}

/**
 * Safely serialize a value, handling circular references and special types
 */
export function safeSerialize(value: unknown, depth = 0, seen = new WeakSet<object>()): JsonValue {
  if (value === null || value === undefined) return null

  const type = typeof value
  const primitive = serializePrimitive(value, type)
  if (primitive !== undefined) return primitive

  if (value instanceof Error) {
    return { name: value.name, message: value.message, stack: value.stack || null }
  }
  if (depth >= MAX_DEPTH) return '[Max depth exceeded]'
  if (type === 'object') return serializeObject(value as object, depth, seen)

  return String(value)
}

/**
 * Get element selector for identification
 */
export function getElementSelector(element: Element | null): string {
  if (!element || !element.tagName) return ''

  const tag = element.tagName.toLowerCase()
  const id = element.id ? `#${element.id}` : ''

  let classes = ''
  const classNameValue = element.className
  if (classNameValue && typeof classNameValue === 'string') {
    classes = '.' + classNameValue.trim().split(/\s+/).slice(0, 2).join('.')
  }

  // Add data-testid if present
  const testId = element.getAttribute('data-testid')
  const testIdStr = testId ? `[data-testid="${testId}"]` : ''

  return `${tag}${id}${classes}${testIdStr}`.slice(0, 100)
}

/**
 * Check if an input contains sensitive data
 */
const SENSITIVE_AUTOCOMPLETE_PATTERNS = ['password', 'cc-', 'credit-card']
const SENSITIVE_NAME_PATTERNS = ['password', 'passwd', 'secret', 'token', 'credit', 'card', 'cvv', 'cvc', 'ssn']

function matchesAny(value: string, patterns: string[]): boolean {
  return patterns.some((p) => value.includes(p))
}

export function isSensitiveInput(element: Element | null): boolean {
  if (!element) return false

  const inputElement = element as InputLikeElement
  const type = (inputElement.type || '').toLowerCase()
  const autocomplete = (inputElement.autocomplete || '').toLowerCase()
  const name = (inputElement.name || '').toLowerCase()

  return (
    SENSITIVE_INPUT_TYPES.includes(type) ||
    matchesAny(autocomplete, SENSITIVE_AUTOCOMPLETE_PATTERNS) ||
    matchesAny(name, SENSITIVE_NAME_PATTERNS)
  )
}
