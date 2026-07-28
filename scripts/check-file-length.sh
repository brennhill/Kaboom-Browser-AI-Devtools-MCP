#!/bin/bash
# check-file-length.sh — Enforce the 800-line limit on every authored source file.
#
# Tests are authored source and are deliberately included. Generated schemas,
# compiled extension output, bundles, vendored code, dependencies, and build
# artifacts are excluded because their canonical inputs—not their outputs—must
# be organized.

set -euo pipefail

MAX_LINES=800
FOUND_VIOLATIONS=0
SCAN_ROOT="${CHECK_FILE_LENGTH_ROOT:-.}"

echo "Checking authored source files against the ${MAX_LINES}-line limit..."
echo ""

check_file() {
    local file="$1"
    local lines
    lines=$(wc -l < "$file" | tr -d ' ')
    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "❌ $file: $lines lines (max: $MAX_LINES)"
        FOUND_VIOLATIONS=1
    fi
}

echo "--- Authored Go, TypeScript, and JavaScript files (including tests) ---"
while IFS= read -r -d '' file; do
    check_file "$file"
done < <(
    find "$SCAN_ROOT" -type f \
        \( -name "*.go" -o -name "*.ts" -o -name "*.tsx" -o \
           -name "*.js" -o -name "*.mjs" -o -name "*.cjs" \) \
        -not -path "*/.git/*" \
        -not -path "*/.agents/*" \
        -not -path "*/.beads/*" \
        -not -path "*/.claude/*" \
        -not -path "*/.planning/*" \
        -not -path "*/node_modules/*" \
        -not -path "*/vendor/*" \
        -not -path "*/testdata/*" \
        -not -path "*/generated/*" \
        -not -path "*/dist/*" \
        -not -path "*/build/*" \
        -not -path "*/coverage/*" \
        -not -path "*/extension/*" \
        -not -path "*/gokaboom.dev/*" \
        -not -path "*/scratchpad/*" \
        -not -name "*.pb.go" \
        -not -name "*.bundled.js" \
        -not -name "*.min.js" \
        -print0 2>/dev/null
)

echo ""

if [ "$FOUND_VIOLATIONS" -eq 1 ]; then
    echo "────────────────────────────────────────────────────────────────"
    echo "Authored files exceed the maximum line limit."
    echo "Split them into focused, change-coupled modules; waivers are not allowed."
    echo "────────────────────────────────────────────────────────────────"
    exit 1
fi

echo "✅ All authored source files are within the line limit"
