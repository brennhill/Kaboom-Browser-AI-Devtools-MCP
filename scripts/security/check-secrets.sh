#!/bin/bash
# check-secrets.sh — Fail the run when a credential pattern lands in source.
# Docs: docs/features/feature/quality-gates/index.md
#
# WHY A LOCAL SCANNER ALONGSIDE GITLEAKS:
# gitleaks runs in `make security-check` and CI as the deep, curated detector.
# The pre-commit hook needs a zero-dependency, sub-second check of exactly the
# staged files, and it must share one allowlist with the deep scan so a
# fixture allowed there is allowed here too.
#
# The allowlist (.secrets-allowlist) is path-glob based: each line is a glob
# optionally followed by " # reason". Fixtures that deliberately contain fake
# credentials (redaction tests, scanner self-tests, docs examples) are listed
# there; anything else that matches fails the run.

# Pattern catalog: id<TAB>extended regex. Deliberately low-false-positive —
# each entry is a full credential format, not a keyword heuristic.
# openai-legacy-key uses a consuming boundary class instead of the lookahead
# form used in .gitleaks.toml because POSIX ERE (BSD/GNU grep -E) has no
# lookaheads; both forms mean "exactly 20 alnum after sk-", which cannot
# collide with sk-ant- (hyphen breaks the run) or the sk-(proj|...)- family.
secret_patterns() {
    cat <<'PATTERNS'
aws-access-key	AKIA[0-9A-Z]{16}
aws-temporary-key	ASIA[0-9A-Z]{16}
github-token-classic	ghp_[A-Za-z0-9]{36}
github-token-fine	gru_[A-Za-z0-9]{36}
github-pat-fine-grained	github_pat_[A-Za-z0-9_]{22,}
stripe-live-secret	sk_live_[A-Za-z0-9]{24}
stripe-restricted-live	rk_live_[A-Za-z0-9]{24}
stripe-webhook-secret	whsec_[A-Za-z0-9]{24}
private-key-block	-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----
slack-token	xox[baprs]-[A-Za-z0-9-]{10,}
anthropic-key	sk-ant-[A-Za-z0-9-]{20,}
openai-project-key	sk-(proj|svc-acct|admin)[-_][A-Za-z0-9_-]{20,}
openai-legacy-key	sk-[A-Za-z0-9]{20}([^A-Za-z0-9_-]|$)
google-api-key	AIza[0-9A-Za-z_-]{35}
npm-access-token	npm_[A-Za-z0-9]{36}
gitlab-pat	glpat-[A-Za-z0-9_-]{20,}
PATTERNS
}

# allowlist_path — resolved per call so KABOOM_SECRETS_ALLOWLIST overrides
# apply when the script is executed or sourced by tests.
allowlist_path() {
    echo "${KABOOM_SECRETS_ALLOWLIST:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/.secrets-allowlist}"
}

# allowlisted <path> — true when the path matches any allowlist glob.
allowlisted() {
    local path="$1" line glob allowlist
    allowlist="$(allowlist_path)"
    [ -f "$allowlist" ] || return 1
    while IFS= read -r line; do
        line="${line%%#*}"
        glob="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
        [ -z "$glob" ] && continue
        case "$path" in
            $glob) return 0 ;;
        esac
    done < "$allowlist"
    return 1
}

# validate_allowlist — reject glob lines that would silence every finding.
# A bare "*" or "**" matches all paths, so one stray line disables the
# scanner; fail closed instead of scanning with a blanket ignore.
validate_allowlist() {
    local allowlist line glob line_number=0
    allowlist="$(allowlist_path)"
    [ -f "$allowlist" ] || return 0
    while IFS= read -r line; do
        line_number=$((line_number + 1))
        line="${line%%#*}"
        glob="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
        case "$glob" in
            '*'|'**')
                echo "INVALID ALLOWLIST: ${allowlist}:${line_number} is '${glob}' — it would silence every finding; use a specific path glob." >&2
                return 1
                ;;
        esac
    done < "$allowlist"
    return 0
}

# redact <matched-text> — keep a recognisable prefix only.
redact() {
    local text="$1"
    if [ "${#text}" -le 8 ]; then
        echo "${text:0:3}***"
    else
        echo "${text:0:6}***"
    fi
}

# scan_paths <file>... — print findings as `path:line:id:redacted` for every
# pattern hit in non-allowlisted text files. Binary files are skipped by grep -I.
# One grep per pattern (not per file): the tracked set is thousands of files.
scan_paths() {
    local id pattern hit path line_number line_text match
    while IFS=$'\t' read -r id pattern; do
        [ -z "$id" ] && continue
        while IFS= read -r hit; do
            path="${hit%%:*}"
            allowlisted "$path" && continue
            line_number="${hit#*:}"; line_number="${line_number%%:*}"
            line_text="${hit#*:*:}"
            match="$(printf '%s' "$line_text" | grep -oE -e "$pattern" | head -1)"
            [ -z "$match" ] && continue
            echo "${path}:${line_number}:${id}:$(redact "$match")"
        done < <(grep -nHIE -e "$pattern" -- "$@" 2>/dev/null || true)
    done < <(secret_patterns)
}

# staged_paths — files the commit is about to add/copy/modify/rename/type-change.
# R and T matter: a rename-only or type-change-only stage otherwise escapes scan.
staged_paths() {
    git diff --cached --name-only --diff-filter=ACMRT 2>/dev/null || true
}

# tracked_paths — every file under version control.
tracked_paths() {
    git ls-files 2>/dev/null || true
}

check_secrets_main() {
    local mode="${1:---tracked}" findings path
    local -a paths=()
    validate_allowlist || return 1
    # mapfile is bash 4+; macOS still ships bash 3.2, so read portably.
    if [ "$mode" = "--staged" ]; then
        while IFS= read -r path; do [ -n "$path" ] && paths+=("$path"); done < <(staged_paths)
    else
        while IFS= read -r path; do [ -n "$path" ] && paths+=("$path"); done < <(tracked_paths)
    fi
    if [ "${#paths[@]}" -eq 0 ]; then
        echo "No files to scan (${mode})."
        return 0
    fi
    findings="$(scan_paths "${paths[@]}")"
    if [ -n "$findings" ]; then
        echo "SECRET PATTERN(S) FOUND:" >&2
        echo "$findings" >&2
        echo "" >&2
        echo "If this is an intentional fixture, add the path to .secrets-allowlist with a reason." >&2
        echo "Never commit real credentials; rotate anything already exposed." >&2
        return 1
    fi
    echo "OK: no credential patterns in ${#paths[@]} scanned file(s) (${mode})."
    return 0
}

# Source guard: run as CLI only when executed directly, not when sourced by tests.
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    check_secrets_main "$1"
fi
