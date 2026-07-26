// Purpose: Validate pyproject normalization used by the PyPI packaging pipeline.
// Why: Prevents malformed dependency metadata from breaking publish/install flows.
// Docs: docs/features/npm-preinstall-fix/tech-spec.md

import assert from 'node:assert/strict'
import test from 'node:test'

import { normalizeMainPyprojectContent, validateMainPyprojectContent } from './normalize-pypi-main-pyproject.js'

const version = '0.7.2'
const baseProject = `[project]
name = "kaboom-agentic-browser"
version = "${version}"
description = "Main package"

[project.urls]
Homepage = "https://gokaboom.dev"

[project.scripts]
kaboom-agentic-browser = "kaboom_agentic_browser.__main__:main"
`

test('normalizer moves dependencies from [project.scripts] to [project]', () => {
  const input = `${baseProject}
dependencies = [
    "kaboom-agentic-browser-darwin-arm64==${version}; sys_platform == 'darwin' and platform_machine == 'arm64'",
    "kaboom-agentic-browser-darwin-x64==${version}; sys_platform == 'darwin' and platform_machine == 'x86_64'",
    "kaboom-agentic-browser-linux-arm64==${version}; sys_platform == 'linux' and platform_machine == 'aarch64'",
    "kaboom-agentic-browser-linux-x64==${version}; sys_platform == 'linux' and platform_machine == 'x86_64'",
    "kaboom-agentic-browser-win32-x64==${version}; sys_platform == 'win32'",
]
`

  const result = normalizeMainPyprojectContent(input)
  assert.equal(result.changed, true)
  assert.match(result.content, /\[project\][\s\S]*dependencies = \[/)
  assert.match(result.content, /\[project\.scripts\]\nkaboom-agentic-browser = "kaboom_agentic_browser\.__main__:main"\n?$/)
  assert.equal(
    result.content.match(/dependencies = \[/g)?.length ?? 0,
    1,
    'dependencies block should exist exactly once'
  )
})

test('normalizer removes duplicate dependencies under [project.scripts] when [project] already has one', () => {
  const input = `[project]
name = "kaboom-agentic-browser"
version = "${version}"
dependencies = [
    "kaboom-agentic-browser-win32-x64==${version}; sys_platform == 'win32'",
]

[project.scripts]
kaboom-agentic-browser = "kaboom_agentic_browser.__main__:main"
dependencies = [
    "kaboom-agentic-browser-win32-x64==${version}; sys_platform == 'win32'",
]
`
  const result = normalizeMainPyprojectContent(input)
  assert.equal(result.changed, true)
  assert.equal(result.content.match(/dependencies = \[/g)?.length ?? 0, 1)
})

test('normalizer leaves already-correct structure unchanged', () => {
  const input = `[project]
name = "kaboom-agentic-browser"
version = "${version}"
dependencies = [
    "kaboom-agentic-browser-win32-x64==${version}; sys_platform == 'win32'",
]

[project.scripts]
kaboom-agentic-browser = "kaboom_agentic_browser.__main__:main"
`

  const result = normalizeMainPyprojectContent(input)
  assert.equal(result.changed, false)
  assert.equal(result.content, input)
})

test('validator rejects misplaced dependencies and accepts normalized content', () => {
  const misplaced = `${baseProject}
dependencies = [
    "kaboom-agentic-browser-win32-x64==${version}; sys_platform == 'win32'",
]
`
  const invalid = validateMainPyprojectContent(misplaced, { expectedVersion: version })
  assert.equal(invalid.valid, false)
  assert.match(invalid.errors.join('\n'), /\[project\.scripts\] must not define dependencies/)

  const normalized = `[project]
name = "kaboom-agentic-browser"
version = "${version}"
dependencies = [
    "kaboom-agentic-browser-darwin-arm64==${version}; sys_platform == 'darwin' and platform_machine == 'arm64'",
    "kaboom-agentic-browser-darwin-x64==${version}; sys_platform == 'darwin' and platform_machine == 'x86_64'",
    "kaboom-agentic-browser-linux-arm64==${version}; sys_platform == 'linux' and platform_machine == 'aarch64'",
    "kaboom-agentic-browser-linux-x64==${version}; sys_platform == 'linux' and platform_machine == 'x86_64'",
    "kaboom-agentic-browser-win32-x64==${version}; sys_platform == 'win32'",
]

[project.scripts]
kaboom-agentic-browser = "kaboom_agentic_browser.__main__:main"
`
  const valid = validateMainPyprojectContent(normalized, { expectedVersion: version })
  assert.equal(valid.valid, true, valid.errors.join('\n'))
})
