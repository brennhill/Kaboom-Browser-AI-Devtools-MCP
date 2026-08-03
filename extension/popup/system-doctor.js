/**
 * Purpose: Loads and renders the daemon's canonical System Doctor report.
 * Why: Gives users actionable readiness diagnostics without duplicating health rules in the extension.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
function doctorElements() {
    return {
        host: document.getElementById('system-doctor'),
        overall: document.getElementById('system-doctor-overall'),
        checks: document.getElementById('system-doctor-checks')
    };
}
function renderMessage(label, detail, status) {
    const elements = doctorElements();
    if (!elements.overall || !elements.checks)
        return;
    elements.overall.textContent = label;
    elements.overall.className = `doctor-overall doctor-${status}`;
    const row = document.createElement('div');
    row.className = `doctor-check doctor-${status}`;
    row.textContent = detail;
    elements.checks.replaceChildren(row);
}
function renderReport(report) {
    const elements = doctorElements();
    if (!elements.overall || !elements.checks)
        return;
    const waitingForAttachment = !report.ready_for_interaction &&
        report.checks.every((check) => check.status === 'pass' || check.name === 'tracked_tab');
    const healthy = report.ready_for_interaction || waitingForAttachment;
    elements.overall.textContent = report.ready_for_interaction
        ? 'Ready'
        : waitingForAttachment
            ? 'Ready when attached'
            : 'Needs attention';
    elements.overall.className = `doctor-overall doctor-${healthy ? 'pass' : 'warn'}`;
    elements.checks.replaceChildren(...report.checks.map((check) => {
        const row = document.createElement('div');
        row.className = `doctor-check doctor-${check.status}`;
        if (check.lifecycle)
            row.dataset.lifecycle = check.lifecycle;
        const detail = document.createElement('div');
        detail.className = 'doctor-check-detail';
        detail.textContent = check.detail;
        row.appendChild(detail);
        const timelineParts = [];
        if (check.correlation_id)
            timelineParts.push(`ID: ${check.correlation_id}`);
        if (check.expected_next_transition) {
            timelineParts.push(`Next: ${check.expected_next_transition.replaceAll('_', ' ')}`);
        }
        if (check.deadline)
            timelineParts.push(`Deadline: ${new Date(check.deadline).toLocaleString()}`);
        if (check.recovery_attempt) {
            timelineParts.push(`Attempt ${check.recovery_attempt} · ${check.recovery_outcome ?? 'pending'}`);
        }
        const recentEvents = (check.history ?? [])
            .slice(-3)
            .map((transition) => transition.event?.replaceAll('_', ' '))
            .filter((event) => Boolean(event));
        if (recentEvents.length > 0)
            timelineParts.push(recentEvents.join(' → '));
        if (timelineParts.length > 0) {
            const timeline = document.createElement('div');
            timeline.className = 'doctor-check-timeline';
            timeline.textContent = timelineParts.join(' · ');
            row.appendChild(timeline);
        }
        if (check.lifecycle === 'recovered') {
            const lifecycle = document.createElement('div');
            lifecycle.className = 'doctor-check-lifecycle';
            lifecycle.textContent = check.recovered_at
                ? `Recovered ${new Date(check.recovered_at).toLocaleString()}`
                : 'Recovered';
            if ((check.occurrences ?? 0) > 1) {
                lifecycle.textContent += ` · ${check.occurrences} occurrences`;
            }
            row.appendChild(lifecycle);
        }
        if (check.fix) {
            const fix = document.createElement('div');
            fix.className = 'doctor-check-fix';
            fix.textContent = `Fix: ${check.fix}`;
            row.appendChild(fix);
        }
        return row;
    }));
}
function isDoctorReport(value) {
    if (!value || typeof value !== 'object')
        return false;
    const candidate = value;
    return (typeof candidate.ready_for_interaction === 'boolean' &&
        typeof candidate.version === 'string' &&
        Array.isArray(candidate.checks));
}
export async function refreshSystemDoctor(status, fetchImpl = fetch) {
    if (!status.connected) {
        renderMessage('Daemon offline', 'Start the daemon to run System Doctor checks.', 'warn');
        return;
    }
    try {
        const response = await fetchImpl(`${status.serverUrl ?? 'http://127.0.0.1:7890'}/doctor`);
        if (!response.ok) {
            renderMessage('Check failed', `System Doctor returned HTTP ${response.status}. Retry after restarting the daemon.`, 'fail');
            return;
        }
        const report = await response.json();
        if (!isDoctorReport(report)) {
            renderMessage('Check failed', 'System Doctor returned an invalid report. Update the daemon and extension.', 'fail');
            return;
        }
        renderReport(report);
    }
    catch {
        renderMessage('Check failed', 'System Doctor could not reach the daemon. Retry after restarting it.', 'fail');
    }
}
//# sourceMappingURL=system-doctor.js.map