#!/bin/bash
# uat-category-script.sh — Resolve a UAT category id to the one script that runs it.
# Docs: docs/features/feature/self-testing/index.md
#
# PURPOSE: both the comprehensive runner and the transcript recorder locate a
# category by globbing cat-<id>-*.sh. Both used `find ... | head -n 1`, which
# picks whichever path the filesystem happened to return first when the glob
# matches more than one file.
#
# It did match more than one. `cat-33-expectations.sh` sat beside
# `cat-33-connected-action-coverage.sh`, and find returned the expectations table
# first. That file is a sourced library: run as a script it defines variables and
# exits 0. So category 33 — the category that invokes every live MCP mode — never
# ran, and the recorder wrote it off as "no-exchanges".
#
# Ambiguity is therefore an error, not something to resolve by ordering. A
# category id names exactly one runnable script or the caller stops.

# uat_resolve_category_script <tests_root> <category_id>
#
# Prints the path on success. On failure prints nothing to stdout, explains on
# stderr, and returns non-zero so `set -e` callers stop.
uat_resolve_category_script() {
    local tests_root="$1"
    local cat_id="$2"
    local matches match_count

    if [ -z "$tests_root" ] || [ -z "$cat_id" ]; then
        echo "uat_resolve_category_script: needs <tests_root> <category_id>" >&2
        return 2
    fi

    matches="$(find "$tests_root" -type f -name "cat-${cat_id}-*.sh" -print | LC_ALL=C sort)"
    match_count="$(printf '%s' "$matches" | grep -c . || true)"

    case "$match_count" in
        1)
            printf '%s\n' "$matches"
            return 0
            ;;
        0)
            echo "No category script for id ${cat_id}: nothing under ${tests_root} matches cat-${cat_id}-*.sh" >&2
            return 1
            ;;
        *)
            {
                echo "Category id ${cat_id} matches ${match_count} scripts, so which one runs would depend on filesystem order:"
                printf '%s\n' "$matches" | sed 's/^/  /'
                echo "A helper that is sourced rather than run must not live in the cat-<id>-* namespace. Rename it."
            } >&2
            return 1
            ;;
    esac
}
