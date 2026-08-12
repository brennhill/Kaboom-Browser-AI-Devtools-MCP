// go-tool-versions.test.mjs — Enforces one pinned security-tool bootstrap contract.
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = (file) => readFileSync(file, 'utf8')

test('local, hook, docs, and CI consume one pinned Go security-tool owner', () => {
  const versions = read('scripts/security/go-tool-versions.env')
  assert.match(versions, /^GOSEC_VERSION=v\d+\.\d+\.\d+$/m)
  assert.match(versions, /^GOVULNCHECK_VERSION=v\d+\.\d+\.\d+$/m)
  assert.doesNotMatch(versions, /latest/)

  const installer = read('scripts/security/install-go-tools.sh')
  assert.match(installer, /source "\$SCRIPT_DIR\/go-tool-versions\.env"/)
  assert.match(installer, /GO_TOOL_BIN=.*go env GOPATH/)
  assert.match(
    installer,
    /GOBIN="\$GO_TOOL_BIN" go install "github\.com\/securego\/gosec\/v2\/cmd\/gosec@\$GOSEC_VERSION"/
  )
  assert.match(
    installer,
    /GOBIN="\$GO_TOOL_BIN" go install "golang\.org\/x\/vuln\/cmd\/govulncheck@\$GOVULNCHECK_VERSION"/
  )

  const workflow = read('.github/workflows/ci.yml')
  assert.match(workflow, /scripts\/security\/install-go-tools\.sh/)
  assert.doesNotMatch(workflow, /go install .*gosec@/)
  assert.doesNotMatch(workflow, /go install .*govulncheck@/)

  const makefile = read('Makefile')
  assert.match(makefile, /include scripts\/security\/go-tool-versions\.env/)
  assert.match(makefile, /^install-security-tools:/m)
  assert.match(makefile, /GOSEC_BIN := \$\(GO_TOOL_BIN\)\/gosec/)
  assert.match(makefile, /GOVULNCHECK_BIN := \$\(GO_TOOL_BIN\)\/govulncheck/)
  assert.match(makefile, /"\$\(GOSEC_BIN\)" -quiet/)
  assert.match(makefile, /"\$\(GOVULNCHECK_BIN\)" \.\/cmd\/browser-agent\/\.\.\. \.\/internal\/\.\.\./)
  assert.doesNotMatch(makefile, /command -v (?:gosec|govulncheck)/)
  assert.doesNotMatch(makefile, /gosec@latest/)

  const docs = read('docs/setup/DEVELOPMENT.md')
  const hook = read('scripts/hooks/pre-commit')
  assert.match(docs, /make install-security-tools/)
  assert.match(hook, /make install-security-tools/)
  assert.doesNotMatch(`${docs}\n${hook}`, /gosec@latest/)
  assert.doesNotMatch(hook, /skipping Go security scan/)
})
