// launcher-shim.contract.test.cjs — Contracts for the bin/ launchers.
//
// Two invariants, both regressions we have already paid for:
//   1. No launcher resolves the binary through PATH. A missing platform
//      optionalDependency must fail loudly, not silently run a stranger's kaboom.
//   2. No launcher process outlives its child. The Node launchers blocked in
//      execFileSync, ignored signals aimed at the group, and were reparented to
//      PID 1 when npx died above them — ~6,900 idle Node runtimes at ~36 MB each.
'use strict'

const assert = require('node:assert/strict')
const { execFileSync, spawn } = require('node:child_process')
const { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } = require('node:fs')
const { describe, test } = require('node:test')
const { tmpdir } = require('node:os')
const { join, resolve } = require('node:path')

const REPO = resolve(__dirname, '..', '..', '..')
const BIN_DIR = join(REPO, 'npm', 'kaboom-agentic-browser', 'bin')
const SHIMS = ['kaboom-agentic-browser', 'kaboom-hooks']
const CMDS = ['kaboom-agentic-browser.cmd', 'kaboom-hooks.cmd']

const read = (f) => readFileSync(join(BIN_DIR, f), 'utf8')

// The shims document the bug they replaced, so scan code only — never the prose.
const code = (f) =>
  read(f)
    .split('\n')
    .filter((l) => !/^\s*(#|::)/.test(l))
    .join('\n')

describe('no launcher resolves through PATH', () => {
  for (const name of [...SHIMS, ...CMDS]) {
    test(`${name} contains no PATH probe`, () => {
      const src = code(name)
      // The exact constructs the old launchers used.
      assert.doesNotMatch(src, /command -v/, 'uses `command -v` to probe PATH')
      assert.doesNotMatch(src, /\bwhich\s+kaboom/, 'uses `which` to probe PATH')
      assert.doesNotMatch(src, /\bwhere\s+kaboom/, 'uses `where` to probe PATH')
      assert.doesNotMatch(src, /%PATH%|\$PATH/, 'reads PATH directly')
    })

    test(`${name} names the optionalDependency and a repair in its failure`, () => {
      const src = read(name)
      assert.match(src, /optionalDependency/, 'failure does not explain the missing platform package')
      assert.match(src, /npm install -g kaboom-agentic-browser@latest/, 'failure offers no repair command')
      assert.match(src, /@brennhill\/kaboom-agentic-browser-/, 'failure does not name the platform package')
    })
  }

  test('the only binary invocations are absolute, never bare command names', () => {
    for (const name of SHIMS) {
      const src = read(name)
      // Every exec of the product binary goes through a variable holding a path.
      const bareExec = /\bexec\s+kaboom-(agentic-browser|hooks)\b/
      assert.doesNotMatch(src, bareExec, `${name} execs a bare command name`)
    }
  })
})

describe('no launcher process outlives its child', () => {
  for (const name of SHIMS) {
    test(`${name} is a POSIX shell shim, not a Node launcher`, () => {
      assert.match(read(name), /^#!\/bin\/sh\n/, 'missing #!/bin/sh')
      assert.doesNotMatch(code(name), /execFileSync|spawnSync|child_process/, 'still uses a Node child-process API')
    })

    test(`${name} hands off with exec, replacing its own image`, () => {
      // A hand-off is any line that RUNS something with the caller's arguments, so
      // no shell survives it; the loop header that iterates those arguments is not one.
      const handoffs = code(name)
        .split('\n')
        .filter((l) => l.includes('"$@"') && !/^\s*for\s/.test(l))
      assert.ok(handoffs.length > 0, 'no hand-off lines found')
      for (const line of handoffs) {
        assert.match(line.trim(), /^(exec |\[ -x .* \] && exec )/, `hand-off does not exec: ${line.trim()}`)
      }
    })

    test(`${name} is valid POSIX shell`, () => {
      execFileSync('/bin/sh', ['-n', join(BIN_DIR, name)])
    })
  }

  test('the exec shim leaves exactly one process: the binary itself', async () => {
    // Build a package layout the shim will resolve, with a stub binary that
    // reports the PID it is running as.
    const root = mkdtempSync(join(tmpdir(), 'kaboom-shim-'))
    const pkg = join(root, 'npm', 'kaboom-agentic-browser')
    mkdirSync(join(pkg, 'bin'), { recursive: true })
    mkdirSync(join(root, 'dist'), { recursive: true })
    writeFileSync(join(pkg, 'bin', 'kaboom-agentic-browser'), read('kaboom-agentic-browser'))
    chmodSync(join(pkg, 'bin', 'kaboom-agentic-browser'), 0o755)

    const pidFile = join(root, 'child.pid')
    const key = process.platform === 'darwin' ? 'darwin' : 'linux'
    const archKey = process.arch === 'arm64' ? 'arm64' : 'x64'
    const stub = join(root, 'dist', `kaboom-agentic-browser-${key}-${archKey}`)
    writeFileSync(stub, `#!/bin/sh\necho "$$" > "${pidFile}"\nexec cat > /dev/null\n`)
    chmodSync(stub, 0o755)

    const child = spawn(join(pkg, 'bin', 'kaboom-agentic-browser'), [], { stdio: ['pipe', 'inherit', 'inherit'] })
    try {
      await new Promise((r) => setTimeout(r, 1500))
      assert.ok(existsSync(pidFile), 'stub binary never ran')
      const reported = readFileSync(pidFile, 'utf8').trim()

      // The PID we spawned IS the binary: exec replaced the shell's image, so no
      // launcher sits between the client and the server.
      assert.equal(String(child.pid), reported, 'a launcher process survives between client and binary')

      const descendants = execFileSync('ps', ['-eo', 'pid,ppid'], { encoding: 'utf8' })
        .split('\n').slice(1)
        .filter((l) => l.trim().split(/\s+/)[1] === String(child.pid))
      assert.equal(descendants.length, 0, `binary has unexpected descendants: ${descendants}`)
    } finally {
      child.kill('SIGKILL')
    }
  })
})

describe('shim resolution matches the JS resolver', () => {
  const { binaryCandidates, detectPlatform, SERVER_BINARY, HOOKS_BINARY } = require('../../../npm/kaboom-agentic-browser/lib/runtime/resolve-binary')

  test('both use the same override env var names', () => {
    assert.match(read('kaboom-agentic-browser'), new RegExp(SERVER_BINARY.overrideKey))
    assert.match(read('kaboom-hooks'), new RegExp(HOOKS_BINARY.overrideKey))
    // And neither shim knows the other's key, so an override cannot cross binaries.
    assert.doesNotMatch(read('kaboom-hooks'), /KABOOM_BINARY_PATH\b/)
  })

  test('both search the same three node_modules depths', () => {
    for (const name of SHIMS) {
      const src = read(name)
      assert.match(src, /\$PKG_DIR\/node_modules\/\$PKG_NAME\/bin/, `${name} missing nested depth`)
      assert.match(src, /\$PKG_DIR\/\.\.\/\$PKG_NAME\/bin/, `${name} missing hoisted depth`)
      assert.match(src, /\$PKG_DIR\/\.\.\/\.\.\/\$PKG_NAME\/bin/, `${name} missing outer hoisted depth`)
    }
    const info = detectPlatform({ platform: 'darwin', arch: 'arm64' })
    const jsDepths = binaryCandidates(SERVER_BINARY, info, '/proj/node_modules/kaboom-agentic-browser')
    assert.equal(jsDepths.length, 3, 'JS resolver no longer searches exactly three depths')
  })

  test('both gate the dev dist build on the source tree', () => {
    for (const name of SHIMS) {
      assert.match(read(name), /basename "\$\(dirname "\$PKG_DIR"\)"\) *= *"npm"|= "npm"/, `${name} missing source-tree guard`)
    }
    // The JS resolver offers no dist candidate for an installed package.
    const info = detectPlatform({ platform: 'darwin', arch: 'arm64' })
    const installed = binaryCandidates(SERVER_BINARY, info, '/proj/node_modules/kaboom-agentic-browser')
    assert.ok(!installed.some((c) => c.includes(`${require('path').sep}dist${require('path').sep}`)), 'installed layout offers a dist candidate')
  })
})

describe('windows launchers', () => {
  for (const name of CMDS) {
    test(`${name} uses CRLF line endings`, () => {
      const raw = readFileSync(join(BIN_DIR, name), 'latin1')
      assert.ok(raw.includes('\r\n'), 'batch file needs CRLF')
      assert.doesNotMatch(raw, /[^\r]\n/, 'contains bare LF line endings')
    })

    test(`${name} runs the binary directly with no Node runtime in the chain`, () => {
      const src = read(name)
      // Node appears only for the CLI branch, never for the server binary.
      const nodeLines = src.split('\r\n').filter((l) => /\bnode\b/.test(l) && !l.trim().startsWith('::'))
      for (const line of nodeLines) {
        assert.match(line, /lib\\cli\\cli\.js/, `Node used outside the CLI branch: ${line}`)
      }
    })
  }
})
