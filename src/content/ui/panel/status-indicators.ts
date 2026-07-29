/**
 * Purpose: Renders terminal connection and subscription/API provider indicators.
 * Why: Keeps compact status presentation with the terminal panel shell.
 * Docs: docs/features/feature/terminal/index.md
 */

export function updateConnectionIndicator(
  dot: HTMLSpanElement | null,
  state: 'connected' | 'disconnected' | 'exited'
): void {
  if (!dot) return
  const colors = {
    connected: '#9ece6a',
    disconnected: '#e0af68',
    exited: '#f7768e'
  } as const
  dot.style.background = colors[state]
}

export function updateExecutionProviderBadge(
  badge: HTMLSpanElement | null,
  provider: string,
  tool: string,
  onAPIBilling: () => void
): void {
  if (!badge) return
  const toolLabel = tool === 'codex' ? 'Codex' : tool === 'claude' ? 'Claude' : 'AI'
  if (provider === 'subscription') {
    badge.textContent = `${toolLabel} · Subscription`
    badge.title = tool === 'codex' ? 'Using your ChatGPT subscription' : 'Using your Claude subscription'
    badge.style.color = '#9ece6a'
    badge.style.borderColor = '#9ece6a'
    return
  }
  if (provider === 'api') {
    badge.textContent = `${toolLabel} · API billing`
    badge.title = 'API usage billing is active; this is not subscription usage'
    badge.style.color = '#e0af68'
    badge.style.borderColor = '#e0af68'
    onAPIBilling()
    return
  }
  badge.textContent = `${toolLabel} · Provider unknown`
  badge.title = 'Kaboom could not determine the execution provider'
  badge.style.color = '#f7768e'
  badge.style.borderColor = '#f7768e'
}
