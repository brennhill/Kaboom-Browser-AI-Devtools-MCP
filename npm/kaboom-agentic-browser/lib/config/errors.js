// Purpose: Implement errors.js behavior for npm wrapper command flows.
// Why: Keeps distribution-channel behavior consistent and supportable.
// Docs: docs/features/feature/enhanced-cli-config/index.md

/**
 * Error classes and message catalog for the Kaboom CLI
 */

/**
 * Base error class for all Kaboom CLI errors
 */
class KaboomError extends Error {
  constructor(message, recovery) {
    super(message);
    this.name = 'KaboomError';
    this.recovery = recovery;
  }

  format() {
    let output = `❌ Error: ${this.message}`;
    if (this.recovery) {
      output += `\n   ${this.recovery}`;
    }
    return output;
  }
}

class PermissionError extends KaboomError {
  constructor(path) {
    super(
      `Permission denied writing ${path}`,
      `Try: sudo kaboom-agentic-browser --install\nOr: Check permissions with: ls -la ${path.split('/').slice(0, -1).join('/')}`
    );
    this.name = 'PermissionError';
  }
}

class InvalidJSONError extends KaboomError {
  constructor(path, lineNumber, errorMessage) {
    const msg = `Invalid JSON in ${path}${lineNumber ? ` at line ${lineNumber}` : ''}\n   ${errorMessage}`;
    super(
      msg,
      `Fix options:\n   1. Manually edit: code ${path}\n   2. Restore from backup and try --install again\n   3. Run: kaboom-agentic-browser --doctor (for more info)`
    );
    this.name = 'InvalidJSONError';
  }
}

class BinaryNotFoundError extends KaboomError {
  constructor(expectedPath) {
    super(
      `Kaboom binary not found at ${expectedPath}`,
      `Reinstall: npm install -g kaboom-agentic-browser@latest\nOr build from source: go build ./cmd/browser-agent`
    );
    this.name = 'BinaryNotFoundError';
  }
}

class InvalidEnvFormatError extends KaboomError {
  constructor(envStr) {
    super(
      `Invalid env format "${envStr}". Expected: KEY=VALUE`,
      `Examples of valid formats:\n   - --env DEBUG=1\n   - --env KABOOM_SERVER=http://localhost:7890\n   - --env LOG_LEVEL=info`
    );
    this.name = 'InvalidEnvFormatError';
  }
}

class EnvWithoutInstallError extends KaboomError {
  constructor() {
    super(
      '--env only works with --install',
      'Usage: kaboom-agentic-browser --install --env KEY=VALUE'
    );
    this.name = 'EnvWithoutInstallError';
  }
}

class ConfigValidationError extends KaboomError {
  constructor(errors) {
    super(
      'Config validation failed',
      `Issues:\n${errors.map(e => `   - ${e}`).join('\n')}`
    );
    this.name = 'ConfigValidationError';
  }
}

class FileSizeError extends KaboomError {
  constructor(path, size) {
    super(
      `Config file too large: ${(size / 1024 / 1024).toFixed(2)}MB`,
      `Max size: 1MB. File: ${path}`
    );
    this.name = 'FileSizeError';
  }
}

module.exports = {
  KaboomError,
  PermissionError,
  InvalidJSONError,
  BinaryNotFoundError,
  InvalidEnvFormatError,
  EnvWithoutInstallError,
  ConfigValidationError,
  FileSizeError,
};
