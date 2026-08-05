// Purpose: Implement output.js behavior for npm wrapper command flows.
// Why: Keeps distribution-channel behavior consistent and supportable.
// Docs: docs/features/feature/enhanced-cli-config/index.md

/**
 * Output formatters for the Kaboom CLI
 */

/**
 * Format success message
 */
function success(message, details) {
  let output = `✅ ${message}`;
  if (details) {
    output += `\n   ${details}`;
  }
  return output;
}

/**
 * Format error message
 */
function error(message, recovery) {
  let output = `❌ ${message}`;
  if (recovery) {
    output += `\n   ${recovery}`;
  }
  return output;
}

/**
 * Format warning message
 */
function warning(message, details) {
  let output = `⚠️  ${message}`;
  if (details) {
    output += `\n   ${details}`;
  }
  return output;
}

/**
 * Format info message
 */
function info(message, details) {
  let output = `ℹ️  ${message}`;
  if (details) {
    output += `\n   ${details}`;
  }
  return output;
}

/**
 * Format JSON diff for dry-run
 */
function jsonDiff(before, after) {
  const beforeStr = JSON.stringify(before, null, 2);
  const afterStr = JSON.stringify(after, null, 2);

  return `ℹ️  Dry run: No files will be written\n\nBefore:\n${beforeStr}\n\nAfter:\n${afterStr}`;
}

/**
 * Format install result
 */
function installResult(result) {
  let output = '';
  const installed = result.installed || result.updated || [];
  const total = result.total || 5;

  if (installed.length > 0) {
    output += `✅ ${installed.length}/${total} clients updated:\n`;
    installed.forEach(entry => {
      if (entry.method === 'cli') {
        output += `   ✅ ${entry.name} (via CLI)\n`;
      } else {
        output += `   ✅ ${entry.name} (at ${entry.path})\n`;
      }
    });

    // Auto-approve transparency: report EXACTLY which clients had all Kaboom
    // tools trusted via config (no more prompts), which can only be done in-app,
    // and any that failed — so the default-ON behavior is never a surprise.
    const approved = installed
      .filter(e => e.autoApprove === 'applied' || e.autoApprove === 'would-apply' || e.autoApprove === 'unchanged')
      .map(e => e.name);
    const uiOnly = installed.filter(e => e.autoApprove === 'ui-only').map(e => e.name);
    const failed = installed.filter(e => e.autoApprove === 'failed');
    if (approved.length > 0 || uiOnly.length > 0 || failed.length > 0) {
      output += '\n🔓 Tool auto-approve (Kaboom trusts its own tools — no approval prompts):\n';
      if (approved.length > 0) {
        output += `   ✅ Auto-approved via config: ${approved.join(', ')}\n`;
      }
      if (uiOnly.length > 0) {
        output += `   🖱  Manual in-app approval (no config option): ${uiOnly.join(', ')}\n`;
      }
      failed.forEach(e => {
        output += `   ⚠️  ${e.name}: could not write auto-approve (${e.autoApproveError || 'unknown error'})\n`;
      });
    }
  }

  if (result.errors && result.errors.length > 0) {
    output += '\n❌ Errors:\n';
    result.errors.forEach(err => {
      if (typeof err === 'string') {
        output += `   ❌ ${err}\n`;
      } else {
        output += `   ❌ ${err.name}: ${err.message}\n`;
      }
    });
  }

  if (result.notFound && result.notFound.length > 0) {
    output += `\nℹ️  Not configured in: ${result.notFound.join(', ')}\n`;
  }

  // The browser extension is the one step the installer cannot click for the
  // user, so give them the EXACT folder to load — not a vague "the extension".
  if (result.extensionDir) {
    output += '\n🧩 LOAD THE BROWSER EXTENSION (one-time):\n';
    output += '   1) Open  chrome://extensions  (or brave://extensions, edge://extensions)\n';
    output += '   2) Turn on "Developer mode" (top-right toggle)\n';
    output += '   3) Click "Load unpacked" and select this exact folder:\n\n';
    output += `        ${result.extensionDir}\n\n`;
    if (result.extensionExists === false) {
      output += '   ⚠️  That folder is not present yet — reinstall, or set KABOOM_EXTENSION_DIR to the\n';
      output += '       unpacked extension you want to load.\n';
    }
  }

  return output;
}

/**
 * Format doctor diagnostic report
 */
