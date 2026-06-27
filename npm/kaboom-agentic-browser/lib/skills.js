/**
 * Purpose: Install and manage bundled Kaboom skills for supported agent targets.
 * Why: Ensures consistent managed skill availability from npm installs and wrapper --install flows.
 * Docs: docs/features/feature/enhanced-cli-config/index.md
 *
 * Skill installer for Kaboom MCP.
 * Supports local bundled skills and optional GitHub subrepo sources.
 * Targets Claude, Codex, and Gemini skill directory layouts.
 */

const fs = require('fs');
const https = require('https');
const os = require('os');
const path = require('path');

const MANAGED_MARKER = '<!-- kaboom-managed-skill';
const LEGACY_MANAGED_MARKERS = ['<!-- kaboom-managed-skill', '<!-- gasoline-managed-skill', '<!-- strum-managed-skill'];
const MANAGED_MARKERS = [MANAGED_MARKER, ...LEGACY_MANAGED_MARKERS];
const BUNDLED_SKILLS_DIR = path.join(__dirname, '..', 'skills');
const DEFAULT_AGENTS = ['claude', 'codex', 'gemini'];
const LEGACY_PREFIXES = ['kaboom-', 'gasoline-', 'strum-'];

function parseBoolEnv(name) {
  const value = process.env[name];
  if (!value) return false;
  const normalized = String(value).trim().toLowerCase();
  return normalized === '1' || normalized === 'true' || normalized === 'yes' || normalized === 'on';
}

function parseBoolValue(value) {
  if (typeof value === 'boolean') return value;
  if (value === null || value === undefined) return false;
  const normalized = String(value).trim().toLowerCase();
  return normalized === '1' || normalized === 'true' || normalized === 'yes' || normalized === 'on';
}

function parseAgents() {
  const raw = process.env.KABOOM_SKILL_TARGETS || process.env.KABOOM_SKILL_TARGET;
  if (!raw) return DEFAULT_AGENTS;
  const requested = raw
    .split(',')
    .map((v) => v.trim().toLowerCase())
    .filter(Boolean);
  const filtered = requested.filter((v) => DEFAULT_AGENTS.includes(v));
  return filtered.length > 0 ? filtered : DEFAULT_AGENTS;
}

function maybeProjectRoot() {
  const initCwd = process.env.INIT_CWD;
  if (!initCwd) return null;
  const resolved = path.resolve(initCwd);
  if (resolved.includes(`node_modules${path.sep}`)) return null;
  return resolved;
}

function detectDefaultScope() {
  const isGlobalInstall = String(process.env.npm_config_global || '')
    .trim()
    .toLowerCase() === 'true';
  if (isGlobalInstall) return 'global';
  return maybeProjectRoot() ? 'project' : 'global';
}

function parseScope(defaultScope = detectDefaultScope()) {
  const scope = String(process.env.KABOOM_SKILL_SCOPE || defaultScope).trim().toLowerCase();
  if (scope === 'project' || scope === 'global' || scope === 'all') return scope;
  return defaultScope;
}

const SKILLS_DIR_ENV_BY_AGENT = {
  claude: 'KABOOM_CLAUDE_SKILLS_DIR',
  codex: 'KABOOM_CODEX_SKILLS_DIR',
  gemini: 'KABOOM_GEMINI_SKILLS_DIR',
};

/**
 * Returns true when bundled skills may be installed into rootDir for agent.
 * Skill roots are only created inside agent layouts that ALREADY exist
 * (e.g. ~/.claude, ~/.gemini) so the installer never fabricates agent dirs
 * that would later trip client auto-detection. Explicit env-var targets
 * (KABOOM_<AGENT>_SKILLS_DIR) are always honored and force-created.
 */
function isAgentRootInstallable(agent, rootDir) {
  const envVar = SKILLS_DIR_ENV_BY_AGENT[agent];
  const envValue = envVar ? process.env[envVar] : '';
  if (envValue && path.resolve(envValue) === path.resolve(rootDir)) {
    return true;
  }
  return fs.existsSync(path.dirname(path.resolve(rootDir)));
}

