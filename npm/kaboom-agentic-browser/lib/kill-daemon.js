// Purpose: Implement kill-daemon.js behavior for npm wrapper command flows.
// Why: Keeps distribution-channel behavior consistent and supportable.
// Docs: docs/features/feature/enhanced-cli-config/index.md

// kill-daemon.js — Best-effort daemon cleanup for install/uninstall.
// Goal: old binaries must not survive an upgrade in memory — without ever
// killing unrelated processes that merely share a port or a name substring.
const fs = require('fs');
const http = require('http');
const os = require('os');
const path = require('path');
const { execSync, execFileSync } = require('child_process');

const KNOWN_PORTS = [
  17890,
  ...Array.from({ length: 21 }, (_, i) => 7890 + i),
];

// Anchored to full daemon binary names (current + legacy brands). Never use
// bare substrings like "kaboom" — they match unrelated command lines
// (e.g. `vim ~/dev/kaboom/notes.md`).
const DAEMON_NAME_PATTERN =
  '(kaboom|gasoline|strum)-(agentic-browser|agentic-devtools|browser-devtools|hooks|mcp)|\\.kaboom/bin/';
const DAEMON_NAME_REGEX = new RegExp(DAEMON_NAME_PATTERN);

// Accepted /health identities: current daemon plus legacy brand eras.
const KABOOM_SERVICE_NAME_REGEX =
  /^(kaboom|gasoline|strum)(-browser-devtools|-agentic-browser|-agentic-devtools|-mcp)?$/;

const LOG_PATH = process.env.KABOOM_KILL_DAEMON_LOG;
const DRY_RUN = process.env.KABOOM_KILL_DAEMON_DRY_RUN === '1';

function logLine(message) {
  if (!LOG_PATH) return;
  try {
    fs.appendFileSync(LOG_PATH, `${message}\n`);
  } catch (_) {
    // Best effort only.
  }
}

function safeExec(command) {
  logLine(`[exec] ${command}`);
  if (DRY_RUN) return;
  try {
    execSync(command, { stdio: 'ignore', shell: true, timeout: 5000 });
  } catch (_) {
    // Best effort only.
  }
}

function safeExecFile(file, args) {
  logLine(`[execFile] ${file} ${args.join(' ')}`.trim());
  if (DRY_RUN) return;
  try {
    execFileSync(file, args, { stdio: 'ignore', timeout: 5000 });
  } catch (_) {
    // Best effort only.
  }
}

function runForceCleanupCommands() {
  // Try installed CLIs first. --force uses the binary's own stop logic.
  for (const binary of ['kaboom-agentic-browser', 'gasoline-mcp', 'kaboom', 'gasoline', 'browser-agent']) {
    safeExecFile(binary, ['--force']);
  }
}

function matchesDaemonCommandLine(cmd) {
  return DAEMON_NAME_REGEX.test(String(cmd || ''));
}

function killByProcessName() {
  if (process.platform === 'win32') {
    // Use wildcards so renamed legacy binaries are cleaned too.
    for (const image of ['kaboom-agentic-browser*.exe', 'gasoline*.exe', 'kaboom*.exe', 'browser-agent*.exe']) {
      safeExec(`taskkill /F /IM ${image} 2>nul`);
    }
    return;
  }

  // Avoid killing this cleanup process even when the repo path contains legacy names.
  const selfPid = process.pid;
  const parentPid = process.ppid;
  const isNodeCmd = (cmd) => /\bnode(\s|$)/.test(cmd) || /\bnpm(\s|$)/.test(cmd);

  logLine(`[pattern] ${DAEMON_NAME_PATTERN}`);
  if (DRY_RUN) return;
  let output = '';
  try {
    output = execSync(`pgrep -af "${DAEMON_NAME_PATTERN}" 2>/dev/null`, { encoding: 'utf8' }).trim();
  } catch (_) {
    output = '';
  }
  if (!output) return;
  for (const line of output.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const [pidPart, ...cmdParts] = trimmed.split(/\s+/);
    const pid = Number(pidPart);
    const cmd = cmdParts.join(' ');
    if (!Number.isFinite(pid) || pid <= 1) continue;
    if (pid === selfPid || pid === parentPid) continue;
    if (isNodeCmd(cmd)) continue;
    // When pgrep reports the command line, re-verify it against the anchored pattern.
    if (cmd && !matchesDaemonCommandLine(cmd)) continue;
    try {
      process.kill(pid, 'SIGKILL');
    } catch (_) {
      // Best effort only.
    }
  }
}

