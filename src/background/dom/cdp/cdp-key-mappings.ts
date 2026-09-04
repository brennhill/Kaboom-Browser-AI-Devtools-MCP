/**
 * Purpose: Key code mappings and character-to-key resolution for CDP Input.dispatchKeyEvent.
 * Why: Separates keyboard layout data from CDP protocol dispatch logic for maintainability.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// Key code mappings for CDP Input.dispatchKeyEvent
export const KEY_CODES: Record<string, { code: string; keyCode: number }> = {
  Enter: { code: 'Enter', keyCode: 13 },
  Tab: { code: 'Tab', keyCode: 9 },
  Escape: { code: 'Escape', keyCode: 27 },
  Backspace: { code: 'Backspace', keyCode: 8 },
  Delete: { code: 'Delete', keyCode: 46 },
  ArrowUp: { code: 'ArrowUp', keyCode: 38 },
  ArrowDown: { code: 'ArrowDown', keyCode: 40 },
  ArrowLeft: { code: 'ArrowLeft', keyCode: 37 },
  ArrowRight: { code: 'ArrowRight', keyCode: 39 },
  Space: { code: 'Space', keyCode: 32 },
  Home: { code: 'Home', keyCode: 36 },
  End: { code: 'End', keyCode: 35 },
  PageUp: { code: 'PageUp', keyCode: 33 },
  PageDown: { code: 'PageDown', keyCode: 34 }
}

// Characters that require shift on a US keyboard
const SHIFT_CHARS = '~!@#$%^&*()_+{}|:"<>?ABCDEFGHIJKLMNOPQRSTUVWXYZ'

export function charToKeyInfo(char: string): { key: string; code: string; keyCode: number; shiftKey: boolean } {
  const shiftKey = SHIFT_CHARS.includes(char)
  const lower = char.toLowerCase()

  // Letter keys
  if (lower >= 'a' && lower <= 'z') {
    return {
      key: char,
      code: `Key${lower.toUpperCase()}`,
      keyCode: lower.charCodeAt(0) - 32, // A=65
      shiftKey
    }
  }

  // Digit keys
  if (char >= '0' && char <= '9') {
    return {
      key: char,
      code: `Digit${char}`,
      keyCode: char.charCodeAt(0),
      shiftKey: false
    }
  }

  // Space
  if (char === ' ') {
    return { key: ' ', code: 'Space', keyCode: 32, shiftKey: false }
  }

  // Common punctuation — approximate key codes
  const punctuation: Record<string, { code: string; keyCode: number }> = {
    '-': { code: 'Minus', keyCode: 189 },
    '=': { code: 'Equal', keyCode: 187 },
    '[': { code: 'BracketLeft', keyCode: 219 },
    ']': { code: 'BracketRight', keyCode: 221 },
    '\\': { code: 'Backslash', keyCode: 220 },
    ';': { code: 'Semicolon', keyCode: 186 },
    "'": { code: 'Quote', keyCode: 222 },
    ',': { code: 'Comma', keyCode: 188 },
    '.': { code: 'Period', keyCode: 190 },
    '/': { code: 'Slash', keyCode: 191 },
    '`': { code: 'Backquote', keyCode: 192 },
    // Shifted variants
    _: { code: 'Minus', keyCode: 189 },
    '+': { code: 'Equal', keyCode: 187 },
    '{': { code: 'BracketLeft', keyCode: 219 },
    '}': { code: 'BracketRight', keyCode: 221 },
    '|': { code: 'Backslash', keyCode: 220 },
    ':': { code: 'Semicolon', keyCode: 186 },
    '"': { code: 'Quote', keyCode: 222 },
    '<': { code: 'Comma', keyCode: 188 },
    '>': { code: 'Period', keyCode: 190 },
    '?': { code: 'Slash', keyCode: 191 },
    '~': { code: 'Backquote', keyCode: 192 },
    '!': { code: 'Digit1', keyCode: 49 },
    '@': { code: 'Digit2', keyCode: 50 },
    '#': { code: 'Digit3', keyCode: 51 },
    $: { code: 'Digit4', keyCode: 52 },
    '%': { code: 'Digit5', keyCode: 53 },
    '^': { code: 'Digit6', keyCode: 54 },
    '&': { code: 'Digit7', keyCode: 55 },
    '*': { code: 'Digit8', keyCode: 56 },
    '(': { code: 'Digit9', keyCode: 57 },
    ')': { code: 'Digit0', keyCode: 48 }
  }

  const punct = punctuation[char]
  if (punct) {
    return { key: char, ...punct, shiftKey }
  }

  // Fallback for other characters
  return { key: char, code: '', keyCode: 0, shiftKey: false }
}

/**
 * CDP `Input.dispatch*Event` modifier bitmask. Chrome defines these bits, not us:
 * Alt=1, Ctrl=2, Meta/Command=4, Shift=8. A wrong bit silently produces a plain
 * click — the page never sees the ctrl/shift the agent asked for.
 */
export const MODIFIER_BITS: Record<string, number> = {
  alt: 1,
  ctrl: 2,
  control: 2,
  meta: 4,
  cmd: 4,
  command: 4,
  shift: 8
}

/**
 * Fold modifier names into the CDP bitmask. Unknown names are ignored rather than
 * rejected: an agent asking for a modifier Chrome has no bit for still gets its click.
 */
export function modifierBitmask(modifiers?: readonly string[]): number {
  if (!modifiers || modifiers.length === 0) return 0
  let mask = 0
  for (const name of modifiers) {
    mask |= MODIFIER_BITS[String(name).trim().toLowerCase()] ?? 0
  }
  return mask
}
