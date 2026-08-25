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

# the scanner's own pattern catalog stays complete
count="$(secret_patterns | grep -c .)"
if [ "$count" -lt 12 ]; then
    echo "FAIL: pattern catalog shrank to $count entries"
    FAILURES=$((FAILURES + 1))
fi

if [ "$FAILURES" -eq 0 ]; then
    echo "check-secrets contract passed"
else
    echo "check-secrets contract FAILED ($FAILURES)"
    exit 1
fi
