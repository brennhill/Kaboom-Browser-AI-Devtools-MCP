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
  cleanupOldDaemons,
  healthySameVersionDaemonRunning,
} = require('./kill-daemon');

// --- Idempotent install: never kill a healthy same-version daemon (restart-storm guard) ---

function fakeHealth(map) {
  // map: { port: healthObjectOrNull }
  return (port) => Promise.resolve(port in map ? map[port] : null);
}

test('healthySameVersionDaemonRunning: true only for a healthy daemon on the exact version', async () => {
  const port = KNOWN_PORTS[0];
  assert.equal(
    await healthySameVersionDaemonRunning({
      selfVersion: '0.8.7',
      fetchHealth: fakeHealth({ [port]: { service: 'kaboom', version: '0.8.7' } }),
    }),
    true,
    'healthy + same version => already running, skip cleanup'
  );
  assert.equal(
    await healthySameVersionDaemonRunning({
      selfVersion: '0.8.7',
      fetchHealth: fakeHealth({ [port]: { service: 'kaboom', version: '0.8.6' } }),
    }),
    false,
    'different version (upgrade) => must replace'
  );
  assert.equal(
    await healthySameVersionDaemonRunning({
      selfVersion: '0.8.7',
      fetchHealth: fakeHealth({}), // stalled/no daemon answers /health
    }),
    false,
    'no health answer (stalled/absent) => proceed to cleanup'
  );
});

test('healthySameVersionDaemonRunning: retries a momentarily-busy healthy daemon (no false kill)', async () => {
  const port = KNOWN_PORTS[0];
  let calls = 0;
  // Busy (null) on the first two probes, answers healthy same-version on the 3rd —
  // exactly the GC-pause/load hiccup that a single 500ms probe would misread as
  // "down" and kill, re-triggering the restart storm.
  const flaky = (p) => {
    if (p !== port) return Promise.resolve(null);
    calls += 1;
    return Promise.resolve(calls >= 3 ? { service: 'kaboom', version: '0.8.7' } : null);
  };
  const result = await healthySameVersionDaemonRunning({
    selfVersion: '0.8.7',
    fetchHealth: flaky,
    sleep: () => Promise.resolve(), // no real backoff wait in the test
  });
  assert.equal(result, true, 'a healthy daemon that misses early probes must still be detected');
  assert.ok(calls >= 3, 'the probe must retry, not give up on the first miss');
});

test('healthySameVersionDaemonRunning: a daemon that never answers still fails after retries', async () => {
  const alwaysDown = () => Promise.resolve(null);
  const result = await healthySameVersionDaemonRunning({
    selfVersion: '0.8.7',
    fetchHealth: alwaysDown,
    sleep: () => Promise.resolve(),
  });
  assert.equal(result, false, 'a truly-down/stalled daemon must still be cleaned up');
});

test('cleanupOldDaemons: skips ALL kills on a same-version (re)install', async () => {
  let killed = false;
  const spies = {
    runForceCleanupCommands: () => { killed = true; },
    killByProcessName: () => { killed = true; },
    killByKnownPorts: async () => { killed = true; },
    cleanupPIDFiles: () => { killed = true; },
  };
  const port = KNOWN_PORTS[0];
  await cleanupOldDaemons({
    env: { npm_lifecycle_event: 'preinstall' },
    selfVersion: '0.8.7',
    fetchHealth: fakeHealth({ [port]: { service: 'kaboom', version: '0.8.7' } }),
    ...spies,
  });
  assert.equal(killed, false, 'a healthy same-version daemon must NOT be killed on reinstall');
});

test('cleanupOldDaemons: still tears down on a version upgrade', async () => {
  let portKills = 0;
  const port = KNOWN_PORTS[0];
  await cleanupOldDaemons({
    env: { npm_lifecycle_event: 'preinstall' },
    selfVersion: '0.8.8',
    fetchHealth: fakeHealth({ [port]: { service: 'kaboom', version: '0.8.7' } }),
    runForceCleanupCommands: () => {},
    killByProcessName: () => {},
    killByKnownPorts: async () => { portKills += 1; },
    cleanupPIDFiles: () => {},
  });
  assert.equal(portKills, 1, 'an older-version daemon must be replaced on upgrade');
});

