#!/bin/bash
# check-secrets.test.sh — Pins the credential-pattern scanner contract.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=check-secrets.sh
source "$SCRIPT_DIR/check-secrets.sh"

FAILURES=0

expect() {
    local what="$1" want="$2" got="$3"
    if [ "$want" != "$got" ]; then
        echo "FAIL: $what — want [$want], got [$got]"
        FAILURES=$((FAILURES + 1))
    fi
}

make_fixture() {
    local dir="$1" name="$2" body="$3"
    mkdir -p "$dir"
    printf '%s\n' "$body" > "${dir}/${name}"
    echo "${dir}/${name}"
}

# allowlisted honors globs, comments, and blank lines
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
printf '# fixtures\ninternal/redaction/*_test.go\n\n docs/examples/*.md # doc samples\n' > "$tmp/allowlist"
KABOOM_SECRETS_ALLOWLIST="$tmp/allowlist"

allowlisted "internal/redaction/redaction_test.go"
expect "glob allowlist matches" 0 $?
allowlisted "docs/examples/security.md"
expect "allowlist with trailing comment matches" 0 $?
allowlisted "internal/redaction/engine.go"
expect "non-listed path is not allowed" 1 $?

# scan_paths flags real credential formats with redacted output
clean="$(make_fixture "$tmp/src" clean.go 'const AccessKey = "AKIAIOSFODNN7EXAMPLE"')"
KABOOM_SECRETS_ALLOWLIST="$tmp/empty-allowlist"; : > "$KABOOM_SECRETS_ALLOWLIST"

finding="$(scan_paths "$clean" | head -1)"
case "$finding" in
    *":1:aws-access-key:AKIAIO***") expect "aws key flagged+redacted" ok ok ;;
    *) expect "aws key flagged+redacted" "path:1:aws-access-key:AKIAIO***" "$finding" ;;
esac

gh="$(make_fixture "$tmp/src" gh.txt "token: ghp_$(printf 'a%.0s' {1..36})")"
gh_finding="$(scan_paths "$gh" | head -1)"
case "$gh_finding" in
    *github-token-classic*) expect "github classic token flagged" ok ok ;;
    *) expect "github classic token flagged" "flagged" "$gh_finding" ;;
esac

pk="$(make_fixture "$tmp/src" key.pem '-----BEGIN RSA PRIVATE KEY-----')"
case "$(scan_paths "$pk" | head -1)" in
    *private-key-block*) expect "private key block flagged" ok ok ;;
    *) expect "private key block flagged" "flagged" "missed" ;;
esac

# benign text produces nothing
benign="$(make_fixture "$tmp/src" benign.go 'const Region = "us-east-1" // not a key')"
expect "benign source clean" "" "$(scan_paths "$benign")"

# openai project/service-account/admin keys are flagged (gitleaks v8.24 defaults miss these)
KABOOM_SECRETS_ALLOWLIST="$tmp/empty-allowlist"
oai="$(make_fixture "$tmp/src" openai.txt "key = \"sk-proj-$(printf 'a%.0s' {1..40})\"")"
case "$(scan_paths "$oai" | head -1)" in
    *":1:openai-project-key:sk-pro***") expect "openai project key flagged+redacted" ok ok ;;
    *) expect "openai project key flagged+redacted" "path:1:openai-project-key:sk-pro***" "$(scan_paths "$oai" | head -1)" ;;
esac

svc="$(make_fixture "$tmp/src" openai-svc.txt "token: sk-svc-acct-$(printf 'b%.0s' {1..30})")"
case "$(scan_paths "$svc")" in
    *openai-project-key*) expect "openai svc-acct key flagged" ok ok ;;
    *) expect "openai svc-acct key flagged" "flagged" "$(scan_paths "$svc")" ;;
esac

adm="$(make_fixture "$tmp/src" openai-admin.txt "token: sk-admin-$(printf 'c%.0s' {1..30})")"
case "$(scan_paths "$adm")" in
    *openai-project-key*) expect "openai admin key flagged" ok ok ;;
    *) expect "openai admin key flagged" "flagged" "$(scan_paths "$adm")" ;;
esac

# legacy keys are exactly 20 alnum chars after sk-; longer runs are other formats
legacy="$(make_fixture "$tmp/src" openai-legacy.txt 'const Key = "sk-abcdefghij1234567890"')"
case "$(scan_paths "$legacy" | head -1)" in
    *":1:openai-legacy-key:sk-abc***") expect "openai legacy key flagged+redacted" ok ok ;;
    *) expect "openai legacy key flagged+redacted" "path:1:openai-legacy-key:sk-abc***" "$(scan_paths "$legacy" | head -1)" ;;
esac

longrun="$(make_fixture "$tmp/src" openai-long.txt 'const Key = "sk-abcdefghij1234567890plus"')"
expect "longer alnum run not flagged as legacy key" "" "$(scan_paths "$longrun")"

# sk-ant- stays anthropic-only: never the openai patterns
ant="$(make_fixture "$tmp/src" anthropic.txt "key = \"sk-ant-$(printf 'd%.0s' {1..30})\"")"
ant_out="$(scan_paths "$ant")"
case "$ant_out" in
    *anthropic-key*) expect "sk-ant still hits anthropic pattern" ok ok ;;
    *) expect "sk-ant still hits anthropic pattern" "flagged" "$ant_out" ;;
esac
case "$ant_out" in
    *openai*) expect "sk-ant not flagged as openai" "no openai finding" "$ant_out" ;;
    *) expect "sk-ant not flagged as openai" ok ok ;;
esac

# a bare * or ** allowlist glob fails closed instead of silencing every finding
printf '# fixtures\n*\n' > "$tmp/star-allowlist"
KABOOM_SECRETS_ALLOWLIST="$tmp/star-allowlist"
star_out="$(check_secrets_main --staged 2>&1)"; star_rc=$?
expect "bare * allowlist fails closed" 1 "$star_rc"
case "$star_out" in
    *"silence"*) expect "bare * error explains the effect" ok ok ;;
    *) expect "bare * error explains the effect" "explains silencing" "$star_out" ;;
esac
case "$star_out" in
    *"'*'"*) expect "bare * error names the glob" ok ok ;;
    *) expect "bare * error names the glob" "names '*'" "$star_out" ;;
esac

printf '**\n' > "$tmp/starstar-allowlist"
KABOOM_SECRETS_ALLOWLIST="$tmp/starstar-allowlist"
starstar_rc=0; check_secrets_main --staged >/dev/null 2>&1 || starstar_rc=$?
expect "bare ** allowlist fails closed" 1 "$starstar_rc"

# staged_paths covers renames and type-changes, not just add/copy/modify
repo="$tmp/repo"
git init -q "$repo"
git -C "$repo" -c user.name=contract -c user.email=contract@test commit -q --allow-empty -m init
printf 'content\n' > "$repo/a.txt"
git -C "$repo" add a.txt
git -C "$repo" -c user.name=contract -c user.email=contract@test commit -q -m add
git -C "$repo" mv a.txt b.txt
expect "staged_paths includes rename target" "b.txt" "$(cd "$repo" && staged_paths)"

# the scanner's own pattern catalog stays complete
count="$(secret_patterns | grep -c .)"
if [ "$count" -lt 16 ]; then
    echo "FAIL: pattern catalog shrank to $count entries"
    FAILURES=$((FAILURES + 1))
fi

if [ "$FAILURES" -eq 0 ]; then
    echo "check-secrets contract passed"
else
    echo "check-secrets contract FAILED ($FAILURES)"
    exit 1
fi
