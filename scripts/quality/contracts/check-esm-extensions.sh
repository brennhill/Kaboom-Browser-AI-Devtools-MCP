#!/bin/bash
# check-esm-extensions.sh — Validate .js extensions on all relative imports.
# Two-layer check: source (catches nodenext blind spots) + compiled output.
set -euo pipefail

FAIL=0

# === Layer 1: Source check (no compile needed) ===
# nodenext enforces .js on `from` imports but NOT on side-effect imports.
# This catches the gap: `import './foo'` (no `from` keyword).
SRC_VIOLATIONS=$(grep -rn "^import '\." src/ --include='*.ts' --include='*.d.ts' | grep -v "\.js'" | grep -v "\.json'" || true)

if [ -n "$SRC_VIOLATIONS" ]; then
  echo "FAIL: Bare side-effect imports found in source (missing .js extension):"
  echo "$SRC_VIOLATIONS"
  echo ""
  echo "nodenext does NOT enforce extensions on side-effect imports (import './foo')."
  echo "Fix: Add .js extensions to these imports in src/"
  FAIL=1
fi

# === Layer 2: Compiled output check (requires compile-ts) ===
EXTENSION_DIR="extension"

if [ ! -d "$EXTENSION_DIR" ]; then
  if [ "$FAIL" -eq 1 ]; then exit 1; fi
  echo "SKIP: $EXTENSION_DIR/ not found (compiled output check skipped)"
  echo "OK: Source imports have .js extensions"
  exit 0
fi

# Find ALL relative imports without .js or .json extensions in compiled JS
# Covers: from '...', import('...'), and import '...' (side-effect)
COMPILED_VIOLATIONS=$(grep -rn "from '\." "$EXTENSION_DIR" --include='*.js' | grep -v "\.js'" | grep -v "\.json'" || true)
COMPILED_VIOLATIONS2=$(grep -rn "import('\." "$EXTENSION_DIR" --include='*.js' | grep -v "\.js'" | grep -v "\.json'" || true)
COMPILED_VIOLATIONS3=$(grep -rn "^import '\." "$EXTENSION_DIR" --include='*.js' | grep -v "\.js'" | grep -v "\.json'" || true)

COMPILED_ALL="$COMPILED_VIOLATIONS$COMPILED_VIOLATIONS2$COMPILED_VIOLATIONS3"

if [ -n "$COMPILED_ALL" ]; then
  echo "FAIL: Bare relative imports found in compiled output (missing .js extension):"
  echo "$COMPILED_ALL"
  FAIL=1
fi

if [ "$FAIL" -eq 1 ]; then
  exit 1
fi

echo "OK: All relative imports have .js extensions (source + compiled)"
