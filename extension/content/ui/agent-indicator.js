/**
 * Purpose: Shows the human whose tab it is what the agent is about to do, and gives them a
 *          way to stop it — phantom cursor, driving indicator, stop control, heartbeat.
 * Why: Kaboom drives with trusted CDP input over a session that outlives a single action.
 *      Per-action toasts narrate what already happened; nothing said "an agent is driving
 *      this tab right now", and there was no stop control anywhere in the product.
 * Docs: docs/features/feature/agent-supervision/index.md
 */
// agent-indicator.ts — Supervision overlay: phantom cursor, driving state, stop, heartbeat.
/** Element ids. Exported so tests and the capture stripper can reason about them. */
export const AGENT_INDICATOR_IDS = {
    root: 'kaboom-agent-indicator',
    cursor: 'kaboom-phantom-cursor',
    glow: 'kaboom-driving-glow',
    pill: 'kaboom-driving-pill',
    stop: 'kaboom-driving-stop'
};
/**
 * How long the overlay survives without a heartbeat.
 *
 * MV3 terminates the service worker without warning. If the worker dies mid-action the
 * overlay must remove ITSELF, or the user is left with a permanent "an agent is driving
 * this tab" badge on a tab nothing is driving — the same staleness failure that left
 * TERMINAL_UI_STATE='open' forever (CLAUDE.md rule 18).
 */
export const HEARTBEAT_TTL_MS = 15_000;
/** Above every plausible page z-index, below nothing we own. */
export const OVERLAY_Z_INDEX = 2147483646;
/**
 * The supervision state machine, with no DOM dependency.
 *
 * Split from rendering so the parts that decide behaviour — heartbeat expiry, whether a stop
 * is honoured, whether the overlay should be visible — are testable as pure functions of the
 * clock and the inputs. This repo has no jsdom; logic that only exists inside DOM callbacks
 * cannot be tested here at all.
 */
export class AgentIndicatorCore {
    now;
    state = {
        driving: false,
        action: null,
        cursor: null,
        lastHeartbeatAt: 0
    };
    constructor(now) {
        this.now = now;
    }
    snapshot() {
        return { ...this.state, cursor: this.state.cursor ? { ...this.state.cursor } : null };
    }
    get driving() {
        return this.state.driving;
    }
    /** Begin (or relabel) driving. Idempotent: a second call updates the label only. */
    startDriving(action) {
        this.state.driving = true;
        this.state.action = action;
        this.state.lastHeartbeatAt = this.now();
    }
    /** The CDP lease was released, or the action finished. */
    stopDriving() {
        this.state.driving = false;
        this.state.action = null;
        this.state.cursor = null;
    }
    /** Move the phantom cursor. Ignored when not driving: a stray cursor implies activity. */
    moveCursor(x, y) {
        if (!this.state.driving)
            return false;
        this.state.cursor = { x, y };
        return true;
    }
    heartbeat() {
        this.state.lastHeartbeatAt = this.now();
    }
    /**
     * Clock check. Returns a teardown reason when the overlay must come down, else null.
     * Pure with respect to the injected clock, so expiry is testable without waiting.
     */
    tick() {
        if (!this.state.driving)
            return null;
        if (this.now() - this.state.lastHeartbeatAt <= HEARTBEAT_TTL_MS)
            return null;
        this.stopDriving();
        return 'heartbeat_expired';
    }
}
/**
 * Decide whether a stop interaction is honoured.
 *
 * Gated on `event.isTrusted`. A page can dispatch a synthetic click on any element in its
 * own document, so without this a hostile page could abort the agent at will — or, worse,
 * silently suppress the stop control's meaning by firing it constantly. Only a real user
 * gesture carries isTrusted:true.
 */
export function isHonouredStop(event) {
    return event?.isTrusted === true;
}
/** Label shown in the pill. Kept here so wording and truncation stay consistent (rule 21). */
export function drivingLabel(action) {
    const trimmed = (action ?? '').trim();
    if (!trimmed)
        return 'Kaboom is driving this tab';
    const cleaned = trimmed.replace(/_/g, ' ');
    const shown = cleaned.length > 40 ? cleaned.slice(0, 39) + '…' : cleaned;
    return `Kaboom is driving this tab — ${shown}`;
}
// =============================================================================
// RENDERING
// =============================================================================
/** Palette. Kaboom's accent, deliberately distinct from any page chrome. */
const ACCENT = '#f97316';
const ROOT_CSS = `
  :host { all: initial; }
  .glow {
    position: fixed; inset: 0; pointer-events: none;
    box-shadow: inset 0 0 0 3px ${ACCENT}, inset 0 0 22px rgba(249,115,22,0.35);
    opacity: 0; transition: opacity 160ms ease;
  }
  .glow.on { opacity: 1; }
  .pill {
    position: fixed; bottom: 18px; left: 50%; transform: translateX(-50%);
    display: none; align-items: center; gap: 10px;
    padding: 8px 10px 8px 14px; border-radius: 999px;
    background: rgba(17,17,17,0.92); color: #fff;
    font: 600 13px/1.2 system-ui, -apple-system, "Segoe UI", sans-serif;
    box-shadow: 0 6px 24px rgba(0,0,0,0.35);
  }
  .pill.on { display: flex; }
  .dot {
    width: 8px; height: 8px; border-radius: 50%; background: ${ACCENT};
    animation: kb-pulse 1.6s ease-in-out infinite;
  }
  @keyframes kb-pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.35; } }
  .stop {
    all: unset; cursor: pointer; pointer-events: auto;
    padding: 4px 12px; border-radius: 999px;
    background: #fff; color: #111;
    font: 700 12px/1.2 system-ui, -apple-system, "Segoe UI", sans-serif;
  }
  .stop:hover { background: #f0eee6; }
  .cursor {
    position: fixed; width: 22px; height: 22px; margin: -3px 0 0 -3px;
    display: none; pointer-events: none;
    transition: transform 120ms cubic-bezier(0.22, 1, 0.36, 1);
  }
  .cursor.on { display: block; }
`;
/** Arrow pointer whose TIP is at (0,0) so it lands on the coordinate CDP will click. */
const CURSOR_SVG = `<svg viewBox="0 0 22 22" width="22" height="22" aria-hidden="true">` +
    `<path d="M1 1 L1 16 L5.5 12 L8.5 19 L11.5 17.5 L8.5 11 L14 11 Z" ` +
    `fill="${ACCENT}" stroke="#fff" stroke-width="1.4" stroke-linejoin="round"/></svg>`;
