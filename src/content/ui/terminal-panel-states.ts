/**
 * Purpose: The terminal panel's non-terminal states — no session, and start failure.
 * Why: Both are recoverable states with actions in them, not error text, and
 * keeping them out of sidepanel.ts keeps that file within the size limit.
 * Docs: docs/features/feature/terminal/index.md
 */

import { START_TERMINAL_BUTTON_ID } from './terminal-widget-types.js'

/**
 * Render a recoverable "no shell" state into `container`.
 *
 * The panel used to print a dead sentence and stop, so an ended or failed
 * session left a panel with nothing to click — the user could neither retry nor
 * change anything without digging through the options page. The root-folder bar
 * above the terminal covers the other half; this covers starting one.
 */
export function renderNoSessionState(container: HTMLElement, onStart: () => void): void {
  container.replaceChildren()

  const wrap = document.createElement('div')
  Object.assign(wrap.style, {
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
    padding: '16px',
    color: '#a9b1d6',
    fontSize: '12px'
  })

  const msg = document.createElement('div')
  msg.textContent = 'No terminal session. Start one below, or check that the KaBOOM! daemon is running.'
  msg.style.color = '#c0caf5'

  const startButton = document.createElement('button')
  startButton.id = START_TERMINAL_BUTTON_ID
  startButton.textContent = 'Start terminal'
  startButton.type = 'button'
  Object.assign(startButton.style, {
    padding: '8px 12px',
    borderRadius: '8px',
    border: '1px solid #7aa2f7',
    background: '#1a1b26',
    color: '#7aa2f7',
    cursor: 'pointer',
    fontSize: '12px',
    alignSelf: 'flex-start'
  })
  startButton.addEventListener('click', (event: MouseEvent) => {
    event.preventDefault()
    event.stopPropagation()
    onStart()
  })

  wrap.appendChild(msg)
  wrap.appendChild(startButton)
  container.appendChild(wrap)
}

/**
 * Render a start failure: what happened, what to do, and the command to run.
 */
export function renderStartFailure(
  container: HTMLElement,
  message: string,
  instruction: string,
  command: string
): void {
  container.replaceChildren()

  const overlay = document.createElement('div')
  Object.assign(overlay.style, {
    display: 'flex',
    flexDirection: 'column',
    gap: '10px',
    padding: '16px',
    borderRadius: '12px',
    background: '#1a1b26',
    border: '1px solid #f7768e',
    color: '#a9b1d6',
    margin: '16px'
  })

  const title = document.createElement('div')
  title.textContent = 'Terminal unavailable'
  Object.assign(title.style, { color: '#f7768e', fontWeight: '600', fontSize: '14px' })

  const msg = document.createElement('div')
  msg.textContent = message
  Object.assign(msg.style, { fontSize: '12px', color: '#787c99' })

  const inst = document.createElement('div')
  inst.textContent = instruction
  inst.style.fontSize = '12px'

  const cmdBox = document.createElement('div')
  Object.assign(cmdBox.style, {
    background: '#16161e',
    border: '1px solid #292e42',
    borderRadius: '8px',
    padding: '10px 12px',
    fontFamily: '"SF Mono", "Fira Code", Menlo, Monaco, monospace',
    fontSize: '12px',
    color: '#9ece6a'
  })
  cmdBox.textContent = command

  overlay.appendChild(title)
  overlay.appendChild(msg)
  // Not every start failure has a remedy to offer; empty boxes would just be noise.
  if (instruction) overlay.appendChild(inst)
  if (command) overlay.appendChild(cmdBox)
  container.appendChild(overlay)
}
