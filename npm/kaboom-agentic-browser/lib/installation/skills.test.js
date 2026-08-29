// Purpose: Validate bundled skill install/cleanup behavior for agent skill roots.
// Why: Prevents fabricating agent dirs that trip client auto-detection and
//      ensures renamed/dropped managed skills are not orphaned on uninstall.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {
  installBundledSkills,
  cleanupInstalledSkills,
  isAgentRootInstallable,
} = require('./skills');

const SKILL_ENV_VARS = [
  'KABOOM_CLAUDE_SKILLS_DIR',
  'KABOOM_CODEX_SKILLS_DIR',
  'KABOOM_GEMINI_SKILLS_DIR',
  'KABOOM_SKILL_TARGETS',
  'KABOOM_SKILL_TARGET',
  'KABOOM_SKILL_SCOPE',
  'KABOOM_SKIP_SKILL_INSTALL',
  'KABOOM_SKIP_SKILLS_INSTALL',
  'KABOOM_SKILLS_DIR',
  'KABOOM_SKILLS_REPO',
  'KABOOM_SKILLS_PATH',
  'KABOOM_SKILLS_MANIFEST_PATH',
  'INIT_CWD',
];

function withEnv(overrides, fn) {
  const saved = {};
  for (const key of SKILL_ENV_VARS) {
    saved[key] = process.env[key];
    delete process.env[key];
  }
  for (const [key, value] of Object.entries(overrides)) {
    if (value !== undefined) process.env[key] = value;
  }
  const restore = () => {
    for (const key of SKILL_ENV_VARS) {
      if (saved[key] === undefined) {
        delete process.env[key];
      } else {
        process.env[key] = saved[key];
      }
    }
  };
  const result = fn();
  if (result && typeof result.then === 'function') {
    return result.finally(restore);
  }
  restore();
  return result;
}

function makeSkillsFixture(tmp) {
  const skillsDir = path.join(tmp, 'bundled');
  fs.mkdirSync(path.join(skillsDir, 'demo'), { recursive: true });
  fs.writeFileSync(
    path.join(skillsDir, 'skills.json'),
    JSON.stringify({ skills: [{ id: 'demo', version: 3 }] }),
    'utf8'
  );
  fs.writeFileSync(
    path.join(skillsDir, 'demo', 'SKILL.md'),
    '---\nname: demo\ndescription: Demo skill.\n---\n# Demo\nbody\n',
    'utf8'
  );
  return skillsDir;
}