/**
 * The supervision overlay as mounted in a page.
 *
 * The root carries `data-kaboom-overlay` so screenshot capture strips it — see
 * setKaboomOverlayVisibility. Do not rely on the id: an id list is exactly the mechanism
 * that silently stopped stripping the draw overlay.
 */
export class AgentIndicator {
    deps;
    rendered = null;
    core;
    constructor(deps) {
        this.deps = deps;
        this.core = new AgentIndicatorCore(deps.now);
    }
    get mounted() {
        return this.rendered !== null;
    }
    get driving() {
        return this.core.driving;
    }
    /** Create the overlay if absent. Idempotent, so any entry point may call it. */
    mount() {
        if (this.rendered || typeof document === 'undefined' || !document.body)
            return;
        const host = document.createElement('div');
        host.id = AGENT_INDICATOR_IDS.root;
        // The marker the capture stripper selects on. Without it this overlay would appear in
        // every screenshot and the agent would read its own UI as page content.
        host.setAttribute('data-kaboom-overlay', 'agent-indicator');
        host.style.cssText = `position:fixed;inset:0;pointer-events:none;z-index:${OVERLAY_Z_INDEX};`;
        const shadow = host.attachShadow({ mode: 'open' });
        const style = document.createElement('style');
        style.textContent = ROOT_CSS;
        shadow.appendChild(style);
        const glow = document.createElement('div');
        glow.className = 'glow';
        glow.id = AGENT_INDICATOR_IDS.glow;
        const pill = document.createElement('div');
        pill.className = 'pill';
        pill.id = AGENT_INDICATOR_IDS.pill;
        const dot = document.createElement('span');
        dot.className = 'dot';
        const label = document.createElement('span');
        const stop = document.createElement('button');
        stop.className = 'stop';
        stop.id = AGENT_INDICATOR_IDS.stop;
        stop.textContent = 'Stop';
        stop.setAttribute('type', 'button');
        stop.addEventListener('click', (event) => {
            // Only a real user gesture stops the agent. A page can synthesise a click on any
            // element in its own document.
            if (!isHonouredStop(event))
                return;
            this.stopDriving();
            this.deps.onStop();
        });
        pill.appendChild(dot);
        pill.appendChild(label);
        pill.appendChild(stop);
        const cursor = document.createElement('div');
        cursor.className = 'cursor';
        cursor.id = AGENT_INDICATOR_IDS.cursor;
        cursor.innerHTML = CURSOR_SVG;
        shadow.appendChild(glow);
        shadow.appendChild(pill);
        shadow.appendChild(cursor);
        document.body.appendChild(host);
        this.rendered = { host, glow, pill, label, stop, cursor };
        this.paint();
    }
    unmount() {
        this.core.stopDriving();
        this.rendered?.host.remove();
        this.rendered = null;
    }
    startDriving(action) {
        this.core.startDriving(action);
        this.mount();
        this.paint();
    }
    stopDriving() {
        this.core.stopDriving();
        this.paint();
    }
    moveCursor(x, y) {
        if (!this.core.moveCursor(x, y))
            return;
        this.paint();
    }
    heartbeat() {
        this.core.heartbeat();
    }
    /** Drive from a timer. Removes the overlay when its worker has stopped heartbeating. */
    tick() {
        const reason = this.core.tick();
        if (reason)
            this.unmount();
        return reason;
    }
    paint() {
        const r = this.rendered;
        if (!r)
            return;
        const state = this.core.snapshot();
        r.glow.classList.toggle('on', state.driving);
        r.pill.classList.toggle('on', state.driving);
        r.label.textContent = drivingLabel(state.action);
        if (state.cursor) {
            r.cursor.classList.add('on');
            r.cursor.style.transform = `translate(${state.cursor.x}px, ${state.cursor.y}px)`;
        }
        else {
            r.cursor.classList.remove('on');
        }
    }
}
//# sourceMappingURL=agent-indicator.js.map