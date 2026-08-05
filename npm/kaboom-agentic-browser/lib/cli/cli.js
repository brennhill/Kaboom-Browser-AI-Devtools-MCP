// Purpose: Route kaboom-agentic-browser wrapper commands to install/config/doctor/uninstall flows.
// Why: Keeps CLI behavior consistent across client setup paths and avoids shell-script drift.
// Docs: docs/features/feature/enhanced-cli-config/index.md
// cli.js — CLI command handler for kaboom-agentic-browser management commands.
// Invoked by the shell wrapper when --install, --config, --doctor, etc. are passed.

const os = require('os');
const config = require('../config/config');
const output = require('./output');
const install = require('../installation/install');
const extension = require('../installation/extension');
const browser = require('../browser/browser');
const health = require('../daemon/health');
const daemon = require('../daemon/daemon');
const skills = require('../installation/skills');
const doctor = require('../daemon/doctor');
const uninstall = require('../installation/uninstall');

// How long the post-install / --connect loop waits for the extension to appear.
const CONNECT_WAIT_MS = 30000;

// Interactive install = a human is watching (a TTY). The daemon-start, connect
// wait, and browser-page open only make sense — and are only safe to trigger —
// for an interactive install; scripted/CI installs must stay side-effect-free
// (no lingering daemon, no windows popping open, no blocking wait).
function isInteractiveInstall() {
  return !!process.stdout.isTTY;
}

// Watch the loop only for an interactive install the user hasn't opted out of.
function shouldWatchConnect() {
  return isInteractiveInstall() && !health.connectWaitDisabled(process.env);
}

/**
 * Ensure the daemon is up, then wait for the browser extension to connect,
 * printing live progress and a targeted next step. Shared by post-install and
 * the standalone --connect command.
 */
async function watchExtensionConnect({ port = health.DEFAULT_PORT, extensionDir, timeoutMs = CONNECT_WAIT_MS } = {}) {
  await daemon.ensureDaemon({ port });

  console.log(`\n⏳ Waiting for the browser extension to connect on port ${port} (Ctrl-C to skip)…`);

  // First Ctrl-C aborts the wait (a real "skip" that continues to the ready
  // message), not a hard kill. `once` restores default SIGINT afterward, so a
  // second Ctrl-C still force-quits.
  const controller = new AbortController();
  const onSigint = () => controller.abort();
  process.once('SIGINT', onSigint);

  let lastPhase = null;
  let result;
  try {
    result = await health.waitForExtension({
      port,
      timeoutMs,
      signal: controller.signal,
      onState: ({ phase }) => {
        if (phase === lastPhase) return;
        lastPhase = phase;
        if (phase === 'daemon_unreachable') {
          console.log('   … waiting for the Kaboom server to come up');
        } else if (phase === 'waiting_extension') {
          console.log('   … server is up — load the extension in your browser to finish');
        }
      },
    });
  } finally {
    process.removeListener('SIGINT', onSigint);
  }

  if (result.connected) {
    console.log('✅ Browser extension connected — Kaboom is fully wired up!');
  } else if (result.reason === 'aborted') {
    console.log('⏭️  Skipped waiting — run "kaboom-agentic-browser --connect" later to confirm the extension connected.');
  } else {
    console.log(`⚠️  ${health.connectHint(result.lastPhase, { port, extensionDir })}`);
  }
  return result;
}

// Write a JSON-RPC error to stdout so MCP clients get a clean protocol-level error
function writeMcpError(message) {
  const errorResponse = JSON.stringify({
    jsonrpc: '2.0',
    id: null,
    error: {
      code: -32603,
      message: message,
      data: { isError: true },
    },
  });
  process.stdout.write(errorResponse + '\n');
}

function showConfigCommand() {
  const mcp = install.generateDefaultConfig();
  console.log('📋 Kaboom Agentic Browser Configuration\n');
  console.log('Add this to your AI assistant settings:\n');
  console.log(JSON.stringify(mcp, null, 2));
  console.log('\n📍 Supported Clients:\n');

  for (const def of config.CLIENT_DEFINITIONS) {
    const detected = config.isClientInstalled(def);
    const icon = detected ? '✅' : '⚪';

    if (def.type === 'cli') {
      console.log(`${icon} ${def.name} (via ${def.detectCommand} CLI)`);
    } else {
      const cfgPath = config.getClientConfigPath(def);
      if (cfgPath) {
        console.log(`${icon} ${def.name}`);
        console.log(`   ${cfgPath}`);
      } else {
        console.log(`⚪ ${def.name} (not available on this platform)`);
      }
    }
    console.log('');
  }

  console.log('Run: kaboom-agentic-browser --install   (auto-installs to all detected clients)');
  process.exit(0);
}

