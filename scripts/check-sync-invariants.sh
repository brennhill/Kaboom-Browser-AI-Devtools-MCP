#!/bin/bash
# check-sync-invariants.sh
# Checks for known regression patterns that caused production issues.
# Run in CI to catch regressions early.

set -euo pipefail

echo "Checking sync client invariants..."

# INVARIANT 1: startSyncClient must NOT call resetConnection
# Background: Calling resetConnection in startSyncClient caused rapid
# connect/disconnect cycling (100+ logs per second).
# Fixed in commit 49250df.
SYNC_MANAGER_FILE="src/background/sync/sync-manager.ts"

if [ ! -f "$SYNC_MANAGER_FILE" ]; then
  echo "❌ REGRESSION: expected $SYNC_MANAGER_FILE to exist"
  exit 1
fi

# Check only the startSyncClient function body.
if awk '/^export function startSyncClient\(/,/^}/' "$SYNC_MANAGER_FILE" | grep "resetConnection" > /dev/null 2>&1; then
  echo "❌ REGRESSION: resetConnection() found inside startSyncClient()"
  echo "   This causes rapid connect/disconnect cycling."
  echo "   See commit 49250df for the fix."
  exit 1
fi

echo "✅ Sync client invariants OK"

echo "Checking DOM dispatch invariants..."

DOM_DISPATCH_FILE="src/background/dom/dom-dispatch.ts"

if [ ! -f "$DOM_DISPATCH_FILE" ]; then
  echo "❌ REGRESSION: expected $DOM_DISPATCH_FILE to exist"
  exit 1
fi

# INVARIANT 2: wait_for polling must reuse domPrimitive from dispatch side.
# Background: injecting a second wait helper drifted from domPrimitive selector logic.
if grep -E "import .*domWaitFor" "$DOM_DISPATCH_FILE" > /dev/null 2>&1; then
  echo "❌ REGRESSION: domWaitFor imported in dom-dispatch.ts"
  echo "   wait_for should poll via domPrimitive to keep one selector engine."
  exit 1
fi

# INVARIANT 3: DOMResult contract must come from shared dom-types.ts.
if grep -E "^interface DOMResult" "$DOM_DISPATCH_FILE" > /dev/null 2>&1; then
  echo "❌ REGRESSION: local DOMResult interface found in dom-dispatch.ts"
  echo "   use src/background/dom/dom-types.ts to avoid manual contract drift."
  exit 1
fi

echo "✅ DOM dispatch invariants OK"

echo "Checking DOM code generation invariants..."

# INVARIANT 4: generated dom-primitives.ts must match template source.
if ! node scripts/generate-dom-primitives.js --check > /dev/null 2>&1; then
  echo "❌ REGRESSION: src/background/dom/dom-primitives.ts is out of date"
  echo "   Run: node scripts/generate-dom-primitives.js"
  exit 1
fi

echo "✅ DOM code generation invariants OK"