function getAgentRoots(agent, scope) {
  const roots = [];
  const home = os.homedir();
  const projectRoot = maybeProjectRoot();

  const globalByAgent = {
    claude: process.env.KABOOM_CLAUDE_SKILLS_DIR || path.join(home, '.claude', 'skills'),
    codex:
      process.env.KABOOM_CODEX_SKILLS_DIR ||
      path.join(process.env.CODEX_HOME || path.join(home, '.codex'), 'skills'),
    gemini:
      process.env.KABOOM_GEMINI_SKILLS_DIR ||
      path.join(process.env.GEMINI_HOME || path.join(home, '.gemini'), 'skills'),
  };

  const projectByAgent = projectRoot
    ? {
        claude: path.join(projectRoot, '.claude', 'skills'),
        codex: path.join(projectRoot, '.codex', 'skills'),
        gemini: path.join(projectRoot, '.gemini', 'skills'),
      }
    : null;

  if (scope === 'global' || scope === 'all') {
    roots.push(globalByAgent[agent]);
  }
  if ((scope === 'project' || scope === 'all') && projectByAgent) {
    roots.push(projectByAgent[agent]);
  }

  return [...new Set(roots.map((r) => path.resolve(r)))];
}

function readSkillBody(sourcePath) {
  return fs.readFileSync(sourcePath, 'utf8').trimEnd() + '\n';
}

function parseManifest(manifestText) {
  let parsed;
  try {
    parsed = JSON.parse(manifestText);
  } catch (err) {
    throw new Error(`invalid skills manifest JSON: ${err.message}`);
  }
  if (!parsed || !Array.isArray(parsed.skills)) {
    throw new Error('invalid skills manifest: expected { skills: [] }');
  }
  return parsed.skills.filter((skill) => skill && typeof skill.id === 'string' && skill.id);
}

function normalizeRepoRelativePath(pathValue) {
  const normalized = String(pathValue || '').trim().replace(/\\/g, '/');
  return normalized.replace(/^\/+/, '');
}

function joinRepoPath(...parts) {
  const clean = parts
    .filter(Boolean)
    .map((part) => normalizeRepoRelativePath(part))
    .filter(Boolean);
  if (clean.length === 0) return '';
  return path.posix.join(...clean);
}

function ensureRepoRelativePath(pathValue, contextLabel) {
  if (!pathValue) return '';
  const normalized = normalizeRepoRelativePath(path.posix.normalize(pathValue));
  if (normalized === '.') return '';
  if (normalized === '..' || normalized.startsWith('../')) {
    throw new Error(`invalid ${contextLabel}: path escapes repository root`);
  }
  return normalized;
}

function resolveSkillPath(skill, manifestPath, defaultSkillsPath) {
  const manifestDir = ensureRepoRelativePath(
    path.posix.dirname(normalizeRepoRelativePath(manifestPath || '')),
    'manifest path'
  );
  const defaultRoot =
    ensureRepoRelativePath(normalizeRepoRelativePath(defaultSkillsPath || ''), 'skills path') ||
    (manifestDir === '.' ? '' : normalizeRepoRelativePath(manifestDir));

  if (skill.path && /^https?:\/\//i.test(skill.path)) {
    return skill.path;
  }

  if (skill.path) {
    const raw = normalizeRepoRelativePath(skill.path);
    if (raw.startsWith('./') || raw.startsWith('../')) {
      const resolved = path.posix.normalize(path.posix.join(manifestDir, raw));
      return ensureRepoRelativePath(resolved, `path for skill '${skill.id}'`);
    }
    return ensureRepoRelativePath(path.posix.normalize(raw), `path for skill '${skill.id}'`);
  }

  return ensureRepoRelativePath(joinRepoPath(defaultRoot, skill.id, 'SKILL.md'), `path for skill '${skill.id}'`);
}

function loadSkillsFromLocalDir(dirPath, options = {}) {
  const manifestRel = options.skillsManifestPath || process.env.KABOOM_SKILLS_MANIFEST_PATH || 'skills.json';
  const rootDir = path.resolve(dirPath);
  const manifestPath = path.join(dirPath, manifestRel);
  if (!path.resolve(manifestPath).startsWith(rootDir + path.sep) && path.resolve(manifestPath) !== rootDir) {
    throw new Error(`invalid manifest path: ${manifestRel}`);
  }
  if (!fs.existsSync(manifestPath)) {
    throw new Error(`skills manifest not found at ${manifestPath}`);
  }

  const manifestText = fs.readFileSync(manifestPath, 'utf8');
  const manifestSkills = parseManifest(manifestText);

  return manifestSkills
    .map((skill) => {
      const relPath = resolveSkillPath(
        skill,
        manifestRel,
        options.skillsPath || process.env.KABOOM_SKILLS_PATH || ''
      );
      const sourcePath = path.resolve(dirPath, relPath);
      if (!sourcePath.startsWith(rootDir + path.sep) && sourcePath !== rootDir) return null;
      if (!fs.existsSync(sourcePath)) return null;
      return {
        id: skill.id,
        version: skill.version || 1,
        body: readSkillBody(sourcePath),
      };
    })
    .filter(Boolean);
}

