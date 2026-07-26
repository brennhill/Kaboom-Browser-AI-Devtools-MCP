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

/**
 * Update the connection status display
 */
// #lizard forgives
export function updateConnectionStatus(status: PopupConnectionStatus): void {
  const statusEl = document.getElementById('status')
  const entriesEl = document.getElementById('entries-count')
  const errorEl = document.getElementById('error-message')
  const serverUrlEl = document.getElementById('server-url')
  const logFileEl = document.getElementById('log-file-path')
  const errorCountEl = document.getElementById('error-count')
  const troubleshootingEl = document.getElementById('troubleshooting')

  if (status.connected) {
    if (statusEl) {
      statusEl.textContent = 'Connected'
      statusEl.classList.remove('disconnected')
      statusEl.classList.add('connected')
    }

    const entries = status.entries || 0
    const maxEntries = status.maxEntries || DEFAULT_MAX_ENTRIES
    if (entriesEl) {
      entriesEl.textContent = `${entries} / ${maxEntries}`
    }

    if (errorEl) {
      errorEl.textContent = ''
    }
    if (troubleshootingEl) {
      troubleshootingEl.style.display = 'none'
    }
  } else {
    if (statusEl) {
      statusEl.textContent = 'Offline'
      statusEl.classList.remove('connected')
      statusEl.classList.add('disconnected')
    }

    if (errorEl && status.error) {
      errorEl.textContent = status.error
    }
    if (troubleshootingEl) {
      troubleshootingEl.style.display = 'block'
    }
  }

  // Version mismatch warning
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

  const securityWarningEl = document.getElementById('security-mode-warning')
  const securityDetailEl = document.getElementById('security-mode-detail')
  if (securityWarningEl) {
    if (status.securityMode === 'insecure_proxy') {
      securityWarningEl.style.display = 'block'
      if (securityDetailEl) {
        const rewrites =
          status.insecureRewritesApplied && status.insecureRewritesApplied.length > 0
            ? status.insecureRewritesApplied.join(', ')
            : 'csp_headers'
        securityDetailEl.textContent = `INSECURE DEBUG MODE active. production_parity=${status.productionParity === false ? 'false' : 'true'}; rewrites=${rewrites}`
      }
    } else {
      securityWarningEl.style.display = 'none'
      if (securityDetailEl) {
        securityDetailEl.textContent = ''
      }
    }
  }

  if (serverUrlEl && status.serverUrl) {
    serverUrlEl.textContent = status.serverUrl
  }

  if (logFileEl && status.logFile) {
    logFileEl.textContent = status.logFile
  }

  if (errorCountEl && status.errorCount !== undefined) {
    errorCountEl.textContent = String(status.errorCount)
  }

  // Log file size
  const fileSizeEl = document.getElementById('log-file-size')
  if (fileSizeEl && status.logFileSize !== undefined) {
    fileSizeEl.textContent = formatFileSize(status.logFileSize)
  }

  // Health indicators (circuit breaker + memory pressure)
  const healthSection = document.getElementById('health-indicators')
  const cbEl = document.getElementById('health-circuit-breaker')
  const mpEl = document.getElementById('health-memory-pressure')

  if (healthSection && cbEl && mpEl) {
    const cbState = status.circuitBreakerState || 'closed'
    const mpState = status.memoryPressure?.memoryPressureLevel || 'normal'

    // Circuit breaker indicator
    cbEl.classList.remove('health-error', 'health-warning')
    if (!status.connected || cbState === 'closed') {
      cbEl.style.display = 'none'
      cbEl.textContent = ''
    } else if (cbState === 'open') {
      cbEl.style.display = ''
      cbEl.classList.add('health-error')
      cbEl.textContent = 'Server: paused (recovering from errors)'
    } else if (cbState === 'half-open') {
      cbEl.style.display = ''
      cbEl.classList.add('health-warning')
      cbEl.textContent = 'Server: recovering'
    }

    // Memory pressure indicator
    mpEl.classList.remove('health-error', 'health-warning')
    if (!status.connected || mpState === 'normal') {
      mpEl.style.display = 'none'
      mpEl.textContent = ''
    } else if (mpState === 'soft') {
      mpEl.style.display = ''
      mpEl.classList.add('health-warning')
      mpEl.textContent = 'Memory: elevated (some features limited)'
    } else if (mpState === 'hard') {
      mpEl.style.display = ''
      mpEl.classList.add('health-error')
      mpEl.textContent = 'Memory: critical (network capture disabled)'
    }

    // Show/hide entire section
    const cbVisible = status.connected && cbState !== 'closed'
    const mpVisible = status.connected && mpState !== 'normal'
    healthSection.style.display = cbVisible || mpVisible ? '' : 'none'
  }

  // Context annotation warning
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
