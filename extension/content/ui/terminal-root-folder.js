/**
 * Purpose: The root-folder bar above the terminal — shows the working directory
 * and lets the user change it, which relaunches the shell there.
 * Why: A PTY's cwd is fixed at spawn, so pointing the agent at a different repo
 * is a restart, not a setting. It used to live only in the no-session state and
 * on the options page, so with a session running there was no way to see or
 * change where the shell actually was.
 * Docs: docs/features/feature/terminal/index.md
 */
import { ROOT_FOLDER_BAR_ID, ROOT_FOLDER_BROWSE_BUTTON_ID, ROOT_FOLDER_INPUT_ID, ROOT_FOLDER_PICKER_ID, ROOT_FOLDER_PICKER_UP_ID, ROOT_FOLDER_PICKER_USE_ID, ROOT_FOLDER_SAVE_BUTTON_ID } from './terminal-widget-types.js';
import { listTerminalDirs } from './terminal-widget-session.js';
/** Height of the bar; the terminal takes the rest of the panel. */
const BAR_HEIGHT = '34px';
/** Tallest the browser list grows before it scrolls. */
const PICKER_MAX_HEIGHT = '220px';
function styleButton(button, color) {
    button.type = 'button';
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
    });
}
/**
 * Build the bar.
 *
 * Returns the element plus a `setRoot` so the panel can reflect a root that
 * changed elsewhere without rebuilding the bar and losing focus mid-typing.
 */
export function createRootFolderBar(options) {
    const bar = document.createElement('div');
    bar.id = ROOT_FOLDER_BAR_ID;
    Object.assign(bar.style, {
        display: 'flex',
        flexDirection: 'column',
        flexShrink: '0',
        background: '#16161e',
        borderBottom: '1px solid #292e42'
    });
    const row = document.createElement('div');
    Object.assign(row.style, {
        display: 'flex',
        alignItems: 'center',
        gap: '6px',
        height: BAR_HEIGHT,
        padding: '0 8px'
    });
    const label = document.createElement('label');
    label.textContent = 'Root';
    label.htmlFor = ROOT_FOLDER_INPUT_ID;
    Object.assign(label.style, { color: '#787c99', fontSize: '11px', flexShrink: '0' });
    const input = document.createElement('input');
    input.id = ROOT_FOLDER_INPUT_ID;
    input.type = 'text';
    input.value = options.initialRoot;
    input.placeholder = '~/dev/your-project';
    input.title = 'Working directory the shell starts in';
    Object.assign(input.style, {
        flex: '1',
        minWidth: '0',
        padding: '3px 6px',
        borderRadius: '5px',
        border: '1px solid #292e42',
        background: '#1a1b26',
        color: '#c0caf5',
        fontSize: '11px'
    });
    const browse = document.createElement('button');
    browse.id = ROOT_FOLDER_BROWSE_BUTTON_ID;
    browse.textContent = 'Browse';
    browse.title = 'Pick a folder';
    styleButton(browse, '#7aa2f7');
    const apply = document.createElement('button');
    apply.id = ROOT_FOLDER_SAVE_BUTTON_ID;
    apply.textContent = 'Reload';
    // The shell cannot move, so this is a relaunch. Saying "Save" would hide that
    // the running session — and whatever is in it — is about to be replaced.
    apply.title = 'Restart the shell in this folder';
    styleButton(apply, '#9ece6a');
    const picker = document.createElement('div');
    picker.id = ROOT_FOLDER_PICKER_ID;
    Object.assign(picker.style, {
        display: 'none',
        flexDirection: 'column',
        maxHeight: PICKER_MAX_HEIGHT,
        overflowY: 'auto',
        borderTop: '1px solid #292e42',
        background: '#1a1b26'
    });
    const applyCurrent = () => {
        options.onApply(input.value.trim());
    };
    apply.addEventListener('click', (event) => {
        event.preventDefault();
        event.stopPropagation();
        applyCurrent();
    });
    input.addEventListener('keydown', (event) => {
        if (event.key !== 'Enter')
            return;
        event.preventDefault();
        applyCurrent();
    });
    browse.addEventListener('click', (event) => {
        event.preventDefault();
        event.stopPropagation();
        if (picker.style.display === 'flex') {
            picker.style.display = 'none';
            return;
        }
        picker.style.display = 'flex';
        void openPicker(picker, input.value.trim(), (chosen) => {
            input.value = chosen;
        });
    });
    row.appendChild(label);
    row.appendChild(input);
    row.appendChild(browse);
    row.appendChild(apply);
    bar.appendChild(row);
    bar.appendChild(picker);
    return {
        element: bar,
        setRoot: (root) => {
            if (document.activeElement === input)
                return; // Never fight the user's typing.
            input.value = root;
        }
    };
}
function pickerRow(text, color) {
    const row = document.createElement('button');
    row.type = 'button';
    row.textContent = text;
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
    });
    return row;
}
/**
 * Render one level of the directory tree into `picker`.
 *
 * Navigation reuses the same element rather than opening a dialog: the panel is
 * narrow, and a modal over a terminal hides the thing being configured.
 */
async function openPicker(picker, path, onChoose) {
    picker.replaceChildren(pickerRow('Loading…', '#787c99'));
    const listing = await listTerminalDirs(path);
    if (!listing) {
        // Typing a path still works, so this is a degraded state, not a dead end.
        picker.replaceChildren(pickerRow('Could not reach the Kaboom daemon — type a path instead', '#f7768e'));
        return;
    }
    picker.replaceChildren();
    picker.appendChild(currentFolderRow(listing, onChoose, picker));
    if (listing.parent) {
        const up = pickerRow('⬆︎ ..', '#7aa2f7');
        up.id = ROOT_FOLDER_PICKER_UP_ID;
        up.addEventListener('click', (event) => {
            event.preventDefault();
            void openPicker(picker, listing.parent, onChoose);
        });
        picker.appendChild(up);
    }
    for (const entry of listing.entries) {
        const row = pickerRow(`📁 ${entry.name}`, '#c0caf5');
        row.addEventListener('click', (event) => {
            event.preventDefault();
            void openPicker(picker, entry.path, onChoose);
        });
        picker.appendChild(row);
    }
    if (listing.truncated) {
        // Silently showing part of a directory reads as showing all of it.
        picker.appendChild(pickerRow('… more folders not shown; type the path to reach them', '#787c99'));
    }
    else if (listing.entries.length === 0) {
        picker.appendChild(pickerRow('No sub-folders here', '#787c99'));
    }
}
/** The "use where I am" row, pinned to the top of every level. */
function currentFolderRow(listing, onChoose, picker) {
    const use = pickerRow(`✓ Use ${listing.path}`, '#9ece6a');
    use.id = ROOT_FOLDER_PICKER_USE_ID;
    use.style.borderBottom = '1px solid #292e42';
    use.addEventListener('click', (event) => {
        event.preventDefault();
        event.stopPropagation();
        onChoose(listing.path);
        picker.style.display = 'none';
    });
    return use;
}
//# sourceMappingURL=terminal-root-folder.js.map