function parseGitHubRepo(repoSpec) {
  const raw = String(repoSpec || '').trim();
  if (!raw) {
    throw new Error('empty repository spec');
  }

  if (/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(raw)) {
    const [owner, repo] = raw.split('/');
    return { owner, repo, inferredRef: null, inferredPath: null };
  }

  if (raw.startsWith('https://github.com/')) {
    const withoutPrefix = raw
      .replace('https://github.com/', '')
      .replace(/\.git$/, '')
      .replace(/\/+$/, '');
    const parts = withoutPrefix.split('/').filter(Boolean);
    if (parts.length >= 2) {
      const owner = parts[0];
      const repo = parts[1];
      if (parts[2] === 'tree' && parts.length >= 4) {
        const inferredRef = decodeURIComponent(parts[3]);
        const inferredPath =
          parts.length > 4 ? parts.slice(4).map((p) => decodeURIComponent(p)).join('/') : null;
        return { owner, repo, inferredRef, inferredPath };
      }
      return { owner, repo, inferredRef: null, inferredPath: null };
    }
  }

  throw new Error(`invalid GitHub repo spec: ${repoSpec}. Use owner/repo or https://github.com/owner/repo`);
}

function fetchText(url, timeoutMs = 8000, redirects = 0) {
  return new Promise((resolve, reject) => {
    const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
    const headers = { 'User-Agent': 'kaboom-mcp-skills' };
    if (token) {
      headers.Authorization = `token ${token}`;
    }

    const req = https.get(url, { timeout: timeoutMs, headers }, (res) => {
      if (
        res.statusCode &&
        [301, 302, 303, 307, 308].includes(res.statusCode) &&
        res.headers.location
      ) {
        if (redirects >= 3) {
          reject(new Error(`too many redirects fetching ${url}`));
          return;
        }
        const redirectURL = toURL(url, res.headers.location);
        fetchText(redirectURL, timeoutMs, redirects + 1).then(resolve).catch(reject);
        return;
      }

      const chunks = [];
      res.on('data', (chunk) => chunks.push(chunk));
      res.on('end', () => {
        const body = Buffer.concat(chunks).toString('utf8');
        if (res.statusCode && res.statusCode >= 200 && res.statusCode < 300) {
          resolve(body);
          return;
        }
        reject(new Error(`HTTP ${res.statusCode || 'unknown'} for ${url}`));
      });
    });

    req.on('timeout', () => {
      req.destroy(new Error(`timeout fetching ${url}`));
    });
    req.on('error', reject);
  });
}