async function installCommand(options) {
  try {
    const result = install.executeInstall(options);

    if (result.success) {
      if (options.dryRun) {
        console.log(`ℹ️  Dry run: No files will be written\n`);
      }
      console.log(output.installResult(result));
      // Reveal the extension folder so "Load unpacked" is one selection away.
      // Interactive installs only, so scripted/CI runs never pop a file-manager
      // window (matches the browser-open gate below). Best-effort and opt-out via
      // KABOOM_NO_OPEN; the exact path is printed above regardless.
      if (!options.dryRun && isInteractiveInstall() && extension.openExtensionDir(result.extensionDir)) {
        console.log('📂 Opened the extension folder for you — just select it in "Load unpacked".');
      }
      // Also open the browser straight to its extensions page (the folder opener
      // can't do that — it's a browser-internal URL). Interactive installs only,
      // so scripted/CI runs never pop a browser window; same KABOOM_NO_OPEN opt-out.
      if (!options.dryRun && isInteractiveInstall()) {
        const target = browser.detectExtensionsTarget();
        if (browser.openExtensionsPage(target)) {
          console.log(`🌐 Opened ${target.name} at ${target.url} — click "Load unpacked" there.`);
        }
      }
      if (!options.dryRun) {
        const skillInstall = await skills.installBundledSkills({
          verbose: options.verbose,
          skillsRepo: options.skillsRepo,
          skillsRef: options.skillsRef,
          skillsPath: options.skillsPath,
          skillsManifestPath: options.skillsManifestPath,
          skillsDir: options.skillsDir,
          skillsNoFallback: options.skillsNoFallback,
        });
        if (!skillInstall.skipped) {
          const s = skillInstall.summary;
          console.log(
            `🧠 Skills installed (${skillInstall.agents.join(', ')} / ${skillInstall.scope}): ` +
            `source=${skillInstall.source}, created=${s.created}, updated=${s.updated}, unchanged=${s.unchanged}, ` +
            `skipped=${s.skipped_user_owned}, errors=${s.errors}`
          );
          for (const warning of skillInstall.warnings || []) {
            console.warn(`⚠️  ${warning}`);
          }
        }
        // Start the daemon so the extension has something to connect to — parity
        // with the curl|sh installer's startDaemonSilently. ensureDaemon reuses a
        // healthy daemon instead of double-binding; opt out with KABOOM_NO_DAEMON.
        // The blocking connect confirmation is interactive-only so scripted/CI
        // installs never hang; they get guidance to run --connect/--doctor.
        if (shouldWatchConnect()) {
          await watchExtensionConnect({ extensionDir: result.extensionDir });
        } else {
          await daemon.ensureDaemon();
          console.log('👉 After loading the extension, run "kaboom-agentic-browser --connect" (or --doctor) to confirm it connected.');
        }
        console.log('✨ Kaboom Agentic Browser is ready to use!');
      }
      process.exit(0);
    } else {
      console.error(output.error('Installation failed'));
      result.errors.forEach(err => {
        if (typeof err === 'string') {
          console.error(`  ${err}`);
        } else {
          console.error(`  ${err.name}: ${err.message}`);
          if (err.recovery) {
            console.error(`  Recovery: ${err.recovery}`);
          }
        }
      });
      process.exit(1);
    }
  } catch (err) {
    console.error(err.format ? err.format() : `Error: ${err.message}`);
    process.exit(1);
  }
}

async function updateCommand(options) {
  try {
    const cleanupResult = uninstall.executeUninstall({
      dryRun: options.dryRun,
      verbose: options.verbose,
    });

    if (options.dryRun || cleanupResult.removed.length > 0 || cleanupResult.skillCleanup?.removed > 0) {
      console.log(output.uninstallResult(cleanupResult));
      if (options.dryRun) {
        console.log('');
      }
    }

    await installCommand(options);
  } catch (err) {
    console.error(err.format ? err.format() : `Error: ${err.message}`);
    process.exit(1);
  }
}

