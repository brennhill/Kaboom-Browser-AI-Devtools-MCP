// Purpose: Validate the install-output auto-approve transparency summary.
// Why: The installer must state exactly which clients were auto-approved via
// config and which need manual in-app approval (default-ON but transparent).
// Docs: docs/features/feature/enhanced-cli-config/index.md

const test = require('node:test');
const assert = require('node:assert/strict');
const output = require('./output');

test('installResult lists config auto-approved clients separately from UI-only', () => {
  const text = output.installResult({
    installed: [
      { name: 'Claude Code', method: 'cli', autoApprove: 'applied' },
      { name: 'Gemini CLI', method: 'file', path: '/g', autoApprove: 'applied' },
      { name: 'Zed', method: 'file', path: '/z', autoApprove: 'unchanged' },
      { name: 'Cursor', method: 'file', path: '/c', autoApprove: 'ui-only' },
      { name: 'VS Code', method: 'file', path: '/v', autoApprove: 'ui-only' },
    ],
    total: 10,
    errors: [],
  });
  assert.match(text, /Tool auto-approve/);
  assert.match(text, /Auto-approved via config: Claude Code, Gemini CLI, Zed/);
  assert.match(text, /Manual in-app approval \(no config option\): Cursor, VS Code/);
});

test('installResult surfaces a failed auto-approve with its error', () => {
  const text = output.installResult({
    installed: [
      { name: 'Claude Code', method: 'cli', autoApprove: 'failed', autoApproveError: 'EACCES' },
    ],
    total: 10,
    errors: [],
  });
  assert.match(text, /Claude Code: could not write auto-approve \(EACCES\)/);
});

test('installResult omits the auto-approve section when no entry carries a status', () => {
  const text = output.installResult({
    updated: [{ name: 'Claude Desktop', path: '/path' }],
    total: 4,
    errors: [],
  });
  assert.ok(text.includes('Claude Desktop'));
  assert.doesNotMatch(text, /Tool auto-approve/);
});
