#!/bin/bash
# check-file-length.sh — Enforce the 800-line limit on hand-written source files.
# Generated code (*/generated/*), build output (extension/), vendored deps and agent
# worktrees (.claude/) are excluded: they are not organized by hand, so splitting them
# is meaningless and an unfixable violation just teaches people to ignore the gate.
#
# Standard: 800 lines per file (soft limit)
# Exceptions: Files with justification comment in first 20 lines
#   - Go: // nolint:filelength - Justification here
#   - TS: // eslint-disable max-lines - Justification here
#
# Exit code: 0 if all files pass, 1 if violations found

set -euo pipefail

MAX_LINES=800
FOUND_VIOLATIONS=0
JUSTIFIED_EXCEPTIONS=0

echo "Checking for files exceeding ${MAX_LINES} lines..."
echo ""

# Function to check a file
check_file() {
    local file="$1"
    local ext="$2"

    lines=$(wc -l < "$file" | tr -d ' ')

    if [ "$lines" -gt "$MAX_LINES" ]; then
        # Check for justification in first 20 lines
        case "$ext" in
            go)
                if head -20 "$file" | grep -q "nolint:filelength\|Maximum file length exceeded with justification"; then
                    echo "⚠️  $file: $lines lines (justified exception)"
                    JUSTIFIED_EXCEPTIONS=$((JUSTIFIED_EXCEPTIONS + 1))
                else
                    echo "❌ $file: $lines lines (max: $MAX_LINES)"
                    FOUND_VIOLATIONS=1
                fi
                ;;
            ts)
                if head -20 "$file" | grep -q "eslint-disable max-lines\|Maximum file length exceeded"; then
                    echo "⚠️  $file: $lines lines (justified exception)"
                    JUSTIFIED_EXCEPTIONS=$((JUSTIFIED_EXCEPTIONS + 1))
                else
                    echo "❌ $file: $lines lines (max: $MAX_LINES)"
                    FOUND_VIOLATIONS=1
                fi
                ;;
        esac
    fi
}

# Check Go files (excluding tests, vendor, generated)
echo "--- Go files ---"
while IFS= read -r -d '' file; do
    check_file "$file" "go"
done < <(find . -name "*.go" \
    -not -path "*/vendor/*" \
    -not -name "*_test.go" \
    -not -path "*/node_modules/*" \
    -not -path "*/.claude/*" \
    -not -path "*/generated/*" \
    -not -name "*.pb.go" \
    -type f -print0 2>/dev/null || true)

# Check TypeScript files (excluding tests, node_modules, dist)
echo ""
echo "--- TypeScript files ---"
while IFS= read -r -d '' file; do
    check_file "$file" "ts"
done < <(find . -name "*.ts" \
    -not -path "*/node_modules/*" \
    -not -path "*/dist/*" \
    -not -path "*/.claude/*" \
    -not -path "*/generated/*" \
    -not -path "./extension/*" \
    -not -path "./gokaboom.dev/*" \
    -not -name "*.test.ts" \
    -not -name "*.spec.ts" \
    -type f -print0 2>/dev/null || true)

echo ""

if [ "$FOUND_VIOLATIONS" -eq 1 ]; then
    echo "────────────────────────────────────────────────────────────────"
    echo "Files exceed maximum line limit. Please split large files into"
    echo "smaller, focused modules."
    echo ""
    echo "To allow exceptions, add a comment in the first 20 lines explaining why:"
    echo "  Go: // nolint:filelength - Justification here"
    echo "  TS: // eslint-disable max-lines - Justification here"
    echo "────────────────────────────────────────────────────────────────"
    exit 1
fi

if [ "$JUSTIFIED_EXCEPTIONS" -gt 0 ]; then
    echo "✅ All files within line limit ($JUSTIFIED_EXCEPTIONS justified exceptions)"
else
    echo "✅ All files within line limit"
fi
