/**
 * Purpose: Validate persisted extension state and apply explicit safe fallbacks.
 */
import type { StateRecoveryDiagnostic } from '../../types/runtime-messages.js'
import { getLocal } from './local.js'
import {
  reportStateRecovery as reportAcrossContexts,
  resolveStateRecovery as resolveAcrossContexts
} from './recovery.js'
import { getSession } from './session.js'

type Reporter = (diagnostic: StateRecoveryDiagnostic) => void
type Resolver = (name: string) => void
type Validator<T> = (value: unknown) => value is T

interface ReadStateOptions<T> {
  key: string
  fallback: T
  validate: Validator<T>
  diagnostic: StateRecoveryDiagnostic
  report?: Reporter
  resolve?: Resolver
}

async function readValidated<T>(
  read: (key: string) => Promise<unknown>,
  options: ReadStateOptions<T>
): Promise<T> {
  const report = options.report ?? reportAcrossContexts
  const resolve = options.resolve ?? resolveAcrossContexts
  try {
    const value = await read(options.key)
    if (value === undefined || value === null) return options.fallback
    if (options.validate(value)) {
      resolve(options.diagnostic.name)
      return value
    }
    report(options.diagnostic)
    return options.fallback
  } catch {
    report(options.diagnostic)
    return options.fallback
  }
}

export function readLocalState<T>(options: ReadStateOptions<T>): Promise<T> {
  return readValidated(getLocal, options)
}

export function readSessionState<T>(options: ReadStateOptions<T>): Promise<T> {
  return readValidated(getSession, options)
}
