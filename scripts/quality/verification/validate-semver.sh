#!/bin/bash
# Validate strict semver versioning across Kaboom
# Ensures all version strings are X.Y.Z format (no pre-release, no build metadata)
# Run: ./scripts/quality/verification/validate-semver.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../../.." && pwd)"
STRICT_SEMVER='^[0-9]+\.[0-9]+\.[0-9]+$'
ERRORS=0

echo "🔍 Validating strict semver across all version references..."
echo ""

# 1. Check extension/manifest.json
MANIFEST="$ROOT_DIR/extension/manifest.json"
if [ -f "$MANIFEST" ]; then
    VERSION=$(grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' "$MANIFEST" | cut -d'"' -f4)
    echo "Checking extension/manifest.json: $VERSION"
    if [[ ! $VERSION =~ $STRICT_SEMVER ]]; then
        echo "  ❌ FAIL: Not strict semver (must be X.Y.Z, no -beta, -alpha, +build)"
        ERRORS=$((ERRORS + 1))
    else
        echo "  ✓ Valid"
    fi
fi

echo ""

# 2. Check VERSION file
VERSION_FILE="$ROOT_DIR/VERSION"
if [ -f "$VERSION_FILE" ]; then
    VERSION=$(tr -d ' \n' < "$VERSION_FILE")
    echo "Checking VERSION file: $VERSION"
    if [[ ! $VERSION =~ $STRICT_SEMVER ]]; then
        echo "  ❌ FAIL: Not strict semver (must be X.Y.Z, no -beta, -alpha, +build)"
        ERRORS=$((ERRORS + 1))
    else
        echo "  ✓ Valid"
    fi
fi

echo ""

# 3. Check that no version string contains pre-release or build metadata
if grep -E '"version"[[:space:]]*:[[:space:]]*"[^"]*(-BETA|beta|alpha|rc|\+)' "$ROOT_DIR/extension/manifest.json" 2>/dev/null; then
    echo "❌ FAIL: Found pre-release identifiers in version string"
    ERRORS=$((ERRORS + 1))
fi

if [ $ERRORS -eq 0 ]; then
    echo "✅ All versions use strict semver (X.Y.Z format)"
    exit 0
else
    echo ""
    echo "❌ Found $ERRORS semver validation error(s)"
    echo ""
    echo "To fix:"
    echo "  - extension/manifest.json must have: \"version\": \"X.Y.Z\""
    echo "  - VERSION file must contain: X.Y.Z"
    echo "  - No pre-release (no -beta, -alpha, -rc)"
    echo "  - No build metadata (no +something)"
    exit 1
fi
