#!/bin/bash
# check-json-casing.sh — Enforce snake_case JSON tags in Go structs.
#
# All JSON field tags must use snake_case. camelCase tags are violations
# unless annotated with // SPEC:<name> on the same line or preceding line.
#
# Exit code: 0 if all tags are snake_case, 1 if camelCase found.

set -euo pipefail

echo "=== Checking Go JSON tag casing (must be snake_case) ==="

VIOLATIONS=0

# Find all Go source files (excluding tests, vendor, generated)
while IFS= read -r -d '' file; do
    while IFS= read -r line; do
        # Skip lines with SPEC: annotation (external protocol fields)
        if echo "$line" | grep -q "// SPEC:"; then
            continue
        fi

        # Extract the JSON field name from the tag (macOS-compatible)
        json_field=$(echo "$line" | sed -n 's/.*json:"\([^",]*\).*/\1/p' || true)

        if [ -z "$json_field" ] || [ "$json_field" = "-" ]; then
            continue
        fi

        # Check for camelCase: lowercase letter immediately followed by uppercase letter
        if echo "$json_field" | grep -qE '[a-z][A-Z]'; then
            echo "VIOLATION: $file"
            echo "  $line"
            echo "  Field '$json_field' uses camelCase (expected snake_case)"
            echo ""
            VIOLATIONS=1
        fi
    done < <(grep -n 'json:"[^"]*"' "$file" 2>/dev/null || true)
done < <(find ./cmd ./internal -name "*.go" \
    -not -name "*_test.go" \
    -not -path "*/vendor/*" \
    -type f -print0 2>/dev/null || true)

if [ "$VIOLATIONS" -eq 1 ]; then
    echo "JSON tag casing violations found. All JSON fields must use snake_case."
    echo "External spec fields should be annotated with // SPEC:<name> to be exempt."
    exit 1
fi

echo "All JSON tags use snake_case"
