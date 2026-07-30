/**
 * Purpose: Loads and renders the daemon's canonical System Doctor report.
 * Why: Gives users actionable readiness diagnostics without duplicating health rules in the extension.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

import type { PopupConnectionStatus } from './shell/types.js'

type DoctorStatus = 'pass' | 'warn' | 'fail'
type DoctorLifecycle = 'active' | 'recovered'

interface DoctorTransition {
  lifecycle: DoctorLifecycle
  at: string
}

interface DoctorCheck {
  name: string
  status: DoctorStatus
  detail: string
  fix?: string
  lifecycle?: DoctorLifecycle
  first_seen_at?: string
  last_seen_at?: string
  recovered_at?: string
  occurrences?: number
  history?: DoctorTransition[]
}

interface DoctorReport {
  status: 'healthy' | 'degraded' | 'unhealthy'
  ready_for_interaction: boolean
  version: string
  checks: DoctorCheck[]
}

type DoctorFetch = (input: string) => Promise<{ ok: boolean; status: number; json: () => Promise<unknown> }>

function doctorElements(): {
  host: HTMLElement | null
  overall: HTMLElement | null
  checks: HTMLElement | null
} {
  return {
    host: document.getElementById('system-doctor'),
    overall: document.getElementById('system-doctor-overall'),
    checks: document.getElementById('system-doctor-checks')
  }
}

function renderMessage(label: string, detail: string, status: 'pass' | 'warn' | 'fail'): void {
  const elements = doctorElements()
  if (!elements.overall || !elements.checks) return
  elements.overall.textContent = label
  elements.overall.className = `doctor-overall doctor-${status}`
  const row = document.createElement('div')
  row.className = `doctor-check doctor-${status}`
  row.textContent = detail
  elements.checks.replaceChildren(row)
}

function renderReport(report: DoctorReport): void {
  const elements = doctorElements()
  if (!elements.overall || !elements.checks) return
  const waitingForAttachment =
    !report.ready_for_interaction &&
    report.checks.every((check) => check.status === 'pass' || check.name === 'tracked_tab')
  const healthy = report.ready_for_interaction || waitingForAttachment
  elements.overall.textContent = report.ready_for_interaction
    ? 'Ready'
    : waitingForAttachment
      ? 'Ready when attached'
      : 'Needs attention'
  elements.overall.className = `doctor-overall doctor-${healthy ? 'pass' : 'warn'}`
  elements.checks.replaceChildren(
    ...report.checks.map((check) => {
      const row = document.createElement('div')
      row.className = `doctor-check doctor-${check.status}`
      if (check.lifecycle) row.dataset.lifecycle = check.lifecycle
      const detail = document.createElement('div')
      detail.className = 'doctor-check-detail'
      detail.textContent = check.detail
      row.appendChild(detail)
      if (check.lifecycle === 'recovered') {
        const lifecycle = document.createElement('div')
        lifecycle.className = 'doctor-check-lifecycle'
        lifecycle.textContent = check.recovered_at
          ? `Recovered ${new Date(check.recovered_at).toLocaleString()}`
          : 'Recovered'
        if ((check.occurrences ?? 0) > 1) {
          lifecycle.textContent += ` · ${check.occurrences} occurrences`
        }
        row.appendChild(lifecycle)
      }
      if (check.fix) {
        const fix = document.createElement('div')
        fix.className = 'doctor-check-fix'
        fix.textContent = `Fix: ${check.fix}`
        row.appendChild(fix)
      }
      return row
    })
  )
}

function isDoctorReport(value: unknown): value is DoctorReport {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Partial<DoctorReport>
  return (
    typeof candidate.ready_for_interaction === 'boolean' &&
    typeof candidate.version === 'string' &&
    Array.isArray(candidate.checks)
  )
}

export async function refreshSystemDoctor(
  status: Pick<PopupConnectionStatus, 'connected' | 'serverUrl'>,
  fetchImpl: DoctorFetch = fetch
): Promise<void> {
  if (!status.connected) {
    renderMessage('Daemon offline', 'Start the daemon to run System Doctor checks.', 'warn')
    return
  }

  try {
    const response = await fetchImpl(`${status.serverUrl ?? 'http://127.0.0.1:7890'}/doctor`)
    if (!response.ok) {
      renderMessage(
        'Check failed',
        `System Doctor returned HTTP ${response.status}. Retry after restarting the daemon.`,
        'fail'
      )
      return
    }
    const report = await response.json()
    if (!isDoctorReport(report)) {
      renderMessage(
        'Check failed',
        'System Doctor returned an invalid report. Update the daemon and extension.',
        'fail'
      )
      return
    }
    renderReport(report)
  } catch {
    renderMessage('Check failed', 'System Doctor could not reach the daemon. Retry after restarting it.', 'fail')
  }
}
