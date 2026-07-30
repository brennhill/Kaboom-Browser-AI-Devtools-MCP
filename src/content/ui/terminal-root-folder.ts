/**
 * Purpose: The root-folder bar above the terminal — shows the working directory
 * and lets the user change it, which relaunches the shell there.
 * Why: A PTY's cwd is fixed at spawn, so pointing the agent at a different repo
 * is a restart, not a setting. It used to live only in the no-session state and
 * on the options page, so with a session running there was no way to see or
 * change where the shell actually was.
 * Docs: docs/features/feature/terminal/index.md
 */

import {
  ROOT_FOLDER_BAR_ID,
  ROOT_FOLDER_BROWSE_BUTTON_ID,
  ROOT_FOLDER_INPUT_ID,
  ROOT_FOLDER_PICKER_ID,
  ROOT_FOLDER_PICKER_UP_ID,
  ROOT_FOLDER_PICKER_USE_ID,
  ROOT_FOLDER_SAVE_BUTTON_ID
} from './terminal-widget-types.js'
import { listTerminalDirs, type TerminalDirListing, type TerminalDirsFailure } from './terminal-widget-session.js'

/** Height of the bar; the terminal takes the rest of the panel. */
const BAR_HEIGHT = '34px'

/** Tallest the browser list grows before it scrolls. */
const PICKER_MAX_HEIGHT = '220px'

export interface RootFolderBarOptions {
  /** Current root, or '' when the daemon picks one. */
  initialRoot: string
  /** Apply a new root. The caller restarts the session. */
  onApply: (root: string) => void
}

function styleButton(button: HTMLButtonElement, color: string): void {
  button.type = 'button'
  Object.assign(button.style, {
    padding: '3px 8px',
    borderRadius: '5px',
    border: `1px solid ${color}`,
    background: 'transparent',
    color,
    cursor: 'pointer',
    fontSize: '11px',
    lineHeight: '16px',
    flexShrink: '0'
  })
}

/**
 * Build the bar.
 *
 * Returns the element plus a `setRoot` so the panel can reflect a root that
 * changed elsewhere without rebuilding the bar and losing focus mid-typing.
 */
export function createRootFolderBar(options: RootFolderBarOptions): {
  element: HTMLDivElement
  setRoot: (root: string) => void
} {
  const bar = document.createElement('div')
  bar.id = ROOT_FOLDER_BAR_ID
  Object.assign(bar.style, {
    display: 'flex',
    flexDirection: 'column',
    flexShrink: '0',
    background: '#16161e',
    borderBottom: '1px solid #292e42'
  })

  const row = document.createElement('div')
  Object.assign(row.style, {
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    height: BAR_HEIGHT,
    padding: '0 8px'
  })

  const label = document.createElement('label')
  label.textContent = 'Root'
  label.htmlFor = ROOT_FOLDER_INPUT_ID
  Object.assign(label.style, { color: '#787c99', fontSize: '11px', flexShrink: '0' })

  const input = document.createElement('input')
  input.id = ROOT_FOLDER_INPUT_ID
  input.type = 'text'
  input.value = options.initialRoot
  input.placeholder = '~/dev/your-project'
  input.title = 'Working directory the shell starts in'
  Object.assign(input.style, {
    flex: '1',
    minWidth: '0',
    padding: '3px 6px',
    borderRadius: '5px',
    border: '1px solid #292e42',
    background: '#1a1b26',
    color: '#c0caf5',
    fontSize: '11px'
  })

  const browse = document.createElement('button')
  browse.id = ROOT_FOLDER_BROWSE_BUTTON_ID
  browse.textContent = 'Browse'
  browse.title = 'Pick a folder'
  styleButton(browse, '#7aa2f7')

  const apply = document.createElement('button')
  apply.id = ROOT_FOLDER_SAVE_BUTTON_ID
  apply.textContent = 'Reload'
  // The shell cannot move, so this is a relaunch. Saying "Save" would hide that
  // the running session — and whatever is in it — is about to be replaced.
  apply.title = 'Restart the shell in this folder'
  styleButton(apply, '#9ece6a')

  const picker = document.createElement('div')
  picker.id = ROOT_FOLDER_PICKER_ID
  Object.assign(picker.style, {
    display: 'none',
    flexDirection: 'column',
    maxHeight: PICKER_MAX_HEIGHT,
    overflowY: 'auto',
    borderTop: '1px solid #292e42',
    background: '#1a1b26'
  })

  const applyCurrent = (): void => {
    options.onApply(input.value.trim())
  }

  apply.addEventListener('click', (event: MouseEvent) => {
    event.preventDefault()
    event.stopPropagation()
    applyCurrent()
  })

  input.addEventListener('keydown', (event: KeyboardEvent) => {
    if (event.key !== 'Enter') return
    event.preventDefault()
    applyCurrent()
  })

  browse.addEventListener('click', (event: MouseEvent) => {
    event.preventDefault()
    event.stopPropagation()
    if (picker.style.display === 'flex') {
      picker.style.display = 'none'
      return
    }
    picker.style.display = 'flex'
    void openPicker(picker, input.value.trim(), (chosen) => {
      // Auto-accept: picking a folder ("✓ Use …") both fills the field AND launches
      // the shell there — no separate Reload/Start click needed.
      input.value = chosen
      applyCurrent()
    })
  })

  row.appendChild(label)
  row.appendChild(input)
  row.appendChild(browse)
  row.appendChild(apply)
  bar.appendChild(row)
  bar.appendChild(picker)

  // Default location: when no root is set, show the daemon's own resolved working
  // directory (where browsing starts) instead of an empty placeholder. The shell
  // already auto-starts on panel open against this same default; this just makes it
  // visible. Best-effort — a daemon that can't list dirs leaves the placeholder.
  if (!options.initialRoot) {
    void listTerminalDirs('')
      .then((result) => {
        if (result.ok && !input.value && document.activeElement !== input) {
          input.value = result.listing.path
        }
      })
      .catch(() => {
        // EXPECTED_ABSENCE: daemon unavailability is normal before terminal startup;
        // logging it would misleadingly mark the usable manual root input failed.
      })
  }

  return {
    element: bar,
    setRoot: (root: string) => {
      if (document.activeElement === input) return // Never fight the user's typing.
      input.value = root
    }
  }
}

