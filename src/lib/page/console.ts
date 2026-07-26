/**
 * Purpose: Monkey-patches console.log/warn/error/info/debug to capture messages and forward them via the bridge while preserving original behavior.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Console method capture.
 * Monkey-patches console.log/warn/error/info/debug to capture messages
 * and forward them via postLog, while preserving original behavior.
 */

import { safeSerialize } from './serialize.js'
import { postLog } from './bridge.js'

type ConsoleMethods = 'log' | 'warn' | 'error' | 'info' | 'debug'

// Store original methods
let originalConsole: Partial<Record<ConsoleMethods, (...args: unknown[]) => void>> = {}

/**
 * Install console capture hooks
 */
export function installConsoleCapture(): void {
  const methods: ConsoleMethods[] = ['log', 'warn', 'error', 'info', 'debug']

  methods.forEach((method) => {
    // eslint-disable-next-line security/detect-object-injection -- method from known-safe local array of console methods
    originalConsole[method] = console[method]

    // eslint-disable-next-line security/detect-object-injection -- method from known-safe local array of console methods
    console[method] = function (...args: unknown[]): void {
      // Post to extension
      postLog({
        level: method,
        type: 'console',
        args: args.map((arg) => safeSerialize(arg))
      })

      // Call original
      // eslint-disable-next-line security/detect-object-injection -- method from known-safe local array of console methods
      originalConsole[method]!.apply(console, args)
    }
  })
}

/**
 * Uninstall console capture hooks
 */
export function uninstallConsoleCapture(): void {
  Object.keys(originalConsole).forEach((method) => {
    // eslint-disable-next-line security/detect-object-injection -- method from Object.keys of our own originalConsole storage
    console[method as ConsoleMethods] = originalConsole[method as ConsoleMethods]!
  })
  originalConsole = {}
}
