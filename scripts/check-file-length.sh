#!/bin/bash
# check-file-length.sh — Enforce 800-line soft limit on source files
#
# Standard: 800 lines per file (soft limit)
# Exceptions: Files with justification comment in first 20 lines
#   - Go: // nolint:filelength - Justification here
#   - TS: // eslint-disable max-lines - Justification here
#
# Generated files are skipped: their size is a property of the generator, not a
# refactoring decision anyone can act on, and nobody may edit them to add a
# justification comment. Two of them (the openapi-typescript output) were failing
# this check permanently, and because `make test` depends on it, that meant the
# whole test target aborted before running a single test.
#
# Exit code: 0 if all files pass, 1 if violations found

set -euo pipefail

MAX_LINES=800
FOUND_VIOLATIONS=0
JUSTIFIED_EXCEPTIONS=0
GENERATED_SKIPPED=0

# Header markers every generator in this repo writes. Matched against the top of
# the file only, where such banners live, so prose further down cannot trip it.
GENERATED_MARKERS='auto-?generated|@generated|do not edit|is generated|do not make direct changes'
GENERATED_HEADER_LINES=10

plural() {
    if [ "$1" -eq 1 ]; then echo "$2"; else echo "${2}s"; fi
}

is_generated() {
    head -"$GENERATED_HEADER_LINES" "$1" | grep -qiE "$GENERATED_MARKERS"
}

echo "Checking for files exceeding ${MAX_LINES} lines..."
echo ""

# Function to check a file
check_file() {
    local file="$1"
    local ext="$2"

    lines=$(wc -l < "$file" | tr -d ' ')

    if [ "$lines" -gt "$MAX_LINES" ]; then
        if is_generated "$file"; then
            GENERATED_SKIPPED=$((GENERATED_SKIPPED + 1))
            return
        fi
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
    -not -name "*.test.ts" \
    -not -name "*.spec.ts" \
    -not -name "*.d.ts" \
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

SUMMARY="✅ All files within line limit"
# Say what was skipped. A gate that quietly ignores files reads as one that
# checked them.
if [ "$JUSTIFIED_EXCEPTIONS" -gt 0 ]; then
    SUMMARY="$SUMMARY ($JUSTIFIED_EXCEPTIONS justified exceptions"
    if [ "$GENERATED_SKIPPED" -gt 0 ]; then
        SUMMARY="$SUMMARY, $GENERATED_SKIPPED generated $(plural "$GENERATED_SKIPPED" file) skipped"
    fi
    SUMMARY="$SUMMARY)"
elif [ "$GENERATED_SKIPPED" -gt 0 ]; then
    SUMMARY="$SUMMARY ($GENERATED_SKIPPED generated $(plural "$GENERATED_SKIPPED" file) skipped)"
fi
echo "$SUMMARY"