function resolveServiceName(health) {
  if (!health || typeof health !== 'object') return '';
  for (const key of ['service-name', 'service_name', 'service']) {
    if (typeof health[key] === 'string' && health[key].trim()) {
      return health[key].trim();
    }
  }
  return '';
}

function isKaboomDaemonHealth(health) {
  return KABOOM_SERVICE_NAME_REGEX.test(resolveServiceName(health).toLowerCase());
}

function resolveVersion(health) {
  if (!health || typeof health !== 'object') return '';
  return typeof health.version === 'string' ? health.version.trim() : '';
}

function readSelfVersion() {
  try {
    return String(require('../package.json').version || '').trim();
  } catch (_) {
    return '';
  }
}

// Retry budget mirrors the daemon-side election (classifyExistingDaemon): a
// healthy daemon that is momentarily busy (GC pause, disk, a burst of MCP load)
// may miss a single 500ms /health probe. Retrying across ~1.5s before concluding
// "not a healthy same-version daemon" stops a hiccup from re-triggering the very
// respawn storm this gate exists to prevent.
const HEALTH_PROBE_RETRIES = 3;
const HEALTH_PROBE_BACKOFF_MS = 500;

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// probeHealthySameVersion returns true only if the daemon on `port` answers
// /health as our exact version. A definitive answer (any parseable health) is
// returned immediately — same version -> keep, any other version -> real upgrade,
// no retry either way. Only a NON-answer (busy/slow) is retried within the budget.
async function probeHealthySameVersion(port, fetchHealth, selfVersion, sleep) {
  for (let attempt = 0; attempt < HEALTH_PROBE_RETRIES; attempt++) {
    const h = await fetchHealth(port);
    if (isKaboomDaemonHealth(h)) {
      return resolveVersion(h) === selfVersion;
    }
    if (attempt < HEALTH_PROBE_RETRIES - 1) await sleep(HEALTH_PROBE_BACKOFF_MS);
  }
  return false;
}

// Is a HEALTHY daemon of the exact version we are installing already running?
// If so, a (re)install has nothing to replace — killing it only triggers a
// respawn storm (npx reinstalls repeatedly, each SIGTERMing the live daemon and
// blinking the terminal port). A stalled daemon does NOT answer /health, so it
// returns false here and still gets cleaned up. An older/newer version also
// returns false, so real upgrades still replace the old binary.
async function healthySameVersionDaemonRunning(deps = {}) {
  const fetchHealth = deps.fetchHealth || readHealthIdentity;
  const selfVersion = deps.selfVersion || readSelfVersion();
  if (!selfVersion) return false;
  const sleep = deps.sleep || delay;
  const results = await Promise.all(
    KNOWN_PORTS.map((port) => probeHealthySameVersion(port, fetchHealth, selfVersion, sleep))
  );
  return results.some(Boolean);
}

function readHealthIdentity(port, timeoutMs = 500) {
  return new Promise((resolve) => {
    const req = http.get(
      { hostname: '127.0.0.1', port, path: '/health', timeout: timeoutMs },
      (res) => {
        let body = '';
        res.setEncoding('utf8');
        res.on('data', (chunk) => {
          body += chunk;
          if (body.length > 64 * 1024) req.destroy();
        });
        res.on('end', () => {
          if (res.statusCode !== 200) {
            resolve(null);
            return;
          }
          try {
            resolve(JSON.parse(body));
          } catch (_) {
            resolve(null);
          }
        });
      }
    );
    req.on('timeout', () => {
      req.destroy();
      resolve(null);
    });
    req.on('error', () => resolve(null));
  });
}

async function killByKnownPorts(deps = {}) {
  if (process.platform === 'win32') {
    return;
  }
  const fetchHealth = deps.fetchHealth || readHealthIdentity;
  const killPort =
    deps.killPort ||
    ((port) => {
      safeExec(`lsof -ti :${port} 2>/dev/null | xargs kill -9 2>/dev/null`);
    });

  await Promise.all(
    KNOWN_PORTS.map(async (port) => {
      // Only kill processes that answer /health as a Kaboom (or legacy) daemon.
      // Unrelated dev servers on these ports must be left alone.
      const health = await fetchHealth(port);
      if (!isKaboomDaemonHealth(health)) {
        logLine(`[port-skip] ${port}`);
        return;
      }
      logLine(`[port-kill] ${port}`);
      killPort(port);
    })
  );
}

