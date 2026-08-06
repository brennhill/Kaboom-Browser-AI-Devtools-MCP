#!/usr/bin/env bash
# lint-tests.sh — Static analysis for common test anti-patterns.
# Zero dependencies. macOS + Linux compatible (no GNU extensions).
# Catches lying tests before they reach CI.
# Exit code 1 if any anti-pattern found.

set -euo pipefail

TEST_DIR="${1:-tests/extension}"
FAIL=0
WARNINGS=0

red()    { printf '\033[31m%s\033[0m\n' "$1"; }
yellow() { printf '\033[33m%s\033[0m\n' "$1"; }
green()  { printf '\033[32m%s\033[0m\n' "$1"; }
dim()    { printf '\033[2m%s\033[0m\n' "$1"; }

report() {
  local severity="$1" file="$2" line="$3" rule="$4" msg="$5"
  if [[ "$severity" == "ERROR" ]]; then
    red "  ERROR [$rule] $file:$line"
    FAIL=1
  else
    yellow "  WARN  [$rule] $file:$line"
    WARNINGS=$((WARNINGS + 1))
  fi
  dim "        $msg"
}

echo "Linting test files in $TEST_DIR..."
echo ""

for file in "$TEST_DIR"/*.test.js; do
  [[ -f "$file" ]] || continue
  basename=$(basename "$file")

  # ── Rule 1: assert.ok(true) — always-passing vacuous assertion ──
  while IFS=: read -r lineno _; do
    report "ERROR" "$basename" "$lineno" "vacuous-assert" \
      "assert.ok(true) always passes — replace with a real assertion or use t.skip()"
  done < <(grep -n 'assert\.ok(true' "$file" 2>/dev/null || true)

  # ── Rule 2: Conditional assertion guards — if (module.fn) { assert... } ──
  # Tests that wrap their entire body in if(module.X) pass vacuously when X isn't exported
  while IFS=: read -r lineno _; do
    report "ERROR" "$basename" "$lineno" "conditional-assert" \
      "Assertions inside if(module.fn) guard — passes vacuously if fn not exported. Assert existence first or use t.skip()"
  done < <(grep -n -E 'if\s*\(.*(bgModule|Module)\.' "$file" 2>/dev/null | grep -iv 'skip\|assert' || true)

  # ── Rule 3: Test re-implements handler logic inline ──
  # Pattern: if (message.type === '...') inside a test file = re-implementing production handler
  while IFS=: read -r lineno _; do
    report "ERROR" "$basename" "$lineno" "reimplemented-handler" \
      "Handler logic (if message.type===) in test file — test should call real handler, not re-implement it"
  done < <(grep -n -E 'if\s*\(.*message\.type\s*===' "$file" 2>/dev/null || true)

  # ── Rule 4: Test file with no production imports ──
  # A test file that never imports from ../../extension/ is suspicious
  has_prod_import=$(grep -c "from '../../extension/\|from \"../../extension/" "$file" 2>/dev/null || true)
  has_prod_import=${has_prod_import:-0}; has_prod_import=${has_prod_import//[^0-9]/}; has_prod_import=${has_prod_import:-0}
  has_tests=$(grep -c "test(" "$file" 2>/dev/null || true)
  has_tests=${has_tests:-0}; has_tests=${has_tests//[^0-9]/}; has_tests=${has_tests:-0}
  if [[ "$has_prod_import" -eq 0 && "$has_tests" -gt 0 ]]; then
    case "$basename" in
      helpers*|fixtures*|setup*) ;;
      *)
        report "WARN" "$basename" "1" "no-prod-import" \
          "Test file has $has_tests tests but no imports from extension/ — may be testing mocks only"
        ;;
    esac
  fi

  # ── Rule 5: catch block with assert.ok(true) — silences import failures ──
  # Detect: } catch { ... assert.ok(true) pattern
  while IFS=: read -r lineno _; do
    # Check next 3 lines after catch for assert.ok(true)
    linenum=$((lineno))
    end=$((linenum + 3))
    if sed -n "${linenum},${end}p" "$file" | grep -q 'assert\.ok(true' 2>/dev/null; then
      report "ERROR" "$basename" "$lineno" "silent-catch" \
        "assert.ok(true) in catch block silences failures — use t.skip() or let it throw"
    fi
  done < <(grep -n 'catch' "$file" 2>/dev/null | grep -v '//' || true)

done

echo ""
echo "──────────────────────────────"
if [[ $FAIL -ne 0 ]]; then
  red "FAILED: Test lint found errors above. Fix before committing."
  exit 1
elif [[ $WARNINGS -gt 0 ]]; then
  yellow "PASSED with $WARNINGS warning(s)."
  exit 0
else
  green "PASSED: No test anti-patterns found."
  exit 0
fi
