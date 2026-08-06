#!/bin/bash
# test-install-hooks-only.sh — Verifies the --hooks-only install flow using local dev binaries.
# This test builds the hooks binary locally and simulates the installer's file placement.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Testing --hooks-only install flow ==="

# Build the hooks binary.
echo "Building kaboom-hooks..."
TEMP_BIN=$(mktemp -d)
trap 'rm -rf "$TEMP_BIN"' EXIT

(cd "$REPO_ROOT" && CGO_ENABLED=0 go build -o "$TEMP_BIN/kaboom-hooks" ./cmd/hooks)

# Verify it runs.
HOOKS_VERSION=$("$TEMP_BIN/kaboom-hooks" --version)
echo "Built kaboom-hooks version: $HOOKS_VERSION"

# Verify --help works.
HOOKS_HELP=$("$TEMP_BIN/kaboom-hooks" --help 2>&1)
if ! echo "$HOOKS_HELP" | grep -q "quality-gate"; then
    echo "FAIL: --help missing quality-gate command"
    exit 1
fi
if ! echo "$HOOKS_HELP" | grep -q "compress-output"; then
    echo "FAIL: --help missing compress-output command"
    exit 1
fi
echo "PASS: --help shows both commands"

# Verify unknown command exits with code 2.
if "$TEMP_BIN/kaboom-hooks" nonexistent 2>/dev/null; then
    echo "FAIL: unknown command should exit non-zero"
    exit 1
fi
echo "PASS: unknown command exits non-zero"

# Verify quality-gate with empty stdin exits 0.
echo "" | "$TEMP_BIN/kaboom-hooks" quality-gate
echo "PASS: quality-gate with empty stdin exits 0"

# Verify compress-output with empty stdin exits 0.
echo "" | "$TEMP_BIN/kaboom-hooks" compress-output
echo "PASS: compress-output with empty stdin exits 0"

# Verify the install.sh script has --hooks-only support.
if ! grep -q "HOOKS_ONLY" "$REPO_ROOT/scripts/setup/install.sh"; then
    echo "FAIL: install.sh missing HOOKS_ONLY support"
    exit 1
fi
if ! grep -q "\-\-hooks-only" "$REPO_ROOT/scripts/setup/install.sh"; then
    echo "FAIL: install.sh missing --hooks-only flag"
    exit 1
fi
echo "PASS: install.sh supports --hooks-only"

# Verify Makefile builds both binaries.
if ! grep -q "HOOKS_BINARY_NAME" "$REPO_ROOT/Makefile"; then
    echo "FAIL: Makefile missing HOOKS_BINARY_NAME"
    exit 1
fi
if ! grep -q "HOOKS_PKG" "$REPO_ROOT/Makefile"; then
    echo "FAIL: Makefile missing HOOKS_PKG"
    exit 1
fi
echo "PASS: Makefile builds both binaries"

# Verify quality gates write kaboom-hooks command.
if ! grep -q '"kaboom-hooks quality-gate"' "$REPO_ROOT/cmd/browser-agent/tools_configure_quality_gates.go"; then
    echo "FAIL: quality gates missing 'kaboom-hooks quality-gate'"
    exit 1
fi
echo "PASS: quality gates reference kaboom-hooks binary"

echo ""
echo "=== All hooks-only install tests passed ==="