function pickerRow(text: string, color: string): HTMLButtonElement {
  const row = document.createElement('button')
  row.type = 'button'
  row.textContent = text
  Object.assign(row.style, {
    display: 'block',
    width: '100%',
    textAlign: 'left',
    padding: '5px 10px',
    border: 'none',
    background: 'transparent',
    color,
    cursor: 'pointer',
    fontSize: '11px'
  })
  return row
}

/**
 * The user-facing reason a listing failed. Each maps to a distinct cause so the
 * message points at the actual fix — every one still ends in "type a path"
 * because the field keeps working regardless of why the browser could not.
 */
function pickerFailureMessage(reason: TerminalDirsFailure): string {
  switch (reason) {
    case 'outdated':
      return 'This Kaboom daemon is too old to browse folders — update Kaboom, or type a path'
    case 'not_found':
      return 'That folder no longer exists — type a path, or browse from one that does'
    case 'denied':
      return 'That folder can’t be read (permission denied) — type a path instead'
    case 'unreachable':
    default:
      return 'Could not reach the Kaboom daemon — type a path instead'
  }
}

/**
 * Render one level of the directory tree into `picker`.
 *
 * Navigation reuses the same element rather than opening a dialog: the panel is
 * narrow, and a modal over a terminal hides the thing being configured.
 */
async function openPicker(picker: HTMLDivElement, path: string, onChoose: (path: string) => void): Promise<void> {
  picker.replaceChildren(pickerRow('Loading…', '#787c99'))

  const result = await listTerminalDirs(path)
  if (!result.ok) {
    // Typing a path still works, so every one of these is a degraded state, not a
    // dead end — but each has a different cause, and conflating them sends the
    // user debugging the wrong thing (updating a daemon that is current, or
    // chasing a connection that is fine).
    picker.replaceChildren(pickerRow(pickerFailureMessage(result.reason), '#f7768e'))
    return
  }
  const listing = result.listing

  picker.replaceChildren()
  picker.appendChild(currentFolderRow(listing, onChoose, picker))

  if (listing.parent) {
    const up = pickerRow('⬆︎ ..', '#7aa2f7')
    up.id = ROOT_FOLDER_PICKER_UP_ID
    up.addEventListener('click', (event: MouseEvent) => {
      event.preventDefault()
      void openPicker(picker, listing.parent, onChoose)
    })
    picker.appendChild(up)
  }

  for (const entry of listing.entries) {
    const row = pickerRow(`📁 ${entry.name}`, '#c0caf5')
    row.addEventListener('click', (event: MouseEvent) => {
      event.preventDefault()
      void openPicker(picker, entry.path, onChoose)
    })
    picker.appendChild(row)
  }

  if (listing.truncated) {
    // Silently showing part of a directory reads as showing all of it.
    picker.appendChild(pickerRow('… more folders not shown; type the path to reach them', '#787c99'))
  } else if (listing.entries.length === 0) {
    picker.appendChild(pickerRow('No sub-folders here', '#787c99'))
  }
}

/** The "use where I am" row, pinned to the top of every level. */
function currentFolderRow(
  listing: TerminalDirListing,
  onChoose: (path: string) => void,
  picker: HTMLDivElement
): HTMLButtonElement {
  const use = pickerRow(`✓ Use ${listing.path}`, '#9ece6a')
  use.id = ROOT_FOLDER_PICKER_USE_ID
  use.style.borderBottom = '1px solid #292e42'
  use.addEventListener('click', (event: MouseEvent) => {
    event.preventDefault()
    event.stopPropagation()
    onChoose(listing.path)
    picker.style.display = 'none'
  })
  return use
}
