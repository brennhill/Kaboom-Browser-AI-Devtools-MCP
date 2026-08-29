const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..')
const STALE_SKILL_ARTIFACTS_PATTERN =
  /const STALE_SKILL_ARTIFACTS = Object\.freeze\(\{[\s\S]*?\n\}\);\n/
const PLATFORM_PACKAGES = [
  ['darwin-arm64', '@brennhill/kaboom-agentic-browser-darwin-arm64'],
  ['darwin-x64', '@brennhill/kaboom-agentic-browser-darwin-x64'],
  ['linux-arm64', '@brennhill/kaboom-agentic-browser-linux-arm64'],
  ['linux-x64', '@brennhill/kaboom-agentic-browser-linux-x64'],
  ['win32-x64', '@brennhill/kaboom-agentic-browser-win32-x64']
]

function readJson(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8'))
}

test('platform npm packages use kaboom names and descriptions', () => {
  for (const [folder, packageName] of PLATFORM_PACKAGES) {
    const packageJson = readJson(`npm/${folder}/package.json`)
    const suffix = folder === 'win32-x64' ? '.exe' : ''
    assert.equal(packageJson.name, packageName)
    assert.match(packageJson.description, /Kaboom/)
    assert.doesNotMatch(packageJson.description, /Gasoline/)
    assert.deepEqual(packageJson.files, [`bin/kaboom-agentic-browser${suffix}`, `bin/kaboom-hooks${suffix}`])
  }
})

test('kaboom npm wrapper metadata points at the final repo slug', () => {
  const packageJson = readJson('npm/kaboom-agentic-browser/package.json')

  assert.equal(packageJson.repository.url, 'https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP')
  assert.equal(packageJson.bugs.url, 'https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/issues')
})

test('npm skill installer targets only canonical kaboom-managed output', () => {
  const skillsSource = fs.readFileSync(
    path.join(REPO_ROOT, 'npm/kaboom-agentic-browser/lib/installation/skills.js'),
    'utf8'
  )
  const postinstallSource = fs.readFileSync(
    path.join(REPO_ROOT, 'npm/kaboom-agentic-browser/lib/installation/postinstall-skills.js'),
    'utf8'
  )

  assert.match(skillsSource, /kaboom-managed-skill/)
  assert.match(skillsSource, STALE_SKILL_ARTIFACTS_PATTERN)
  assert.doesNotMatch(
    skillsSource.replace(STALE_SKILL_ARTIFACTS_PATTERN, ''),
    /\b(?:gasoline|strum|legacy)\b/i
  )
  assert.match(postinstallSource, /\[kaboom-mcp\]/)
})

// --- scripts/setup/install-bundled-skills.sh parity with the npm installer ---

const SKILLS_SH = path.join(REPO_ROOT, 'scripts', 'setup', 'install-bundled-skills.sh')

function runSkillsScript(env) {
  return spawnSync('bash', [SKILLS_SH], {
    env: { ...process.env, ...env },
    encoding: 'utf8',
    timeout: 30000,
  })
}

test('install-bundled-skills.sh has valid bash syntax', () => {
  if (process.platform === 'win32') return
  const res = spawnSync('bash', ['-n', SKILLS_SH], { encoding: 'utf8' })
  assert.equal(res.status, 0, `bash -n failed: ${res.stderr}`)
})