function toURL(base, relOrURL) {
  if (/^https?:\/\//i.test(relOrURL)) return relOrURL;
  return new URL(relOrURL, base).toString();
}

async function loadSkillsFromGitHub(repoSpec, ref, options = {}) {
  const { owner, repo } = parseGitHubRepo(repoSpec);
  const base = `https://raw.githubusercontent.com/${owner}/${repo}/${encodeURIComponent(ref)}/`;

  const manifestPath =
    options.skillsManifestPath || process.env.KABOOM_SKILLS_MANIFEST_PATH || 'skills/skills.json';
  const skillRoot = options.skillsPath || process.env.KABOOM_SKILLS_PATH || '';

  const manifestURL = toURL(base, manifestPath);
  const manifestText = await fetchText(manifestURL);
  const manifestSkills = parseManifest(manifestText);

  const resolved = await Promise.all(
    manifestSkills.map(async (skill) => {
      const relPath = resolveSkillPath(skill, manifestPath, skillRoot);
      const skillURL = toURL(base, relPath);
      const body = await fetchText(skillURL);
      return {
        id: skill.id,
        version: skill.version || 1,
        body: body.trimEnd() + '\n',
      };
    })
  );

  return resolved;
}

function resolveSkillsSource(options = {}) {
  const explicitDir = options.skillsDir || process.env.KABOOM_SKILLS_DIR;
  if (explicitDir) {
    const dir = path.resolve(explicitDir);
    return {
      type: 'local',
      source: `dir:${dir}`,
      dir,
      skillsPath: options.skillsPath || process.env.KABOOM_SKILLS_PATH || '',
      manifestPath: options.skillsManifestPath || process.env.KABOOM_SKILLS_MANIFEST_PATH || 'skills.json',
    };
  }

  const repoInput = options.skillsRepo || process.env.KABOOM_SKILLS_REPO;
  if (repoInput) {
    const parsed = parseGitHubRepo(repoInput);
    const repo = `${parsed.owner}/${parsed.repo}`;
    const ref = options.skillsRef || process.env.KABOOM_SKILLS_REF || parsed.inferredRef || 'main';
    const inferredPath = parsed.inferredPath || '';
    const skillsPath = options.skillsPath || process.env.KABOOM_SKILLS_PATH || inferredPath || '';
    const manifestPath =
      options.skillsManifestPath ||
      process.env.KABOOM_SKILLS_MANIFEST_PATH ||
      (inferredPath ? joinRepoPath(inferredPath, 'skills.json') : 'skills/skills.json');
    return {
      type: 'github',
      source: `github:${repo}@${ref}`,
      repo,
      ref,
      skillsPath,
      manifestPath,
    };
  }

  return {
    type: 'local',
    source: 'bundled',
    dir: BUNDLED_SKILLS_DIR,
    skillsPath: '',
    manifestPath: 'skills.json',
  };
}

async function loadSkillCatalog(options = {}) {
  const source = resolveSkillsSource(options);
  if (source.type === 'local') {
    const skills = loadSkillsFromLocalDir(source.dir, {
      ...options,
      skillsPath: source.skillsPath,
      skillsManifestPath: source.manifestPath,
    });
    return { source: source.source, skills, warnings: [] };
  }

  try {
    const skills = await loadSkillsFromGitHub(source.repo, source.ref, {
      ...options,
      skillsPath: source.skillsPath,
      skillsManifestPath: source.manifestPath,
    });
    return { source: source.source, skills, warnings: [] };
  } catch (err) {
    const fallbackAllowed =
      !parseBoolValue(options.skillsNoFallback) && !parseBoolEnv('KABOOM_SKILLS_NO_FALLBACK');
    if (!fallbackAllowed) {
      throw err;
    }
    const fallbackSkills = loadSkillsFromLocalDir(BUNDLED_SKILLS_DIR, {
      skillsManifestPath: 'skills.json',
      skillsPath: '',
    });
    return {
      source: 'bundled',
      skills: fallbackSkills,
      warnings: [
        `remote skills source failed (${source.source}): ${err.message}; falling back to bundled skills`,
      ],
    };
  }
}

function buildManagedContent(skillId, version, body) {
  return `${MANAGED_MARKER} id:${skillId} version:${version} -->\n${body}`;
}

function isManagedSkillContent(content) {
  return MANAGED_MARKERS.some((marker) => content.includes(marker));
}

function safeWriteManagedFile(filePath, content) {
  const result = { status: 'unchanged', path: filePath, error: null };
  try {
    fs.mkdirSync(path.dirname(filePath), { recursive: true });

    if (fs.existsSync(filePath)) {
      const existing = fs.readFileSync(filePath, 'utf8');
      if (existing === content) {
        result.status = 'unchanged';
        return result;
      }
      if (!isManagedSkillContent(existing)) {
        result.status = 'skipped_user_owned';
        return result;
      }
      fs.writeFileSync(filePath, content, 'utf8');
      result.status = 'updated';
      return result;
    }

    fs.writeFileSync(filePath, content, 'utf8');
    result.status = 'created';
    return result;
  } catch (err) {
    result.status = 'error';
    result.error = err.message;
    return result;
  }
}

function skillFilePath(agent, rootDir, skillId) {
  if (agent === 'codex') {
    return path.join(rootDir, skillId, 'SKILL.md');
  }
  return path.join(rootDir, `${skillId}.md`);
}

function removeManagedSkillFile(agent, rootDir, skillId, options = {}) {
  const { dryRun = false } = options;
  const managedPath = skillFilePath(agent, rootDir, skillId);
  const result = { removed: 0, skipped_user_owned: 0, errors: 0 };
  if (!fs.existsSync(managedPath)) return result;

  try {
    const existing = fs.readFileSync(managedPath, 'utf8');
    if (!isManagedSkillContent(existing)) {
      result.skipped_user_owned += 1;
      return result;
    }
    if (!dryRun) {
      fs.unlinkSync(managedPath);
      if (agent === 'codex') {
        const managedDir = path.dirname(managedPath);
        try {
          fs.rmdirSync(managedDir);
        } catch (err) {
          // Ignore non-empty or missing directory errors.
        }
      }
    }
    result.removed += 1;
    return result;
  } catch (err) {
    result.errors += 1;
    return result;
  }
}

function removeLegacySkillVariants(agent, rootDir, skillId, options = {}) {
  const { dryRun = false } = options;
  let removed = 0;

  for (const prefix of LEGACY_PREFIXES) {
    const legacyId = `${prefix}${skillId}`;
    const legacyPath = skillFilePath(agent, rootDir, legacyId);
    if (!fs.existsSync(legacyPath)) continue;

    try {
      const existing = fs.readFileSync(legacyPath, 'utf8');
      if (!isManagedSkillContent(existing)) continue;
      if (!dryRun) {
        fs.unlinkSync(legacyPath);
      }
      removed += 1;
      if (!dryRun && agent === 'codex') {
        const legacyDir = path.dirname(legacyPath);
        try {
          fs.rmdirSync(legacyDir);
        } catch (err) {
          // Ignore non-empty or missing directory errors.
        }
      }
    } catch (err) {
      // Ignore unreadable legacy artifacts and continue cleanup.
    }
  }

  return removed;
}

function isManagedSkillFileStart(content) {
  const text = String(content || '');
  return MANAGED_MARKERS.some((marker) => text.startsWith(marker));
}

/**
 * Remove managed skill files that are no longer in the bundled manifest
 * (renamed or dropped skills). Only files whose content STARTS with a managed
 * marker are removed — user-owned files are never touched.
 * handledPaths lists files already processed by the manifest-based cleanup so
 * dry-run counts are not duplicated.
 */
function removeOrphanedManagedSkills(agent, rootDir, handledPaths, options = {}) {
  const { dryRun = false } = options;
  const result = { removed: 0, errors: 0 };
  let entries;
  try {
    entries = fs.readdirSync(rootDir, { withFileTypes: true });
  } catch (_) {
    return result; // Root does not exist — nothing to clean.
  }

  for (const entry of entries) {
    let candidate = null;
    if (agent === 'codex') {
      if (entry.isDirectory()) candidate = path.join(rootDir, entry.name, 'SKILL.md');
    } else if (entry.isFile() && entry.name.endsWith('.md')) {
      candidate = path.join(rootDir, entry.name);
    }
    if (!candidate || handledPaths.has(candidate)) continue;
    try {
      if (!fs.existsSync(candidate)) continue;
      const content = fs.readFileSync(candidate, 'utf8');
      if (!isManagedSkillFileStart(content)) continue;
      if (!dryRun) {
        fs.unlinkSync(candidate);
        if (agent === 'codex') {
          try {
            fs.rmdirSync(path.dirname(candidate));
          } catch (_) {
            // Ignore non-empty or missing directory errors.
          }
        }
      }
      result.removed += 1;
    } catch (_) {
      result.errors += 1;
    }
  }
  return result;
}

function cleanupInstalledSkills(options = {}) {
  const verbose = Boolean(options.verbose);
  const dryRun = Boolean(options.dryRun);
  const bundledSkills = loadSkillsFromLocalDir(BUNDLED_SKILLS_DIR, {
    skillsManifestPath: 'skills.json',
    skillsPath: '',
  });
  const agents = options.agents || parseAgents();
  const scope = options.scope || parseScope();
  const summary = {
    removed: 0,
    skipped_user_owned: 0,
    errors: 0,
  };

  for (const agent of agents) {
    const roots = getAgentRoots(agent, scope);
    for (const rootDir of roots) {
      const handledPaths = new Set();
      for (const skill of bundledSkills) {
        handledPaths.add(skillFilePath(agent, rootDir, skill.id));
        for (const prefix of LEGACY_PREFIXES) {
          handledPaths.add(skillFilePath(agent, rootDir, `${prefix}${skill.id}`));
        }
        const current = removeManagedSkillFile(agent, rootDir, skill.id, { dryRun });
        summary.removed += current.removed;
        summary.skipped_user_owned += current.skipped_user_owned;
        summary.errors += current.errors;
        summary.removed += removeLegacySkillVariants(agent, rootDir, skill.id, { dryRun });
        if (verbose && current.removed > 0) {
          console.log(`[kaboom-mcp] skills removed: ${agent}:${skill.id} -> ${rootDir}`);
        }
      }
      // Also remove managed skills orphaned by renames/drops in the manifest.
      const orphans = removeOrphanedManagedSkills(agent, rootDir, handledPaths, { dryRun });
      summary.removed += orphans.removed;
      summary.errors += orphans.errors;
      if (verbose && orphans.removed > 0) {
        console.log(`[kaboom-mcp] orphaned managed skills removed: ${agent} -> ${rootDir} (${orphans.removed})`);
      }
    }
  }

  return { agents, scope, ...summary };
}

// #lizard forgives
async function installBundledSkills(options = {}) {
  const verbose = Boolean(options.verbose);
  const skip =
    parseBoolEnv('KABOOM_SKIP_SKILL_INSTALL') || parseBoolEnv('KABOOM_SKIP_SKILLS_INSTALL');
  if (skip) {
    return {
      skipped: true,
      reason: 'disabled_by_env',
      agents: [],
      source: 'none',
      warnings: [],
      summary: {
        created: 0,
        updated: 0,
        unchanged: 0,
        skipped_user_owned: 0,
        legacy_removed: 0,
        errors: 0,
      },
      results: [],
    };
  }

  const catalog = await loadSkillCatalog(options);
  const bundledSkills = catalog.skills;
  if (bundledSkills.length === 0) {
    return {
      skipped: true,
      reason: 'no_bundled_skills',
      agents: [],
      source: catalog.source,
      warnings: catalog.warnings || [],
      summary: {
        created: 0,
        updated: 0,
        unchanged: 0,
        skipped_user_owned: 0,
        legacy_removed: 0,
        errors: 0,
      },
      results: [],
    };
  }

  const agents = options.agents || parseAgents();
  const scope = options.scope || parseScope();

  const results = [];
  const summary = {
    created: 0,
    updated: 0,
    unchanged: 0,
    skipped_user_owned: 0,
    legacy_removed: 0,
    errors: 0,
  };

  for (const agent of agents) {
    const roots = getAgentRoots(agent, scope);
    for (const rootDir of roots) {
      if (!isAgentRootInstallable(agent, rootDir)) {
        if (verbose) {
          console.log(
            `[kaboom-mcp] skills skipped for ${agent}: agent directory not found (${path.dirname(path.resolve(rootDir))})`
          );
        }
        continue;
      }
      for (const skill of bundledSkills) {
        const content = buildManagedContent(skill.id, skill.version, skill.body);
        const filePath = skillFilePath(agent, rootDir, skill.id);
        const writeResult = safeWriteManagedFile(filePath, content);
        results.push({ agent, rootDir, skill: skill.id, ...writeResult });
        if (summary[writeResult.status] !== undefined) {
          summary[writeResult.status] += 1;
        } else {
          summary.errors += 1;
        }
        if (verbose && writeResult.status !== 'unchanged') {
          const suffix = writeResult.error ? ` (${writeResult.error})` : '';
          console.log(
            `[kaboom-mcp] skills ${writeResult.status}: ${agent}:${skill.id} -> ${filePath}${suffix}`
          );
        }
        summary.legacy_removed += removeLegacySkillVariants(agent, rootDir, skill.id);
      }
    }
  }

  return {
    skipped: false,
    agents,
    scope,
    source: catalog.source,
    warnings: catalog.warnings || [],
    summary,
    results,
  };
}

module.exports = {
  cleanupInstalledSkills,
  installBundledSkills,
  isAgentRootInstallable,
  parseAgents,
  parseScope,
  loadSkillCatalog,
  loadSkillsFromLocalDir,
  resolveSkillsSource,
};
