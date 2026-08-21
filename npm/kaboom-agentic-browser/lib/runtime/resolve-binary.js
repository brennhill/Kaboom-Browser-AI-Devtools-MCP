// resolve-binary.js — Single source of truth for locating a Kaboom platform binary.
// Purpose: Resolve the Go binary from an explicit override, the source-tree dist
//   build, or the platform optionalDependency — and fail loudly otherwise.
// Why: PATH probing silently bound the launcher to whatever kaboom happened to be
//   installed elsewhere on the machine. A missing optionalDependency is an install
//   fault and must surface as one, not resolve to a stranger's binary.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const path = require('path');
const fs = require('fs');

const PLATFORM_MAP = { darwin: 'darwin', linux: 'linux', win32: 'win32' };
const ARCH_MAP = { x64: 'x64', arm64: 'arm64' };
const PKG_SCOPE = '@brennhill/kaboom-agentic-browser';

class BinaryNotFoundError extends Error {
  constructor(message) {
    super(message);
    this.name = 'BinaryNotFoundError';
  }
}

/**
 * A binary shipped inside the platform optionalDependency.
 * `distName` differs per binary: the server build is platform-suffixed by the
 * Makefile ($(BINARY_NAME)-<platformKey>), the hooks build is not.
 */
const SERVER_BINARY = {
  command: 'kaboom-agentic-browser',
  overrideKey: 'KABOOM_BINARY_PATH',
  distName: (info) => `kaboom-agentic-browser-${info.platformKey}${info.ext}`,
};

const HOOKS_BINARY = {
  command: 'kaboom-hooks',
  overrideKey: 'KABOOM_HOOKS_BINARY_PATH',
  distName: (info) => `kaboom-hooks${info.ext}`,
};

function detectPlatform({ platform = process.platform, arch = process.arch } = {}) {
  const platformName = PLATFORM_MAP[platform];
  const archName = ARCH_MAP[arch];
  if (!platformName || !archName) return null;

  // The win32 x64 build runs on arm64 under emulation; no arm64 Windows build ships.
  const effectiveArch = platformName === 'win32' ? 'x64' : archName;
  const ext = platformName === 'win32' ? '.exe' : '';
  const platformKey = `${platformName}-${effectiveArch}`;

  return { platform: platformName, arch: effectiveArch, platformKey, ext, pkgName: `${PKG_SCOPE}-${platformKey}` };
}

/**
 * Ordered candidate paths. PATH is deliberately absent — see the file header.
 * The source-tree dist build comes first so a fresh `make build` is what runs
 * during development instead of a stale installed package.
 */
function binaryCandidates(spec, info, packageRoot) {
  const candidates = [];

  // Dev build — repo-root dist/. The package dir's parent is "npm" in the source
  // tree and "node_modules" once installed, so a dist/ planted in a user's
  // project can never be reached.
  if (path.basename(path.dirname(packageRoot)) === 'npm') {
    candidates.push(path.resolve(packageRoot, '..', '..', 'dist', spec.distName(info)));
  }

  // The platform optionalDependency, at the three depths npm may place it:
  // nested, hoisted beside the package, or hoisted one level further.
  const binaryName = `${spec.command}${info.ext}`;
  candidates.push(
    path.resolve(packageRoot, 'node_modules', info.pkgName, 'bin', binaryName),
    path.resolve(packageRoot, '..', info.pkgName, 'bin', binaryName),
    path.resolve(packageRoot, '..', '..', info.pkgName, 'bin', binaryName)
  );

  return candidates;
}

function missingBinaryMessage(spec, info, packageRoot) {
  if (!info) {
    return `Unsupported platform: ${process.platform}-${process.arch}. Kaboom ships binaries for darwin, linux, and win32 on x64/arm64.`;
  }
  return [
    `Kaboom binary "${spec.command}" not found for ${info.platformKey}.`,
    `Expected it in the platform package ${info.pkgName}, which installs as an optionalDependency of kaboom-agentic-browser.`,
    'It is missing — the usual cause is an install run with --no-optional, --omit=optional, or a locked-down registry that skipped the platform package.',
    'Repair: npm install -g kaboom-agentic-browser@latest (without --no-optional).',
    `To point at a binary yourself, set ${spec.overrideKey}=/absolute/path/to/${spec.command}.`,
    `Searched: ${binaryCandidates(spec, info, packageRoot).join(', ')}`,
  ].join(' ');
}

/**
 * Resolve an absolute path to the binary, or throw BinaryNotFoundError.
 * Never returns a bare command name and never consults PATH.
 */
function resolveBinary({
  spec,
  env = process.env,
  platform = process.platform,
  arch = process.arch,
  packageRoot = path.resolve(__dirname, '..', '..'),
  existsFn = fs.existsSync,
} = {}) {
  // 1. Explicit operator override. A set-but-missing override is a real failure:
  // silently falling through would run a binary the operator did not choose.
  const override = env[spec.overrideKey];
  if (override) {
    const resolved = path.resolve(override);
    if (!existsFn(override) && !existsFn(resolved)) {
      throw new BinaryNotFoundError(
        `${spec.overrideKey} is set to "${override}" but no file exists there. Unset it or point it at a real ${spec.command} binary.`
      );
    }
    return resolved;
  }

  const info = detectPlatform({ platform, arch });
  if (!info) throw new BinaryNotFoundError(missingBinaryMessage(spec, null, packageRoot));

  for (const candidate of binaryCandidates(spec, info, packageRoot)) {
    if (existsFn(candidate)) return candidate;
  }

  throw new BinaryNotFoundError(missingBinaryMessage(spec, info, packageRoot));
}

module.exports = {
  BinaryNotFoundError,
  HOOKS_BINARY,
  SERVER_BINARY,
  binaryCandidates,
  detectPlatform,
  resolveBinary,
};
