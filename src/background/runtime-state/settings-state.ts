/**
 * Purpose: Own locally cached background settings and server configuration.
 * Why: Settings are loaded and mutated as one lifecycle, separate from connection health.
 */
import { DEFAULT_SERVER_URL } from '../../lib/constants.js'

let serverUrl = DEFAULT_SERVER_URL
let debugMode = false
let currentLogLevel = 'all'
let screenshotOnError = false
let captureOverrides: Readonly<Record<string, string>> = Object.freeze({})

export function getServerUrl(): string {
  return serverUrl
}
export function setServerUrl(url: string): void {
  serverUrl = url
}
export function isDebugMode(): boolean {
  return debugMode
}
export function setDebugModeRaw(enabled: boolean): void {
  debugMode = enabled
}
export function getCurrentLogLevel(): string {
  return currentLogLevel
}
export function setCurrentLogLevel(level: string): void {
  currentLogLevel = level
}
export function isScreenshotOnError(): boolean {
  return screenshotOnError
}
export function setScreenshotOnError(enabled: boolean): void {
  screenshotOnError = enabled
}
export function isAiControlled(): boolean {
  return Object.keys(captureOverrides).length > 0
}

export function applySettingOverrides(overrides: Readonly<Record<string, string>>): void {
  captureOverrides = Object.freeze({ ...overrides })
  if (overrides.log_level !== undefined) setCurrentLogLevel(overrides.log_level)
  if (overrides.screenshot_on_error !== undefined) setScreenshotOnError(overrides.screenshot_on_error === 'true')
}