async function doctorCommand(verbose) {
  try {
    const report = await doctor.runDiagnostics(verbose);
    console.log(output.diagnosticReport(report));
    // Exit non-zero only for a HARD failure the user must fix: a missing/broken
    // platform binary means the tool cannot run at all, so scripts and CI should
    // catch it. SOFT/expected post-install states (daemon not running, extension
    // not yet connected) stay exit 0 so they never fail an install script.
    const binaryFailed = !!(report.binary && report.binary.ok === false);
    process.exit(binaryFailed ? 1 : 0);
  } catch (err) {
    console.error(err.format ? err.format() : `Error: ${err.message}`);
    process.exit(1);
  }
}

async function connectCommand() {
  try {
    const ext = extension.resolveExtensionDir();
    const result = await watchExtensionConnect({ extensionDir: ext.dir });
    process.exit(result.connected ? 0 : 1);
  } catch (err) {
    console.error(err.format ? err.format() : `Error: ${err.message}`);
    process.exit(1);
  }
}

function uninstallCommand(dryRun, verbose) {
  try {
    const result = uninstall.executeUninstall({ dryRun, verbose });

    if (dryRun) {
      console.log(`ℹ️  Dry run: No files will be modified\n`);
    }
    console.log(output.uninstallResult(result));
    process.exit(0);
  } catch (err) {
    console.error(err.format ? err.format() : `Error: ${err.message}`);
    process.exit(1);
  }
}

function showHelp() {
  console.log('Kaboom Agentic Browser Server\n');
  console.log('Usage: kaboom-agentic-browser [command] [options]\n');
  console.log('Commands:');
  console.log('  --config, -c          Show MCP configuration and detected clients');
  console.log('  --install, -i [tool]  Auto-install to detected clients, or a specific tool');
  console.log('  --update [tool]       Clean reinstall Kaboom for detected clients or one specific tool');
  console.log('  --doctor              Run diagnostics (configs + live daemon/extension/Node)');
  console.log('  --connect             Wait for the browser extension to connect, with live status');
  console.log('  --uninstall           Remove Kaboom from all clients');
  console.log('  --help, -h            Show this help message\n');
  console.log('Supported clients:');
  console.log('  Claude Code           via claude CLI (mcp add-json)');
  console.log('  Claude Desktop        config file');
  console.log('  Cursor                config file');
  console.log('  Windsurf              config file');
  console.log('  VS Code               config file');
  console.log('  Gemini CLI            config file');
  console.log('  OpenCode              config file');
  console.log('  Antigravity           config file');
  console.log('  Zed                   config file');
  console.log('  Codex CLI             config file (TOML)\n');
  console.log('Install auto-approves all Kaboom tools where a client supports it via config');
  console.log('(Claude Code, Gemini, OpenCode, Zed, Codex), so you are never prompted.');
  console.log('Cursor, Windsurf, VS Code, Claude Desktop, and Antigravity need an in-app');
  console.log('approval (no config option); the installer reports which is which.\n');
  console.log('Tool aliases for --install <tool>:');
  console.log('  claude, claude-desktop, cursor, windsurf, vscode, gemini, opencode,');
  console.log('  antigravity, zed, codex\n');
  console.log('Options (with --install):');
  console.log('  --dry-run             Preview changes without writing');
  console.log('  --env KEY=VALUE       Add environment variables to config (multiple allowed)');
  console.log('  --skills-repo VALUE   Skill source repo (owner/repo or GitHub URL)');
  console.log('  --skills-ref VALUE    Git ref when loading skills from --skills-repo');
  console.log('  --skills-path VALUE   Repo path containing skill folders (optional)');
  console.log('  --skills-manifest VALUE Repo path to skills manifest JSON (for example skills/skills.json)');
  console.log('  --skills-dir PATH     Local skills directory override');
  console.log('  --skills-no-fallback  Do not fall back to bundled skills if remote fetch fails');
  console.log('  --verbose             Show detailed operation logs\n');
  console.log('Options (with --uninstall):');
  console.log('  --dry-run             Preview changes without writing');
  console.log('  --verbose             Show detailed operation logs\n');
  console.log('Examples:');
  console.log('  kaboom-agentic-browser --install                # Install to all detected clients');
  console.log('  kaboom-agentic-browser --install gemini          # Install to Gemini CLI only');
  console.log('  kaboom-agentic-browser --install opencode        # Install to OpenCode only');
  console.log('  kaboom-agentic-browser --install --dry-run      # Preview without changes');
  console.log('  kaboom-agentic-browser --install --env DEBUG=1  # Install with env vars');
  console.log('  kaboom-agentic-browser --install --skills-repo brennhill/kaboom-skills');
  console.log('  kaboom-agentic-browser --update                 # Clean reinstall Kaboom');
  console.log('  kaboom-agentic-browser --config                 # Show config and detected clients');
  console.log('  kaboom-agentic-browser --doctor                 # Check config + live daemon/extension health');
  console.log('  kaboom-agentic-browser --connect                # Wait for the extension to connect');
  console.log('  kaboom-agentic-browser --uninstall              # Remove from all clients\n');
  process.exit(0);
}

