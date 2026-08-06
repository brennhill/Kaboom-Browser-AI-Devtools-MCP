#!/bin/bash
# check-circular-deps.sh — Detect circular import dependencies in TypeScript.
#
# Uses madge to find circular dependencies in the src/ directory.
# Exit code: 0 always (warning-only until existing cycles are resolved).
# Set CIRCULAR_DEPS_STRICT=1 to fail on cycles.

set -euo pipefail

echo "=== Checking for circular TypeScript dependencies ==="

OUTPUT=$(npx --yes madge --circular --extensions ts --ts-config tsconfig.json src/ 2>/dev/null || true)

if echo "$OUTPUT" | grep -q "^[0-9]\+)"; then
    CYCLE_COUNT=$(echo "$OUTPUT" | grep -c "^[0-9]\+)")
    echo "$OUTPUT"
    echo ""
    echo "WARNING: Found $CYCLE_COUNT circular dependency chain(s)"

    if [ "${CIRCULAR_DEPS_STRICT:-0}" = "1" ]; then
        echo "CIRCULAR_DEPS_STRICT=1 — failing build"
        exit 1
    fi

    echo "(Set CIRCULAR_DEPS_STRICT=1 to enforce)"
else
    echo "No circular dependencies found"
fi
