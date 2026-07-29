/**
 * @fileoverview Safety-contract tests for scripts/install.sh (and docs install
 * one-liners). Static regression guards for the 2026-06-10 code-review fixes:
 * anchored process-kill patterns, backup-preserving cleanup,
 * bash-only invocation strings, fish rc-dir creation, exact checksum matching,
 * kaboom-identity health checks, EXT_DIR guards, and stale-staging sweeps.
 */

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..')
const INSTALL_SH = path.join(REPO_ROOT, 'scripts', 'install.sh')
const UNINSTALL_SH = path.join(REPO_ROOT, 'scripts', 'uninstall.sh')

const script = fs.readFileSync(INSTALL_SH, 'utf8')
const lines = script.split('\n')

/** Extracts a top-level bash function body (functions close with a column-0 brace). */
function functionBody(name) {
  const match = script.match(new RegExp(`${name}\\(\\) \\{[\\s\\S]*?\\n\\}`))
  assert.ok(match, `install.sh must define ${name}()`)
  return match[0]
}

// ─────────────────────────────────────────────────────────────
// Baseline — the installer must parse as bash.
// ─────────────────────────────────────────────────────────────

test('install.sh parses cleanly under bash -n', () => {
  const result = spawnSync('bash', ['-n', INSTALL_SH], { encoding: 'utf8' })
  assert.equal(result.status, 0, `bash -n failed:\n${result.stderr}`)
})

// ─────────────────────────────────────────────────────────────
// Finding 3 — process-kill patterns must be anchored full names,
// consistent with scripts/uninstall.sh.
// ─────────────────────────────────────────────────────────────

test('process kill patterns are anchored and match uninstall.sh', () => {
  const anchored = String.raw`kaboom-(agentic-browser|hooks)|\.kaboom/bin/`

  assert.ok(
    script.includes(anchored),
    'install.sh must use the anchored full-name process pattern'
  )

  const uninstall = fs.readFileSync(UNINSTALL_SH, 'utf8')
  assert.ok(
    uninstall.includes(anchored),
    'uninstall.sh must keep the same anchored process pattern (consistency)'
  )

  // Every pgrep/pkill invocation must go through the shared anchored pattern.
  const procLines = lines.filter((l) => /\b(pgrep|pkill)\b.* -f /.test(l))
  assert.ok(procLines.length >= 3, 'expected pgrep + pkill TERM + pkill KILL branches')
  for (const line of procLines) {
    assert.ok(
      line.includes('"$proc_pattern"'),
      `pgrep/pkill must use the shared anchored pattern variable, got: ${line.trim()}`
    )
  }
})

// ─────────────────────────────────────────────────────────────
// Finding 4 — interrupted promotion must not destroy the user's
// only extension copy: the EXIT trap restores the backup.
// ─────────────────────────────────────────────────────────────

test('cleanup trap restores extension backup before deleting it', () => {
  const body = functionBody('cleanup')

  assert.match(
    body,
    /if \[ ! -d "\$EXT_DIR" \] && \[ -d "\$BACKUP_EXT_DIR" \]/,
    'cleanup must detect a missing EXT_DIR with a surviving backup'
  )
  assert.match(
    body,
    /mv "\$BACKUP_EXT_DIR" "\$EXT_DIR"/,
    'cleanup must restore the backup to EXT_DIR instead of deleting the only copy'
  )

  const restoreIdx = body.indexOf('mv "$BACKUP_EXT_DIR" "$EXT_DIR"')
  const deleteIdx = body.indexOf('rm -rf "$STAGE_EXT_DIR" "$BACKUP_EXT_DIR"')
  assert.ok(deleteIdx > -1, 'cleanup must still remove staging/backup debris')
  assert.ok(restoreIdx > -1 && restoreIdx < deleteIdx, 'restore must happen before deletion')
})

// ─────────────────────────────────────────────────────────────
// Finding 5 — install.sh is bash-only; documented invocations
// must pipe to bash, never sh.
// ─────────────────────────────────────────────────────────────

test('install.sh and docs never pipe the installer to sh', () => {
  assert.doesNotMatch(
    script,
    /\|\s*sh(\s|$)/m,
    'install.sh is bash-only (set -o pipefail, echo -e, ==): pipe to bash, not sh'
  )
  assert.match(script, /install\.sh \| bash/, 'usage strings must pipe install.sh to bash')

  const docPaths = [
    path.join(REPO_ROOT, 'README.md'),
    path.join(REPO_ROOT, 'docs', 'features', 'feature', 'quality-gates', 'index.md'),
    path.join(REPO_ROOT, 'docs', 'features', 'feature', 'quality-gates', 'setup-guide.md'),
  ]
  for (const docPath of docPaths) {
    const doc = fs.readFileSync(docPath, 'utf8')
    assert.doesNotMatch(
      doc,
      /install\.sh \| sh\b/,
      `${path.relative(REPO_ROOT, docPath)} must pipe install.sh to bash, not sh`
    )
  }
})