test('cleanupOldDaemons: uninstall always tears down, even same version', async () => {
  let torn = false;
  const port = KNOWN_PORTS[0];
  await cleanupOldDaemons({
    env: { npm_lifecycle_event: 'preuninstall' },
    selfVersion: '0.8.7',
    fetchHealth: fakeHealth({ [port]: { service: 'kaboom', version: '0.8.7' } }),
    runForceCleanupCommands: () => {},
    killByProcessName: () => {},
    killByKnownPorts: async () => { torn = true; },
    cleanupPIDFiles: () => {},
  });
  assert.equal(torn, true, 'uninstall must remove the daemon regardless of version');
});

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

test('cleanup targets only canonical kaboom daemon names', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-kill-test-'));
  const binDir = path.join(tmp, 'bin');
  fs.mkdirSync(binDir, { recursive: true });

  const logPath = path.join(tmp, 'kill-daemon.log');
  runKillDaemon({ homeDir: tmp, binDir, logPath });

  const log = fs.existsSync(logPath) ? fs.readFileSync(logPath, 'utf8') : '';
  if (process.platform === 'win32') {
    assert.match(log, /kaboom-agentic-browser\*\.exe/, 'expected cleanup to target kaboom-agentic-browser*.exe');
    assert.match(log, /\[execFile\] kaboom-agentic-browser --force/, 'expected cleanup to invoke kaboom-agentic-browser --force');
    return;
  }

  assert.match(
    log,
    /\[pattern\] kaboom-\(agentic-browser\|agentic-devtools\|browser-devtools\|hooks\|mcp\)/,
    'expected cleanup to target anchored full daemon binary names'
  );
  assert.doesNotMatch(log, /\[pattern\] kaboom\s*$/m, 'must not pgrep on bare "kaboom"');
});

// --- Identity-gated process matching (regression: blind kills by substring/port) ---

test('daemon command-line matching only targets full kaboom binary names', () => {
  // Must NOT match unrelated processes that merely mention Kaboom.
  assert.equal(matchesDaemonCommandLine('vim /Users/dev/kaboom/notes.md'), false);
  assert.equal(matchesDaemonCommandLine('tail -f /var/log/kaboom'), false);
  // Must match canonical daemon binary names.
  assert.equal(matchesDaemonCommandLine('/usr/local/bin/kaboom-agentic-browser --port 7890'), true);
  assert.equal(matchesDaemonCommandLine('/home/u/.kaboom/bin/kaboom-agentic-browser'), true);
});

test('health identity check accepts canonical kaboom daemons only', () => {
  assert.equal(isKaboomDaemonHealth({ 'service-name': 'kaboom-browser-devtools' }), true);
  assert.equal(isKaboomDaemonHealth({ service_name: 'kaboom-browser-devtools' }), true);
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

test('cleanup removes canonical kaboom pid files', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-kill-pids-'));
  const binDir = path.join(tmp, 'bin');
  fs.mkdirSync(binDir, { recursive: true });

  const modernPid = path.join(tmp, '.kaboom', 'run', 'kaboom-7890.pid');
  fs.mkdirSync(path.dirname(modernPid), { recursive: true });
  fs.writeFileSync(modernPid, '123');

  runKillDaemon({ homeDir: tmp, binDir, logPath: path.join(tmp, 'kill-daemon.log') });

  assert.equal(fs.existsSync(modernPid), false, `expected pid file removed: ${modernPid}`);
});

test('cleanup removes pid files across known ports and XDG state root', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-kill-known-ports-'));
  const binDir = path.join(tmp, 'bin');
  const xdgStateHome = path.join(tmp, 'xdg-state');
  fs.mkdirSync(binDir, { recursive: true });

  const trackedPaths = [];
  for (const port of KNOWN_PORTS) {
    const xdgPid = path.join(xdgStateHome, 'kaboom', 'run', `kaboom-${port}.pid`);

    fs.mkdirSync(path.dirname(xdgPid), { recursive: true });
    fs.writeFileSync(xdgPid, String(port));

    trackedPaths.push(xdgPid);
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
  const pkgPath = path.join(__dirname, '..', '..', 'package.json');
  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));

  assert.equal(pkg?.scripts?.preinstall, 'node lib/daemon/kill-daemon.js');
  assert.equal(pkg?.scripts?.preuninstall, 'node lib/daemon/kill-daemon.js');
});
