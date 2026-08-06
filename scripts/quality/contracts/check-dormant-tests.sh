#!/usr/bin/env bash
# check-dormant-tests.sh — Fail on Go or Node tests excluded from default suites.
#
# Why this exists: five separate test files in this repo were sitting behind
# `//go:build integration` with justifications that were plainly false by the
# time anyone looked —
#
#   internal/security/security_test.go        1295 lines  "NetworkBody needs to be
#                                                          imported from capture"  (it imported fine)
#   internal/security/security_flagging_test.go 323 lines  same tag, same story
#   internal/analysis/binary_integration_test.go           the "justification" was a
#                                                          run instruction, not a constraint
#   internal/redaction/redaction_test.go       823 lines  "raceEnabled needs to be
#                                                          exported or defined here" (it was defined
#                                                          in the same package)
#   internal/export/sarif/export_test.go       977 lines  "requires MCP handler types
#                                                          that aren't exported" (referenced none)
#
# That is ~3,400 lines and several hundred assertions that CI never ran, in code
# covering credential redaction and security scanning. The failure mode is always
# the same: a tag gets added mid-refactor to unblock a build, the note describes a
# transient problem, and nobody revisits it once the problem goes away.
#
# A dormant test is worse than no test. It looks like coverage in the tree and on
# a checklist, and it silently rots — `internal/capture/boundary_test.go.skip`
# drifted so far it no longer compiles against the code it claims to test.
#
# So: excluding a Go test from the default suite is a decision that must be made
# deliberately and reviewed, not inherited. This gate makes it loud.
#
# Escape hatch: add the file to DORMANT_ALLOWLIST below with a reason. That keeps
# the exclusion visible in review instead of buried in a file header.

set -euo pipefail

cd "$(dirname "$0")/../../.."

# Files permitted to sit outside the default suite, with why.
# Format: one path per line. Keep the reason on the line above.
DORMANT_ALLOWLIST=$(
  cat <<'EOF'
# Real-binary lifecycle tests run on every CI invocation in the named
# "Go Integration Checks" job; excluding them from repeated unit/coverage
# passes prevents load-sensitive daemon overlap without making them dormant.
cmd/browser-agent/bridge_faststart_extended_test.go
cmd/browser-agent/bridge_faststart_test.go
cmd/browser-agent/bridge_startup_contention_test.go
cmd/browser-agent/cli_modes_subprocess_test.go
cmd/browser-agent/integration_test.go
cmd/browser-agent/mcp_initialize_test.go
cmd/browser-agent/mcp_protocol_test.go
cmd/browser-agent/server_persistence_test.go
cmd/browser-agent/server_reliability_integration_test.go
cmd/browser-agent/server_reliability_test.go
cmd/browser-agent/stdio_silence_test.go
EOF
)

fail=0

# ---------------------------------------------------------------------------
# 1. Build-tag-gated test files.
#
# Platform and race tags are legitimate — they select an implementation for a
# real build configuration rather than removing a test from every run. Anything
# else (integration, e2e, manual, slow, production, ...) means "this does not run
# in CI" and needs justifying.
# ---------------------------------------------------------------------------
legit='^(darwin|linux|windows|freebsd|netbsd|openbsd|js|wasip1|unix|race|cgo|go1\.[0-9]+)$'

while IFS= read -r file; do
  [ -n "$file" ] || continue

  # Read the build constraint's terms, stripping negation and boolean syntax.
  terms=$(grep -m1 -oE '^//go:build .*' "$file" 2>/dev/null |
    sed 's|^//go:build ||' |
    tr '&|()!' ' ' |
    tr -s ' ' '\n' |
    grep -v '^$' || true)

  [ -n "$terms" ] || continue

  suspicious=""
  while IFS= read -r term; do
    [ -n "$term" ] || continue
    if ! echo "$term" | grep -qE "$legit"; then
      suspicious="$suspicious $term"
    fi
  done <<<"$terms"

  [ -n "$suspicious" ] || continue

  if echo "$DORMANT_ALLOWLIST" | grep -qxF "$file"; then
    continue
  fi

  echo "❌ $file is excluded from the default test suite by build tag:$suspicious"
  echo "   $(grep -m1 -oE '^//go:build .*' "$file")"
  echo "   A tag that removes a test from every CI run needs a reason that is still true."
  echo "   Verify it: delete the tag, then run 'go vet ./<pkg>/' and the package's tests."
  echo "   If they compile and pass, the tag is stale — remove it. If the exclusion is"
  echo "   genuinely intended, add the path to DORMANT_ALLOWLIST in $0 with a reason."
  echo
  fail=1
done < <(grep -rl '^//go:build' --include='*_test.go' . 2>/dev/null |
  grep -v '/node_modules/' | grep -v '/.claude/worktrees/' | sed 's|^\./||' | sort)

# ---------------------------------------------------------------------------
# 2. Test files disabled by renaming them out of Go's sight.
#
# `foo_test.go.skip` and friends are invisible to the toolchain, so they cannot
# even fail to compile. They rot silently and without limit.
# ---------------------------------------------------------------------------
while IFS= read -r file; do
  [ -n "$file" ] || continue
  if echo "$DORMANT_ALLOWLIST" | grep -qxF "$file"; then
    continue
  fi
  echo "❌ $file is a Go test disabled by file extension."
  echo "   The toolchain never sees it, so it cannot even fail to compile — it just rots."
  echo "   Either restore it to a *_test.go name and make it pass, or delete it."
  echo
  fail=1
done < <(find . -name '*_test.go.*' \
  -not -path './node_modules/*' \
  -not -path './.claude/worktrees/*' \
  -not -path './.git/*' 2>/dev/null | sed 's|^\./||' | sort)

# ---------------------------------------------------------------------------
# 3. Node test files absent from the canonical sharded runner.
# ---------------------------------------------------------------------------
ACTIVE_JS_TESTS=$(bash scripts/uat/runners/test-js-sharded.sh --list)
while IFS= read -r file; do
  [ -n "$file" ] || continue
  if ! echo "$ACTIVE_JS_TESTS" | grep -qxF "$file"; then
    echo "❌ $file is a Node test absent from scripts/uat/runners/test-js-sharded.sh."
    echo "   Add its root/extension to the canonical inventory or remove the stale test."
    echo
    fail=1
  fi
done < <(find tests scripts extension/background -type f \
  \( -name '*.test.js' -o -name '*.test.cjs' -o -name '*.test.mjs' \) | sort)

if [ "$fail" -ne 0 ]; then
  echo "Dormant tests found. A test excluded from default CI is worse than no test:"
  echo "it reads as coverage while catching nothing, and drifts out of sync with the code."
  exit 1
fi

echo "✅ No dormant Go or Node tests."
