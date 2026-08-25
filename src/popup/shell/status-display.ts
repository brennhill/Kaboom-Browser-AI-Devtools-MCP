/**
 * Purpose: Renders popup connection, health, and warning indicators from background status payloads.
 * Why: Converts raw runtime status into operator-readable diagnostics during extension/server troubleshooting.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

/**
 * @fileoverview Status Display Module
 * Updates connection status display in popup
 */

import type { PopupConnectionStatus } from './types.js'
import { formatFileSize } from './ui-utils.js'

const DEFAULT_MAX_ENTRIES = 1000

interface StatusElements {
  statusEl: HTMLElement | null
  entriesEl: HTMLElement | null
  errorEl: HTMLElement | null
  serverUrlEl: HTMLElement | null
  logFileEl: HTMLElement | null
  errorCountEl: HTMLElement | null
  troubleshootingEl: HTMLElement | null
}

function getStatusElements(): StatusElements {
  return {
    statusEl: document.getElementById('status'),
    entriesEl: document.getElementById('entries-count'),
    errorEl: document.getElementById('error-message'),
    serverUrlEl: document.getElementById('server-url'),
    logFileEl: document.getElementById('log-file-path'),
    errorCountEl: document.getElementById('error-count'),
    troubleshootingEl: document.getElementById('troubleshooting')
  }
}

function setConnectionBadge(statusEl: HTMLElement | null, connected: boolean): void {
  if (!statusEl) return
  statusEl.textContent = connected ? 'Connected' : 'Offline'
  statusEl.classList.remove(connected ? 'disconnected' : 'connected')
  statusEl.classList.add(connected ? 'connected' : 'disconnected')
}

function renderConnectionPanel(status: PopupConnectionStatus, els: StatusElements): void {
  setConnectionBadge(els.statusEl, status.connected)

  if (status.connected) {
    const entries = status.entries || 0
    const maxEntries = status.maxEntries || DEFAULT_MAX_ENTRIES
    if (els.entriesEl) {
      els.entriesEl.textContent = `${entries} / ${maxEntries}`
    }

    if (els.errorEl) {
      els.errorEl.textContent = ''
    }
    if (els.troubleshootingEl) {
      els.troubleshootingEl.style.display = 'none'
    }
  } else {
    if (els.errorEl && status.error) {
      els.errorEl.textContent = status.error
    }
    if (els.troubleshootingEl) {
      els.troubleshootingEl.style.display = 'block'
    }
  }
}

function renderVersionWarning(status: PopupConnectionStatus): void {
  const versionWarningEl = document.getElementById('version-mismatch')
  if (versionWarningEl) {
    if (status.versionMismatch && status.serverVersion && status.extensionVersion) {
      versionWarningEl.style.display = 'block'
      const versionDetail = versionWarningEl.querySelector('.version-detail')
      if (versionDetail) {
        versionDetail.textContent = `Server: v${status.serverVersion} / Extension: v${status.extensionVersion}`
      }
    } else {
      versionWarningEl.style.display = 'none'
    }
  }
}

function securityDetailText(status: PopupConnectionStatus): string {
  const rewrites =
    status.insecureRewritesApplied && status.insecureRewritesApplied.length > 0
      ? status.insecureRewritesApplied.join(', ')
      : 'csp_headers'
  return `INSECURE DEBUG MODE active. production_parity=${status.productionParity === false ? 'false' : 'true'}; rewrites=${rewrites}`
}

function renderSecurityWarning(status: PopupConnectionStatus): void {
  const securityWarningEl = document.getElementById('security-mode-warning')
  const securityDetailEl = document.getElementById('security-mode-detail')
  if (securityWarningEl) {
    if (status.securityMode === 'insecure_proxy') {
      securityWarningEl.style.display = 'block'
      if (securityDetailEl) {
        securityDetailEl.textContent = securityDetailText(status)
      }
    } else {
      securityWarningEl.style.display = 'none'
      if (securityDetailEl) {
        securityDetailEl.textContent = ''
      }
    }
  }
}