test('install-bundled-skills.sh uses per-skill manifest versions and manifest iteration', () => {
  if (process.platform === 'win32') return
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-sh-'))
  try {
    const skillsSrc = path.join(tmp, 'bundled')
    fs.mkdirSync(path.join(skillsSrc, 'alpha'), { recursive: true })
    fs.mkdirSync(path.join(skillsSrc, 'beta'), { recursive: true })
    fs.mkdirSync(path.join(skillsSrc, 'not-in-manifest'), { recursive: true })
    fs.writeFileSync(
      path.join(skillsSrc, 'skills.json'),
      JSON.stringify({ skills: [{ id: 'alpha', version: 1 }, { id: 'beta', version: 2 }] })
    )
    fs.writeFileSync(path.join(skillsSrc, 'alpha', 'SKILL.md'), '# Alpha\n')
    fs.writeFileSync(path.join(skillsSrc, 'beta', 'SKILL.md'), '# Beta\n')
    fs.writeFileSync(path.join(skillsSrc, 'not-in-manifest', 'SKILL.md'), '# Stray\n')

    const claudeRoot = path.join(tmp, 'claude-skills')
    fs.mkdirSync(claudeRoot, { recursive: true })
    const res = runSkillsScript({
      KABOOM_BUNDLED_SKILLS_DIR: skillsSrc,
      KABOOM_CLAUDE_SKILLS_DIR: claudeRoot,
      KABOOM_SKILL_TARGETS: 'claude',
      KABOOM_SKILL_SCOPE: 'global',
    })
    assert.equal(res.status, 0, `script failed:\nstdout: ${res.stdout}\nstderr: ${res.stderr}`)

    const alpha = fs.readFileSync(path.join(claudeRoot, 'alpha', 'SKILL.md'), 'utf8')
    const beta = fs.readFileSync(path.join(claudeRoot, 'beta', 'SKILL.md'), 'utf8')
    assert.match(alpha, /^<!-- kaboom-managed-skill id:alpha version:1 -->/, 'alpha must carry manifest version 1')
    assert.match(beta, /^<!-- kaboom-managed-skill id:beta version:2 -->/, 'beta must carry manifest version 2, not a hardcoded 1')
    assert.equal(
      fs.existsSync(path.join(claudeRoot, 'not-in-manifest', 'SKILL.md')),
      false,
      'must only install manifest-listed skills, not every directory'
    )
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('install-bundled-skills.sh markers match the real bundled manifest versions (site-audit v2)', () => {
  if (process.platform === 'win32') return
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-sh-real-'))
  try {
    const claudeRoot = path.join(tmp, 'claude-skills')
    fs.mkdirSync(claudeRoot, { recursive: true })

    const res = runSkillsScript({
      KABOOM_CLAUDE_SKILLS_DIR: claudeRoot,
      KABOOM_SKILL_TARGETS: 'claude',
      KABOOM_SKILL_SCOPE: 'global',
    })
    assert.equal(res.status, 0, `script failed:\nstdout: ${res.stdout}\nstderr: ${res.stderr}`)

    const manifest = JSON.parse(
      fs.readFileSync(path.join(REPO_ROOT, 'npm/kaboom-agentic-browser/skills/skills.json'), 'utf8')
    )
    for (const skill of manifest.skills) {
      const installed = path.join(claudeRoot, skill.id, 'SKILL.md')
      assert.ok(fs.existsSync(installed), `manifest skill ${skill.id} must be installed`)
      const content = fs.readFileSync(installed, 'utf8')
      const expectedVersion = skill.version || 1
      assert.match(content, /^---\n/, `${skill.id} must retain frontmatter on line 1`)
      assert.match(
        content,
        new RegExp(`\n<!-- kaboom-managed-skill id:${skill.id} version:${expectedVersion} -->\n`),
        `${skill.id} marker version must match the manifest (shell/npm installs must not fight)`
      )
    }
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('install-bundled-skills.sh keeps Claude and Codex YAML frontmatter first', () => {
  if (process.platform === 'win32') return
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-sh-directory-'))
  try {
    for (const [agent, envName] of [
      ['claude', 'KABOOM_CLAUDE_SKILLS_DIR'],
      ['codex', 'KABOOM_CODEX_SKILLS_DIR'],
    ]) {
      const root = path.join(tmp, `${agent}-skills`)
      const res = runSkillsScript({
        [envName]: root,
        KABOOM_SKILL_TARGETS: agent,
        KABOOM_SKILL_SCOPE: 'global',
      })
      assert.equal(res.status, 0, `script failed:\nstdout: ${res.stdout}\nstderr: ${res.stderr}`)

      const content = fs.readFileSync(path.join(root, 'debug', 'SKILL.md'), 'utf8')
      assert.match(content, /^---\n/, `${agent} requires YAML frontmatter on the first line`)
      assert.match(content, /\n<!-- kaboom-managed-skill id:debug version:\d+ -->\n/)
    }
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('install-bundled-skills.sh leaves Gemini on its flat layout', () => {
  if (process.platform === 'win32') return
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-sh-gemini-'))
  try {
    const geminiRoot = path.join(tmp, 'gemini-skills')
    const res = runSkillsScript({
      KABOOM_GEMINI_SKILLS_DIR: geminiRoot,
      KABOOM_SKILL_TARGETS: 'gemini',
      KABOOM_SKILL_SCOPE: 'global',
    })
    assert.equal(res.status, 0, `script failed:\nstdout: ${res.stdout}\nstderr: ${res.stderr}`)
    assert.equal(fs.existsSync(path.join(geminiRoot, 'debug.md')), true)
    assert.equal(fs.existsSync(path.join(geminiRoot, 'debug', 'SKILL.md')), false)
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true })
  }
})

test('installer stamps the install epoch next to the binary (latest-install-wins tiebreaker)', () => {
  const installSh = fs.readFileSync(path.join(REPO_ROOT, 'scripts/setup/install.sh'), 'utf8')
  // A per-install epoch stamp gives the daemon's single-instance takeover a
  // deterministic tiebreaker at equal versions (see install_epoch.go): the latest
  // install wins, so two same-version installs can't thrash into a takeover war.
  assert.match(installSh, /\.kaboom-install-epoch/, 'install.sh must write the .kaboom-install-epoch stamp')
  // Nanosecond units (to match the binary-mtime fallback), with a BSD/macOS
  // whole-seconds fallback since their date lacks %N.
  assert.match(installSh, /date \+%s%N/, 'stamp should prefer GNU date %N nanoseconds')
  assert.match(installSh, /date \+%s\)000000000/, 'stamp needs a BSD/macOS seconds-scaled-to-nanos fallback')
  // Next to the binary in BIN_DIR, where install_epoch.go looks for it.
  assert.match(installSh, /"\$BIN_DIR\/\.kaboom-install-epoch"/, 'stamp must live next to the binary in BIN_DIR')
})