test('installBundledSkills preserves Codex frontmatter as the first document block', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-codex-frontmatter-'));
  try {
    const codexRoot = path.join(tmp, 'codex-skills');
    const skillsDir = makeSkillsFixture(tmp);
    const result = await withEnv({ KABOOM_CODEX_SKILLS_DIR: codexRoot }, () =>
      installBundledSkills({ agents: ['codex'], scope: 'global', skillsDir })
    );

    assert.equal(result.summary.created, 1);
    const content = fs.readFileSync(path.join(codexRoot, 'demo', 'SKILL.md'), 'utf8');
    assert.match(content, /^---\nname: demo\ndescription: Demo skill\.\n---\n/);
    assert.match(content, /\n<!-- kaboom-managed-skill id:demo version:3 -->\n# Demo/);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('installBundledSkills writes Claude skills as <id>/SKILL.md with frontmatter first', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-claude-frontmatter-'));
  try {
    const claudeRoot = path.join(tmp, 'claude-skills');
    const skillsDir = makeSkillsFixture(tmp);
    const result = await withEnv({ KABOOM_CLAUDE_SKILLS_DIR: claudeRoot }, () =>
      installBundledSkills({ agents: ['claude'], scope: 'global', skillsDir })
    );

    assert.equal(result.summary.created, 1);
    const skillPath = path.join(claudeRoot, 'demo', 'SKILL.md');
    const content = fs.readFileSync(skillPath, 'utf8');
    assert.match(content, /^---\nname: demo\ndescription: Demo skill\.\n---\n/);
    assert.match(content, /\n<!-- kaboom-managed-skill id:demo version:3 -->\n# Demo/);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'demo.md')), false);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

// --- isAgentRootInstallable (regression: postinstall fabricated ~/.gemini etc.) ---

test('isAgentRootInstallable requires the agent parent dir to already exist', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-root-'));
  try {
    withEnv({}, () => {
      const existingParent = path.join(tmp, '.claude');
      fs.mkdirSync(existingParent, { recursive: true });
      assert.equal(isAgentRootInstallable('claude', path.join(existingParent, 'skills')), true);
      assert.equal(
        isAgentRootInstallable('gemini', path.join(tmp, '.gemini', 'skills')),
        false,
        'must not install into agent layouts that do not exist'
      );
    });
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('isAgentRootInstallable force-allows explicit env-var skill targets', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-root-'));
  try {
    const forcedRoot = path.join(tmp, 'does-not-exist', 'claude-skills');
    withEnv({ KABOOM_CLAUDE_SKILLS_DIR: forcedRoot }, () => {
      assert.equal(isAgentRootInstallable('claude', forcedRoot), true, 'explicit env target must force-create');
      assert.equal(
        isAgentRootInstallable('claude', path.join(tmp, 'other', 'skills')),
        false,
        'env override only forces its own resolved root'
      );
    });
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('installBundledSkills only installs into agent dirs that already exist', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-gate-'));
  try {
    const projectRoot = path.join(tmp, 'project');
    fs.mkdirSync(path.join(projectRoot, '.claude'), { recursive: true });
    const skillsDir = makeSkillsFixture(tmp);

    const result = await withEnv({ INIT_CWD: projectRoot }, () =>
      installBundledSkills({ agents: ['claude', 'gemini'], scope: 'project', skillsDir })
    );

    assert.equal(result.skipped, false);
    assert.equal(
      fs.existsSync(path.join(projectRoot, '.claude', 'skills', 'demo', 'SKILL.md')),
      true,
      'must install into the existing .claude layout'
    );
    assert.equal(
      fs.existsSync(path.join(projectRoot, '.gemini')),
      false,
      'must NOT fabricate a .gemini dir for an agent that is not installed'
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('installBundledSkills force-creates explicit env-var skill targets', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-force-'));
  try {
    const forcedRoot = path.join(tmp, 'brand-new', 'claude-skills');
    const skillsDir = makeSkillsFixture(tmp);

    const result = await withEnv({ KABOOM_CLAUDE_SKILLS_DIR: forcedRoot }, () =>
      installBundledSkills({ agents: ['claude'], scope: 'global', skillsDir })
    );

    assert.equal(result.skipped, false);
    assert.equal(
      fs.existsSync(path.join(forcedRoot, 'demo', 'SKILL.md')),
      true,
      'explicit env-var target must be created even when its parent did not exist'
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

// --- Orphaned managed skill cleanup (regression: renamed/dropped skills leaked) ---

test('cleanupInstalledSkills removes managed skills no longer in the manifest', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-orphan-'));
  try {
    const claudeRoot = path.join(tmp, 'claude-skills');
    fs.mkdirSync(path.join(claudeRoot, 'old-renamed-skill'), { recursive: true });
    fs.writeFileSync(
      path.join(claudeRoot, 'old-renamed-skill', 'SKILL.md'),
      '---\nname: old-renamed-skill\n---\nrenamed away\n' +
        '<!-- kaboom-managed-skill id:old-renamed-skill version:1 -->\n',
      'utf8'
    );
    fs.writeFileSync(
      path.join(claudeRoot, 'gasoline-ancient.md'),
      '<!-- gasoline-managed-skill id:ancient version:1 -->\nold era\n',
      'utf8'
    );
    fs.writeFileSync(path.join(claudeRoot, 'user-note.md'), '# my own skill\n', 'utf8');
    fs.writeFileSync(
      path.join(claudeRoot, 'mentions-marker.md'),
      '# docs about kaboom\nThe installer writes <!-- kaboom-managed-skill markers.\n',
      'utf8'
    );

    const result = withEnv({ KABOOM_CLAUDE_SKILLS_DIR: claudeRoot }, () =>
      cleanupInstalledSkills({ agents: ['claude'], scope: 'global' })
    );

    assert.equal(result.removed, 2, `expected both managed skill layouts removed, got ${result.removed}`);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'old-renamed-skill')), false);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'gasoline-ancient.md')), false);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'user-note.md')), true, 'user files must survive');
    assert.equal(
      fs.existsSync(path.join(claudeRoot, 'mentions-marker.md')),
      true,
      'files merely mentioning the marker mid-content must survive'
    );
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('cleanupInstalledSkills removes orphaned codex skill directories', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-orphan-codex-'));
  try {
    const codexRoot = path.join(tmp, 'codex-skills');
    fs.mkdirSync(path.join(codexRoot, 'dropped-skill'), { recursive: true });
    fs.writeFileSync(
      path.join(codexRoot, 'dropped-skill', 'SKILL.md'),
      '<!-- kaboom-managed-skill id:dropped-skill version:1 -->\ndropped\n',
      'utf8'
    );
    fs.mkdirSync(path.join(codexRoot, 'user-skill'), { recursive: true });
    fs.writeFileSync(path.join(codexRoot, 'user-skill', 'SKILL.md'), '# mine\n', 'utf8');

    const result = withEnv({ KABOOM_CODEX_SKILLS_DIR: codexRoot }, () =>
      cleanupInstalledSkills({ agents: ['codex'], scope: 'global' })
    );

    assert.ok(result.removed >= 1);
    assert.equal(fs.existsSync(path.join(codexRoot, 'dropped-skill')), false, 'orphaned codex dir must be removed');
    assert.equal(fs.existsSync(path.join(codexRoot, 'user-skill', 'SKILL.md')), true, 'user skills must survive');
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('cleanupInstalledSkills dry-run counts orphans without deleting and without double counting', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-orphan-dry-'));
  try {
    const claudeRoot = path.join(tmp, 'claude-skills');
    fs.mkdirSync(path.join(claudeRoot, 'debug'), { recursive: true });
    // One manifest-known skill + one orphan.
    fs.writeFileSync(
      path.join(claudeRoot, 'debug', 'SKILL.md'),
      '---\nname: debug\n---\ncurrent\n<!-- kaboom-managed-skill id:debug version:1 -->\n',
      'utf8'
    );
    fs.writeFileSync(
      path.join(claudeRoot, 'orphan.md'),
      '<!-- kaboom-managed-skill id:orphan version:1 -->\norphan\n',
      'utf8'
    );

    const result = withEnv({ KABOOM_CLAUDE_SKILLS_DIR: claudeRoot }, () =>
      cleanupInstalledSkills({ agents: ['claude'], scope: 'global', dryRun: true })
    );

    assert.equal(result.removed, 2, 'dry-run must count each managed file exactly once');
    assert.equal(fs.existsSync(path.join(claudeRoot, 'debug', 'SKILL.md')), true);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'orphan.md')), true);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('installBundledSkills reclaims every managed flat Claude variant', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-flat-migration-'));
  try {
    const claudeRoot = path.join(tmp, 'claude-skills');
    fs.mkdirSync(claudeRoot, { recursive: true });
    const staleFiles = [
      ['demo.md', 'kaboom'],
      ['kaboom-demo.md', 'kaboom'],
      ['gasoline-demo.md', 'gasoline'],
      ['strum-demo.md', 'strum'],
    ];
    for (const [filename, markerBrand] of staleFiles) {
      fs.writeFileSync(
        path.join(claudeRoot, filename),
        `<!-- ${markerBrand}-managed-skill id:demo version:1 -->\nold\n`,
        'utf8'
      );
    }

    const result = await withEnv({ KABOOM_CLAUDE_SKILLS_DIR: claudeRoot }, () =>
      installBundledSkills({ agents: ['claude'], scope: 'global', skillsDir: makeSkillsFixture(tmp) })
    );

    assert.equal(result.summary.removed, staleFiles.length);
    for (const [filename] of staleFiles) {
      assert.equal(fs.existsSync(path.join(claudeRoot, filename)), false, `${filename} must be reclaimed`);
    }
    assert.equal(fs.existsSync(path.join(claudeRoot, 'demo', 'SKILL.md')), true);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('installBundledSkills preserves user-authored flat Claude variants', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-flat-user-owned-'));
  try {
    const claudeRoot = path.join(tmp, 'claude-skills');
    fs.mkdirSync(claudeRoot, { recursive: true });
    const userFiles = [
      ['demo.md', 'kaboom'],
      ['kaboom-demo.md', 'kaboom'],
      ['gasoline-demo.md', 'gasoline'],
      ['strum-demo.md', 'strum'],
    ];
    for (const [filename, markerBrand] of userFiles) {
      fs.writeFileSync(
        path.join(claudeRoot, filename),
        `# User skill: ${filename}\nDocs mention <!-- ${markerBrand}-managed-skill inline.\n`,
        'utf8'
      );
    }

    const result = await withEnv({ KABOOM_CLAUDE_SKILLS_DIR: claudeRoot }, () =>
      installBundledSkills({ agents: ['claude'], scope: 'global', skillsDir: makeSkillsFixture(tmp) })
    );

    assert.equal(result.summary.removed, 0);
    for (const [filename] of userFiles) {
      assert.equal(fs.existsSync(path.join(claudeRoot, filename)), true, `${filename} is user-owned`);
    }
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('installBundledSkills reports stale Claude cleanup failures', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-flat-error-'));
  try {
    const claudeRoot = path.join(tmp, 'claude-skills');
    fs.mkdirSync(path.join(claudeRoot, 'demo.md'), { recursive: true });

    const result = await withEnv({ KABOOM_CLAUDE_SKILLS_DIR: claudeRoot }, () =>
      installBundledSkills({ agents: ['claude'], scope: 'global', skillsDir: makeSkillsFixture(tmp) })
    );

    assert.equal(result.summary.errors, 1, 'unexpected stale-file errors must not be discarded');
    assert.equal(fs.existsSync(path.join(claudeRoot, 'demo', 'SKILL.md')), true);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('installBundledSkills leaves Gemini on its flat layout', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-gemini-layout-'));
  try {
    const geminiRoot = path.join(tmp, 'gemini-skills');
    await withEnv({ KABOOM_GEMINI_SKILLS_DIR: geminiRoot }, () =>
      installBundledSkills({ agents: ['gemini'], scope: 'global', skillsDir: makeSkillsFixture(tmp) })
    );

    assert.equal(fs.existsSync(path.join(geminiRoot, 'demo.md')), true);
    assert.equal(fs.existsSync(path.join(geminiRoot, 'demo', 'SKILL.md')), false);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});
