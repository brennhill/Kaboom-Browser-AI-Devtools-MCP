/**
 * Cross-context message-envelope contract between the terminal iframe
 * (cmd/browser-agent/internal/terminal/assets/terminal_assets/terminal.html) and the
 * extension side that hosts it (src/content/ui/terminal-write-guard.ts +
 * src/sidepanel.ts).
 *
 * Why this exists: the eb248ff6 refactor dropped `target: 'kaboom-terminal'`
 * from notifyIframe. The iframe's listener silently drops any message whose
 * `target` is not exactly that, so every agent/annotation write, focus, and
 * redraw vanished — and no test caught it because the behavioural tests mocked
 * the iframe and only checked `command`/`text`, never the envelope. These
 * assertions pin BOTH directions of the envelope so a rename or drop on either
 * side fails the build (repo rule 20: cross-context message contracts).
 */
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

const TERMINAL_HTML = 'cmd/browser-agent/internal/terminal/assets/terminal_assets/terminal.html'
const WRITE_GUARD = 'src/content/ui/terminal-write-guard.ts'
const SIDEPANEL = 'src/sidepanel.ts'

function firstMatch(text, regex, label) {
  const m = text.match(regex)
  assert.ok(m, `${label}: expected to find /${regex.source}/ — the message-contract shape changed; update this test AND both sides together`)
  return m[1]
}

describe('terminal iframe message envelope contract', () => {
  const html = readFileSync(TERMINAL_HTML, 'utf8')
  const writeGuard = readFileSync(WRITE_GUARD, 'utf8')
  const sidepanel = readFileSync(SIDEPANEL, 'utf8')

  test('parent -> iframe: notifyIframe posts the exact `target` the iframe requires', () => {
    // The gate inside terminal.html: `if (event.data.target !== '<X>') return`.
    const requiredTarget = firstMatch(
      html,
      /event\.data\.target\s*!==\s*['"]([^'"]+)['"]/,
      'terminal.html inbound-message guard'
    )
    // What notifyIframe actually stamps on every outbound message.
    const sentTarget = firstMatch(
      writeGuard,
      /postMessage\(\s*\{\s*target:\s*['"]([^'"]+)['"]/,
      'notifyIframe postMessage target'
    )
    assert.strictEqual(
      sentTarget,
      requiredTarget,
      `notifyIframe posts target:'${sentTarget}' but terminal.html only accepts '${requiredTarget}'. ` +
        'A mismatch means every parent->iframe write/focus/redraw is silently dropped.'
    )
  })

  test('iframe -> parent: the parent accepts the exact `source` the iframe stamps', () => {
    // terminal.html stamps `source: '<A>'` on every message to the parent.
    const sentSource = firstMatch(
      html,
      /source:\s*['"]([^'"]+)['"]/,
      'terminal.html notifyParent source'
    )
    // sidepanel.ts gate: `if (event.data.source !== '<B>') return`.
    const requiredSource = firstMatch(
      sidepanel,
      /event\.data\.source\s*!==\s*['"]([^'"]+)['"]/,
      'sidepanel.ts inbound-message guard'
    )
    assert.strictEqual(
      sentSource,
      requiredSource,
      `terminal.html stamps source:'${sentSource}' but sidepanel.ts only accepts '${requiredSource}'. ` +
        'A mismatch means every iframe->parent event (connected/focus/typing/exited) is ignored.'
    )
  })
})