function renderOptionalText(el: HTMLElement | null, value: string | undefined): void {
  if (el && value) {
    el.textContent = value
  }
}

function renderInfoFields(status: PopupConnectionStatus, els: StatusElements): void {
  renderOptionalText(els.serverUrlEl, status.serverUrl)
  renderOptionalText(els.logFileEl, status.logFile)

  if (els.errorCountEl && status.errorCount !== undefined) {
    els.errorCountEl.textContent = String(status.errorCount)
  }

  // Log file size
  const fileSizeEl = document.getElementById('log-file-size')
  if (fileSizeEl && status.logFileSize !== undefined) {
    fileSizeEl.textContent = formatFileSize(status.logFileSize)
  }
}

type HealthLevel = 'ok' | 'warn' | 'error' | 'unknown'

function circuitBreakerLevel(state: string): HealthLevel {
  if (state === 'closed') return 'ok'
  if (state === 'open') return 'error'
  if (state === 'half-open') return 'warn'
  return 'unknown'
}

function memoryPressureLevel(state: string): HealthLevel {
  if (state === 'normal') return 'ok'
  if (state === 'soft') return 'warn'
  if (state === 'hard') return 'error'
  return 'unknown'
}

function renderHealthIndicator(
  el: HTMLElement,
  connected: boolean,
  level: HealthLevel,
  warnText: string,
  errorText: string
): void {
  el.classList.remove('health-error', 'health-warning')
  if (!connected || level === 'ok') {
    el.style.display = 'none'
    el.textContent = ''
    return
  }
  if (level === 'error') {
    el.style.display = ''
    el.classList.add('health-error')
    el.textContent = errorText
    return
  }
  if (level === 'warn') {
    el.style.display = ''
    el.classList.add('health-warning')
    el.textContent = warnText
  }
}

function renderHealthIndicators(status: PopupConnectionStatus): void {
  const healthSection = document.getElementById('health-indicators')
  const cbEl = document.getElementById('health-circuit-breaker')
  const mpEl = document.getElementById('health-memory-pressure')

  if (healthSection && cbEl && mpEl) {
    const cbState = status.circuitBreakerState || 'closed'
    const mpState = status.memoryPressure?.memoryPressureLevel || 'normal'

    renderHealthIndicator(
      cbEl,
      status.connected,
      circuitBreakerLevel(cbState),
      'Server: recovering',
      'Server: paused (recovering from errors)'
    )

    renderHealthIndicator(
      mpEl,
      status.connected,
      memoryPressureLevel(mpState),
      'Memory: elevated (some features limited)',
      'Memory: critical (network capture disabled)'
    )

    // Show/hide entire section
    const cbVisible = status.connected && cbState !== 'closed'
    const mpVisible = status.connected && mpState !== 'normal'
    healthSection.style.display = cbVisible || mpVisible ? '' : 'none'
  }
}

function renderContextWarning(status: PopupConnectionStatus): void {
  const contextWarningEl = document.getElementById('context-warning')
  const contextWarningTextEl = document.getElementById('context-warning-text')
  if (contextWarningEl) {
    if (status.connected && status.contextWarning) {
      contextWarningEl.style.display = 'block'
      if (contextWarningTextEl) {
        contextWarningTextEl.textContent = `${status.contextWarning.count} recent entries have context annotations averaging ${status.contextWarning.sizeKB}KB. This may consume significant AI context window space.`
      }
    } else {
      contextWarningEl.style.display = 'none'
      if (contextWarningTextEl) {
        contextWarningTextEl.textContent = ''
      }
    }
  }
}

/**
 * Update the connection status display
 */
export function updateConnectionStatus(status: PopupConnectionStatus): void {
  const els = getStatusElements()

  renderConnectionPanel(status, els)
  renderVersionWarning(status)
  renderSecurityWarning(status)
  renderInfoFields(status, els)
  renderHealthIndicators(status)
  renderContextWarning(status)
}
