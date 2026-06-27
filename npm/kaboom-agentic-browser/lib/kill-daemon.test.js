// Purpose: Validate daemon cleanup behavior for install/uninstall upgrade paths.
// Why: Prevents stale daemon processes from breaking MCP handoff during wrapper operations.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const {
  KNOWN_PORTS,
  matchesDaemonCommandLine,
  isKaboomDaemonHealth,
  killByKnownPorts,
} = require('./kill-daemon');

function writeExecutable(filePath, body) {
  fs.writeFileSync(filePath, body, { mode: 0o755 });
}

function runKillDaemon({ homeDir, binDir, env = {}, logPath }) {
  const scriptPath = path.join(__dirname, 'kill-daemon.js');
  const run = spawnSync(process.execPath, [scriptPath], {
    env: {
      ...process.env,
      ...env,
      KABOOM_KILL_DAEMON_DRY_RUN: '1',
      KABOOM_KILL_DAEMON_LOG: logPath || '',
      HOME: homeDir,
      PATH: `${binDir}${path.delimiter}${process.env.PATH || ''}`,
    },
    encoding: 'utf8',
  });
  assert.equal(run.status, 0, `kill-daemon.js exited with ${run.status}: ${run.stderr}`);
}

test('cleanup targets kaboom and legacy daemon names', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-kill-test-'));
  const binDir = path.join(tmp, 'bin');
  fs.mkdirSync(binDir, { recursive: true });

  const logPath = path.join(tmp, 'kill-daemon.log');
  runKillDaemon({ homeDir: tmp, binDir, logPath });

  const log = fs.existsSync(logPath) ? fs.readFileSync(logPath, 'utf8') : '';
  if (process.platform === 'win32') {
    assert.match(log, /kaboom-agentic-browser\*\.exe/, 'expected cleanup to target kaboom-agentic-browser*.exe');
    assert.match(log, /gasoline\*\.exe/, 'expected cleanup to target gasoline*.exe');
    assert.match(log, /browser-agent\*\.exe/, 'expected cleanup to target legacy browser-agent*.exe');
    assert.match(log, /\[execFile\] kaboom-agentic-browser --force/, 'expected cleanup to invoke kaboom-agentic-browser --force');
    return;
  }

  assert.match(
    log,
    /\[pattern\] .*\(kaboom\|gasoline\|strum\)-/,
    'expected cleanup to target anchored full daemon binary names'
  );
  assert.doesNotMatch(log, /\[pattern\] kaboom\s*$/m, 'must not pgrep on bare "kaboom"');
  assert.doesNotMatch(log, /\[pattern\] gasoline\s*$/m, 'must not pgrep on bare "gasoline"');
});

// --- Identity-gated process matching (regression: blind kills by substring/port) ---

test('daemon command-line matching only targets full kaboom binary names', () => {
  // Must NOT match unrelated processes that merely mention "kaboom"/"gasoline".
  assert.equal(matchesDaemonCommandLine('vim /Users/dev/kaboom/notes.md'), false);
  assert.equal(matchesDaemonCommandLine('tail -f /var/log/gasoline'), false);
  assert.equal(matchesDaemonCommandLine('bash ./strum-along.sh'), false);
  // Must match real daemon binary names (current and legacy).
  assert.equal(matchesDaemonCommandLine('/usr/local/bin/kaboom-agentic-browser --port 7890'), true);
  assert.equal(matchesDaemonCommandLine('gasoline-mcp --port 7890'), true);
  assert.equal(matchesDaemonCommandLine('strum-agentic-browser serve'), true);
  assert.equal(matchesDaemonCommandLine('/home/u/.kaboom/bin/kaboom-agentic-browser'), true);
});

test('health identity check accepts kaboom and legacy daemons only', () => {
  assert.equal(isKaboomDaemonHealth({ 'service-name': 'kaboom-browser-devtools' }), true);
  assert.equal(isKaboomDaemonHealth({ service_name: 'kaboom-browser-devtools' }), true);
  assert.equal(isKaboomDaemonHealth({ 'service-name': 'gasoline-browser-devtools' }), true);
  assert.equal(isKaboomDaemonHealth({ service_name: 'gasoline' }), true);
  assert.equal(isKaboomDaemonHealth({ 'service-name': 'strum-browser-devtools' }), true);
  assert.equal(isKaboomDaemonHealth({ 'service-name': 'vite-dev-server' }), false);
  assert.equal(isKaboomDaemonHealth({ 'service-name': 'my-kaboom-clone-2' }), false);
  assert.equal(isKaboomDaemonHealth({}), false);
  assert.equal(isKaboomDaemonHealth(null), false);
});