function readPidFromFile(filePath) {
  try {
    const raw = fs.readFileSync(filePath, 'utf8').trim();
    const pid = Number.parseInt(raw, 10);
    if (!Number.isFinite(pid) || pid <= 0) return 0;
    return pid;
  } catch (_) {
    return 0;
  }
}

function killPid(pid) {
  if (!pid || pid <= 0) return;
  logLine(`[pid] ${pid}`);
  if (DRY_RUN) return;

  if (process.platform === 'win32') {
    safeExec(`taskkill /F /PID ${pid} /T 2>nul`);
    return;
  }

  try {
    process.kill(pid, 'SIGKILL');
  } catch (_) {
    // Best effort only.
  }
}

function cleanupPIDFiles() {
  const home = process.env.HOME || process.env.USERPROFILE || os.homedir();
  const modernRoot = path.join(home, '.kaboom', 'run');
  const legacyRoot = path.join(home, '.gasoline', 'run');
  const roots = [modernRoot, legacyRoot];
  if (process.env.XDG_STATE_HOME) {
    roots.push(path.join(process.env.XDG_STATE_HOME, 'kaboom', 'run'));
    roots.push(path.join(process.env.XDG_STATE_HOME, 'gasoline', 'run'));
  }

  const pidFiles = new Set();

  for (const root of roots) {
    try {
      for (const entry of fs.readdirSync(root)) {
        if (entry.startsWith('kaboom-') && entry.endsWith('.pid')) {
          pidFiles.add(path.join(root, entry));
        }
        if (entry.startsWith('gasoline-') && entry.endsWith('.pid')) {
          pidFiles.add(path.join(root, entry));
        }
        if (entry.startsWith('browser-agent-') && entry.endsWith('.pid')) {
          pidFiles.add(path.join(root, entry));
        }
      }
    } catch (_) {
      // Best effort only.
    }
  }

  try {
    for (const entry of fs.readdirSync(home)) {
      if (entry.startsWith('.kaboom-') && entry.endsWith('.pid')) {
        pidFiles.add(path.join(home, entry));
      }
      if (entry.startsWith('.gasoline-') && entry.endsWith('.pid')) {
        pidFiles.add(path.join(home, entry));
      }
      if (entry.startsWith('.browser-agent-') && entry.endsWith('.pid')) {
        pidFiles.add(path.join(home, entry));
      }
    }
  } catch (_) {
    // Best effort only.
  }

  for (const port of KNOWN_PORTS) {
    for (const root of roots) {
      pidFiles.add(path.join(root, `kaboom-${port}.pid`));
      pidFiles.add(path.join(root, `gasoline-${port}.pid`));
      pidFiles.add(path.join(root, `browser-agent-${port}.pid`));
    }
    pidFiles.add(path.join(home, `.kaboom-${port}.pid`));
    pidFiles.add(path.join(home, `.gasoline-${port}.pid`));
    pidFiles.add(path.join(home, `.browser-agent-${port}.pid`));
  }

  for (const pidPath of pidFiles) {
    const pid = readPidFromFile(pidPath);
    killPid(pid);
    try {
      fs.rmSync(pidPath, { force: true });
    } catch (_) {
      // Best effort only.
    }
  }
}

async function cleanupOldDaemons(deps = {}) {
  const env = deps.env || process.env;
  const lifecycleEvent = env.npm_lifecycle_event || '';
  // Only short-circuit on INSTALL. Uninstall must always tear the daemon down,
  // even a healthy same-version one, because the binary is going away.
  const isInstall = lifecycleEvent === 'preinstall' || lifecycleEvent === 'install';

  if (isInstall && (await healthySameVersionDaemonRunning(deps))) {
    logLine('[skip-cleanup] healthy daemon already on target version');
    return;
  }

  const runForce = deps.runForceCleanupCommands || runForceCleanupCommands;
  const killByName = deps.killByProcessName || killByProcessName;
  const killPorts = deps.killByKnownPorts || killByKnownPorts;
  const cleanPids = deps.cleanupPIDFiles || cleanupPIDFiles;

  runForce();
  killByName();
  await killPorts(deps);
  cleanPids();
}

if (require.main === module) {
  cleanupOldDaemons().catch(() => {
    // Best effort only — never fail the npm lifecycle hook.
  });
}

module.exports = {
  cleanupOldDaemons,
  KNOWN_PORTS,
  matchesDaemonCommandLine,
  isKaboomDaemonHealth,
  killByKnownPorts,
  healthySameVersionDaemonRunning,
  resolveVersion,
};