// ─────────────────────────────────────────────────────────────
// Finding 6 — register_path must create the rc-file directory
// (fish: ~/.config/fish may not exist) before appending.
// ─────────────────────────────────────────────────────────────

test('register_path creates the rc-file directory before appending', () => {
  const body = functionBody('register_path')

  assert.match(
    body,
    /mkdir -p "\$\(dirname "\$rc_file"\)"/,
    'register_path must mkdir -p the rc-file directory (fish config dir may not exist)'
  )

  const mkdirIdx = body.indexOf('mkdir -p "$(dirname "$rc_file")"')
  const appendIdx = body.indexOf('>> "$rc_file"')
  assert.ok(appendIdx > -1, 'register_path must append the PATH line')
  assert.ok(mkdirIdx > -1 && mkdirIdx < appendIdx, 'mkdir must precede the rc-file append')
})

// ─────────────────────────────────────────────────────────────
// Finding 7 — checksum lookup must be an exact field match, not
// an unanchored substring grep.
// ─────────────────────────────────────────────────────────────

test('checksum lookup uses exact awk field match', () => {
  assert.match(
    script,
    /awk -v f="\$asset_name" '\$2 == f \|\| \$2 == "\*" f \{print \$1; exit\}' "\$TEMP_ROOT\/checksums\.txt"/,
    'expected_hash must come from an exact-field awk match on checksums.txt'
  )
  assert.doesNotMatch(
    script,
    /grep "\$asset_name" "\$TEMP_ROOT\/checksums\.txt"/,
    'unanchored grep on checksums.txt can multi-match similarly named assets'
  )
})

// ─────────────────────────────────────────────────────────────
// Finding 8 — health check must verify kaboom identity, not just
// any responder containing "status".
// ─────────────────────────────────────────────────────────────

test('post-install health check requires kaboom identity', () => {
  assert.match(
    script,
    /echo "\$HEALTH_RESPONSE" \| grep -q 'kaboom-browser-devtools'/,
    'health check must match the daemon identity (service-name kaboom-browser-devtools from /health)'
  )
})

// ─────────────────────────────────────────────────────────────
// Finding 9 — destructive rm/mv of EXT_DIR must be guarded
// (mirrors safe_rm_rf in scripts/uninstall.sh).
// ─────────────────────────────────────────────────────────────

test('EXT_DIR destructive operations are guarded against unsafe paths', () => {
  const guardBody = functionBody('assert_safe_ext_dir')

  assert.match(
    guardBody,
    /""\|"\/"\|"\$HOME"\|"\$HOME\/"/,
    'guard must refuse empty, /, and $HOME extension paths'
  )
  assert.match(guardBody, /\/\*\) ;;/, 'guard must require an absolute path')

  const promoteBody = functionBody('promote_extension_stage')
  assert.match(
    promoteBody,
    /assert_safe_ext_dir/,
    'promote_extension_stage must invoke the EXT_DIR guard before rm/mv of EXT_DIR'
  )
})

// ─────────────────────────────────────────────────────────────
// Finding 10 — stale PID-suffixed staging/backup debris from
// interrupted runs must be swept at script start.
// ─────────────────────────────────────────────────────────────

test('stale extension staging debris is swept at startup', () => {
  const body = functionBody('sweep_stale_staging_dirs')

  assert.match(body, /"\$INSTALL_DIR"\/\.extension-stage-\*/, 'must sweep orphaned stage dirs')
  assert.match(body, /"\$INSTALL_DIR"\/\.extension-backup-\*/, 'must sweep orphaned backup dirs')
  assert.match(body, /\[ "\$stale" = "\$STAGE_EXT_DIR" \]/, 'must never sweep this run\'s own stage dir')
  assert.match(body, /\[ "\$stale" = "\$BACKUP_EXT_DIR" \]/, 'must never sweep this run\'s own backup dir')

  const sweepCallIdx = lines.findIndex((l) => l.trim() === 'sweep_stale_staging_dirs')
  const extDownloadIdx = lines.findIndex((l) => l.includes('Refreshing browser extension'))
  assert.ok(sweepCallIdx > -1, 'sweep_stale_staging_dirs must be invoked')
  assert.ok(
    extDownloadIdx === -1 || sweepCallIdx < extDownloadIdx,
    'sweep must run before extension staging begins'
  )
})