function parseSingleValueFlag(args, flagName) {
  const index = args.indexOf(flagName);
  if (index === -1) return null;
  const next = args[index + 1];
  if (!next || next.startsWith('--')) {
    throw new Error(`Missing value for ${flagName}`);
  }
  return next;
}

function parseSkillInstallOptions(args) {
  return {
    skillsRepo: parseSingleValueFlag(args, '--skills-repo'),
    skillsRef: parseSingleValueFlag(args, '--skills-ref'),
    skillsPath: parseSingleValueFlag(args, '--skills-path'),
    skillsManifestPath: parseSingleValueFlag(args, '--skills-manifest'),
    skillsDir: parseSingleValueFlag(args, '--skills-dir'),
    skillsNoFallback: args.includes('--skills-no-fallback'),
  };
}

function parseInstallLikeCommandOptions(args, primaryFlag, secondaryFlag) {
  const commandIdx = args.indexOf(primaryFlag) !== -1 ? args.indexOf(primaryFlag) : args.indexOf(secondaryFlag);
  let targetTool = null;
  const nextArg = args[commandIdx + 1];
  if (nextArg && !nextArg.startsWith('--')) {
    targetTool = nextArg;
  }

  const envVars = {};
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--env' && i + 1 < args.length) {
      const parsed = config.parseEnvVar(args[i + 1]);
      envVars[parsed.key] = parsed.value;
    }
  }

  const skillOptions = parseSkillInstallOptions(args);
  return {
    dryRun: args.includes('--dry-run'),
    envVars,
    verbose: args.includes('--verbose'),
    targetTool,
    ...skillOptions,
  };
}

async function main() {
  const args = process.argv.slice(2);
  const verbose = args.includes('--verbose');
  const dryRun = args.includes('--dry-run');

  // Config command
  if (args.includes('--config') || args.includes('-c')) {
    showConfigCommand();
    return;
  }

  // Install command
  if (args.includes('--install') || args.includes('-i')) {
    try {
      await installCommand(parseInstallLikeCommandOptions(args, '--install', '-i'));
    } catch (err) {
      console.error(output.error(err.message, 'Run kaboom-agentic-browser --help for usage.'));
      process.exit(1);
    }
    return;
  }

  // Update command
  if (args.includes('--update')) {
    try {
      await updateCommand(parseInstallLikeCommandOptions(args, '--update', '--update'));
    } catch (err) {
      console.error(output.error(err.message, 'Run kaboom-agentic-browser --help for usage.'));
      process.exit(1);
    }
    return;
  }

  // Doctor command
  if (args.includes('--doctor')) {
    await doctorCommand(verbose);
    return;
  }

  // Connect command — wait for the browser extension to connect.
  if (args.includes('--connect')) {
    await connectCommand();
    return;
  }

  // Uninstall command
  if (args.includes('--uninstall')) {
    uninstallCommand(dryRun, verbose);
    return;
  }

  // Help command
  if (args.includes('--help') || args.includes('-h')) {
    showHelp();
    return;
  }

  // Version (print and exit)
  if (args.includes('--version') || args.includes('-v')) {
    const pkg = require('../../package.json');
    console.log(`kaboom-agentic-browser v${pkg.version}`);
    process.exit(0);
  }

  // If we get here with no recognized flags, show help
  console.error('Unknown command. Run kaboom-agentic-browser --help for usage.');
  process.exit(1);
}

main().catch((err) => {
  console.error(err.format ? err.format() : `Error: ${err.message}`);
  process.exit(1);
});