test('killByKnownPorts only kills ports whose /health identifies a kaboom daemon', async () => {
  if (process.platform === 'win32') return;
  const killed = [];
  const probed = [];
  await killByKnownPorts({
    fetchHealth: async (port) => {
      probed.push(port);
      if (port === 7890) return { 'service-name': 'kaboom-browser-devtools' };
      if (port === 7891) return { 'service-name': 'vite-dev-server' };
      return null; // no /health listener — must not be killed
    },
    killPort: (port) => killed.push(port),
  });
  assert.deepEqual(killed, [7890], 'must only kill ports owned by a kaboom daemon');
  assert.equal(probed.length, KNOWN_PORTS.length, 'must probe every known port');
});

test('cleanup removes kaboom and legacy pid files', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-kill-pids-'));
  const binDir = path.join(tmp, 'bin');
  fs.mkdirSync(binDir, { recursive: true });

  const modernPid = path.join(tmp, '.kaboom', 'run', 'kaboom-7890.pid');
  const legacyPid = path.join(tmp, '.gasoline-7890.pid');
  fs.mkdirSync(path.dirname(modernPid), { recursive: true });
  fs.writeFileSync(modernPid, '123');
  fs.writeFileSync(legacyPid, '456');

  runKillDaemon({ homeDir: tmp, binDir, logPath: path.join(tmp, 'kill-daemon.log') });

  assert.equal(fs.existsSync(modernPid), false, `expected pid file removed: ${modernPid}`);
  assert.equal(fs.existsSync(legacyPid), false, `expected pid file removed: ${legacyPid}`);
});

test('cleanup removes pid files across known ports and XDG state root', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-kill-known-ports-'));
  const binDir = path.join(tmp, 'bin');
  const xdgStateHome = path.join(tmp, 'xdg-state');
  fs.mkdirSync(binDir, { recursive: true });

  const trackedPaths = [];
  for (const port of KNOWN_PORTS) {
    const modernPid = path.join(tmp, '.kaboom', 'run', `kaboom-${port}.pid`);
    const xdgPid = path.join(xdgStateHome, 'kaboom', 'run', `kaboom-${port}.pid`);
    const legacyPid = path.join(tmp, `.gasoline-${port}.pid`);

    fs.mkdirSync(path.dirname(modernPid), { recursive: true });
    fs.mkdirSync(path.dirname(xdgPid), { recursive: true });
    fs.writeFileSync(modernPid, String(port));
    fs.writeFileSync(xdgPid, String(port));
    fs.writeFileSync(legacyPid, String(port));

    trackedPaths.push(modernPid, xdgPid, legacyPid);
  }

  runKillDaemon({
    homeDir: tmp,
    binDir,
    env: { XDG_STATE_HOME: xdgStateHome },
    logPath: path.join(tmp, 'kill-daemon.log'),
  });

  for (const pidPath of trackedPaths) {
    assert.equal(fs.existsSync(pidPath), false, `expected pid file removed: ${pidPath}`);
  }
});

test('cleanup attempts to terminate pids discovered from pid files', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-kill-pid-kill-'));
  const binDir = path.join(tmp, 'bin');
  fs.mkdirSync(binDir, { recursive: true });

  const modernPid = path.join(tmp, '.kaboom', 'run', 'kaboom-22222.pid');
  fs.mkdirSync(path.dirname(modernPid), { recursive: true });
  fs.writeFileSync(modernPid, '22222');

  const logPath = path.join(tmp, 'kill-daemon.log');
  runKillDaemon({ homeDir: tmp, binDir, logPath });

  const log = fs.existsSync(logPath) ? fs.readFileSync(logPath, 'utf8') : '';
  assert.match(log, /\[pid\] 22222/, 'expected cleanup to attempt pid-file based process termination');
});

test('npm lifecycle hooks invoke daemon cleanup script', () => {
  const pkgPath = path.join(__dirname, '..', 'package.json');
  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));

  assert.equal(pkg?.scripts?.preinstall, 'node lib/kill-daemon.js');
  assert.equal(pkg?.scripts?.preuninstall, 'node lib/kill-daemon.js');
});