function diagnosticReport(report) {
  let output = '\n📋 Kaboom Diagnostic Report\n\n';

  report.tools.forEach(tool => {
    if (tool.status === 'ok') {
      output += `✅ ${tool.name}\n`;
      if (tool.type === 'cli') {
        output += `   Configured via CLI - Ready\n\n`;
      } else {
        output += `   ${tool.path} - Configured and ready\n\n`;
      }
    } else if (tool.status === 'error') {
      output += `❌ ${tool.name}\n`;
      if (tool.path) {
        output += `   ${tool.path}\n`;
      }
      if (tool.issues && tool.issues.length > 0) {
        tool.issues.forEach(issue => {
          output += `   Issue: ${issue}\n`;
        });
      }
      if (tool.suggestions && tool.suggestions.length > 0) {
        tool.suggestions.forEach(suggestion => {
          output += `   Fix: ${suggestion}\n`;
        });
      }
      output += '\n';
    } else if (tool.status === 'info') {
      output += `⚪ ${tool.name}\n`;
      if (tool.issues && tool.issues.length > 0) {
        tool.issues.forEach(issue => {
          output += `   ${issue}\n`;
        });
      }
      output += '\n';
    } else if (tool.status === 'warning') {
      output += `⚠️  ${tool.name}\n`;
      if (tool.path) {
        output += `   ${tool.path}\n`;
      }
      if (tool.issues && tool.issues.length > 0) {
        tool.issues.forEach(issue => {
          output += `   Issue: ${issue}\n`;
        });
      }
      if (tool.suggestions && tool.suggestions.length > 0) {
        tool.suggestions.forEach(suggestion => {
          output += `   Suggestion: ${suggestion}\n`;
        });
      }
      output += '\n';
    }
  });

  if (report.binary) {
    if (report.binary.ok) {
      output += `✅ Binary Check\n`;
      output += `   Kaboom binary found at ${report.binary.path}\n`;
      if (report.binary.version) {
        output += `   Version: ${report.binary.version}\n`;
      }
    } else {
      output += `❌ Binary Check\n`;
      output += `   ${report.binary.error}\n`;
    }
    output += '\n';
  }

  if (report.port) {
    if (report.port.available) {
      output += `✅ Port ${report.port.port}\n`;
      output += `   Default port is available\n`;
    } else {
      output += `⚠️  Port ${report.port.port}\n`;
      output += `   ${report.port.error}\n`;
      output += `   Suggestion: Use --port ${report.port.port + 1} or kill the process using the port\n`;
    }
  }

  // Node runtime the launcher/CLI runs under.
  if (report.node) {
    if (report.node.ok) {
      output += `✅ Node.js ${report.node.version}\n`;
    } else {
      output += `⚠️  Node.js ${report.node.version}\n`;
      output += `   Kaboom needs Node ${report.node.minMajor}+; upgrade Node to run the CLI reliably.\n`;
    }
  }

  // Live daemon + browser-extension status (the actual data path).
  if (report.daemon) {
    if (report.daemon.reachable) {
      const ver = report.daemon.version ? ` (v${report.daemon.version})` : '';
      output += `✅ Kaboom daemon${ver}\n`;
      output += `   Responding on port ${report.daemon.port}\n`;
    } else {
      output += `⚪ Kaboom daemon\n`;
      output += `   Not running — it starts when your AI client launches Kaboom (or run --install).\n`;
    }
  }

  // Restart churn: repeated daemon restarts drop the extension's connection.
  if (report.restarts && report.restarts.available && report.restarts.restarts >= 5) {
    output += `⚠️  Daemon restart churn\n`;
    output += `   The daemon restarted ${report.restarts.restarts} times in the last ${report.restarts.windowMinutes} min — each restart drops the extension's WebSocket ("connection died").\n`;
    output += `   Usual causes: repeated installs / version switches, or two Kaboom binaries fighting for the port. Keep a single binary and re-check.\n`;
  }

  if (report.extension) {
    if (report.extension.connected) {
      output += `✅ Browser extension\n`;
      output += `   Connected and streaming to the daemon\n`;
    } else if (report.daemon && report.daemon.reachable) {
      output += `⚠️  Browser extension\n`;
      output += `   Not connected — open chrome://extensions, "Load unpacked", select the Kaboom folder, and enable it.\n`;
    } else {
      output += `⚪ Browser extension\n`;
      output += `   Can't check until the daemon is running.\n`;
    }
  }

  output += `\n${report.summary}\n`;
  return output;
}

/**
 * Format uninstall result
 */
function uninstallResult(result) {
  let output = '';

  if (result.removed.length > 0) {
    output += `✅ Removed from ${result.removed.length} client${result.removed.length === 1 ? '' : 's'}:\n`;
    result.removed.forEach(entry => {
      if (entry.method === 'cli') {
        output += `   ✅ ${entry.name} (via CLI)\n`;
      } else {
        output += `   ✅ ${entry.name} (removed from ${entry.path})\n`;
      }
    });
  } else {
    output += `ℹ️  Kaboom not configured in any clients\n`;
  }

  if (result.notConfigured && result.notConfigured.length > 0) {
    output += `\nℹ️  Not configured in: ${result.notConfigured.join(', ')}\n`;
  }

  if (result.errors && result.errors.length > 0) {
    output += '\n❌ Errors:\n';
    result.errors.forEach(err => {
      output += `   ${err}\n`;
    });
  }

  return output;
}

module.exports = {
  success,
  error,
  warning,
  info,
  jsonDiff,
  installResult,
  diagnosticReport,
  uninstallResult,
};
