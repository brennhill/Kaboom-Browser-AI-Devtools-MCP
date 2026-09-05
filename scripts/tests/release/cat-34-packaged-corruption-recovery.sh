#!/bin/bash
# cat-34-packaged-corruption-recovery.sh — Exercise corrupt persisted state through the packed npm artifact.
# Docs: docs/features/feature/self-testing/index.md
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

PORT="${1:-17890}"
OUTPUT_FILE="${2:-/dev/null}"
export KABOOM_TELEMETRY=off
RAW_FIXTURE_SECRET="RAW_FIXTURE_SECRET_41i3"
PACKAGE_ROOT="$(mktemp -d)"
STATE_ROOT="$PACKAGE_ROOT/state"
HOME_ROOT="$PACKAGE_ROOT/home"
PROJECT_FIXTURE="$PACKAGE_ROOT/project"
PACK_ROOT="$PACKAGE_ROOT/packs"
INSTALL_ROOT="$PACKAGE_ROOT/install"
DAEMON_LOG="$PACKAGE_ROOT/daemon.log"
mkdir -p "$STATE_ROOT" "$HOME_ROOT/.kaboom" "$PROJECT_FIXTURE" "$PACK_ROOT" "$INSTALL_ROOT"

case "$(go env GOOS)-$(go env GOARCH)" in
    darwin-arm64) PLATFORM_DIR="darwin-arm64"; PLATFORM_KEY="darwin-arm64" ;;
    darwin-amd64) PLATFORM_DIR="darwin-x64"; PLATFORM_KEY="darwin-x64" ;;
    linux-arm64) PLATFORM_DIR="linux-arm64"; PLATFORM_KEY="linux-arm64" ;;
    linux-amd64) PLATFORM_DIR="linux-x64"; PLATFORM_KEY="linux-x64" ;;
    *)
        echo "FATAL: unsupported release-UAT platform: $(go env GOOS)-$(go env GOARCH)" >&2
        exit 1
        ;;
esac

MAIN_PACKAGE="$PACKAGE_ROOT/kaboom-agentic-browser"
PLATFORM_PACKAGE="$PACKAGE_ROOT/$PLATFORM_DIR"
cp -R "$PROJECT_ROOT/npm/kaboom-agentic-browser" "$MAIN_PACKAGE"
cp -R "$PROJECT_ROOT/npm/$PLATFORM_DIR" "$PLATFORM_PACKAGE"
mkdir -p "$PLATFORM_PACKAGE/bin"

VERSION="$(tr -d '[:space:]' < "$PROJECT_ROOT/VERSION")"
GOOS="$(go env GOOS)" GOARCH="$(go env GOARCH)" CGO_ENABLED=0 \
    go build -ldflags="-X main.version=$VERSION" \
    -o "$PLATFORM_PACKAGE/bin/kaboom-agentic-browser" "$PROJECT_ROOT/cmd/browser-agent"
GOOS="$(go env GOOS)" GOARCH="$(go env GOARCH)" CGO_ENABLED=0 \
    go build -ldflags="-X main.version=$VERSION" \
    -o "$PLATFORM_PACKAGE/bin/kaboom-hooks" "$PROJECT_ROOT/cmd/hooks"
platform_tgz="$(npm pack "$PLATFORM_PACKAGE" --pack-destination "$PACK_ROOT" --silent)"
main_tgz="$(npm pack "$MAIN_PACKAGE" --pack-destination "$PACK_ROOT" --silent)"
npm install --prefix "$INSTALL_ROOT" --ignore-scripts --omit=optional \
    "$PACK_ROOT/$platform_tgz" "$PACK_ROOT/$main_tgz" >/dev/null
PACKAGED_WRAPPER="$INSTALL_ROOT/node_modules/.bin/kaboom-agentic-browser"
export KABOOM_UAT_WRAPPER="$PACKAGED_WRAPPER"
init_framework "$PORT" "$OUTPUT_FILE"
framework_package_cleanup() {
    framework_cleanup
    rm -rf "$PACKAGE_ROOT"
}
trap framework_package_cleanup EXIT INT TERM
begin_category "34" "Packaged Corruption Recovery" "npm artifact"
begin_test "34.1" "Build and install npm artifacts" \
    "Pack the public launcher and current-platform binary package" \
    "Recovery must execute the same npm layout users install"
if "$PACKAGED_WRAPPER" --version 2>&1 | grep -q "$VERSION"; then
    pass "Packed npm launcher resolves the packed $PLATFORM_KEY binary at version $VERSION"
else
    fail "Packed npm launcher did not resolve the packed platform binary"
    finish_category
fi

project_rel="${PROJECT_FIXTURE#/}"
PROJECT_STATE="$STATE_ROOT/projects/$project_rel"
mkdir -p \
    "$PROJECT_STATE/session" \
    "$PROJECT_STATE/sequences" \
    "$STATE_ROOT/recordings/broken"
printf '{\"secret\":\"%s\"' "$RAW_FIXTURE_SECRET" > "$HOME_ROOT/.kaboom/install_id"
printf '%s' "$RAW_FIXTURE_SECRET" > "$STATE_ROOT/restart-history.json"
printf '%s' "$RAW_FIXTURE_SECRET" > "$PROJECT_STATE/meta.json"
printf '%s' "$RAW_FIXTURE_SECRET" > "$PROJECT_STATE/session/response_mode.json"
printf '%s' "$RAW_FIXTURE_SECRET" > "$PROJECT_STATE/session/broken.json"
printf '%s' "$RAW_FIXTURE_SECRET" > "$PROJECT_STATE/sequences/broken.json"
printf '%s' "$RAW_FIXTURE_SECRET" > "$STATE_ROOT/recordings/broken/metadata.json"

begin_test "34.2" "Startup survives corrupt persisted state" \
    "Launch the packed daemon with isolated corrupt identity, lifecycle, session, preference, sequence, and recording fixtures" \
    "Every loader must fall back without aborting startup or exposing raw values"
(
    cd "$PROJECT_FIXTURE" || exit 1
    HOME="$HOME_ROOT" KABOOM_STATE_DIR="$STATE_ROOT" KABOOM_TELEMETRY=off \
        "$PACKAGED_WRAPPER" --daemon --parallel --port "$PORT" --state-dir "$STATE_ROOT"
) >"$DAEMON_LOG" 2>&1 &
DAEMON_PID="$!"
if wait_for_health 80; then
    pass "Packed daemon reached health after corrupt-state recovery"
else
    fail "Packed daemon did not start with corrupt fixtures"
    finish_category
fi

packaged_call() {
    local tool="$1"
    local arguments="$2"
    printf '{"jsonrpc":"2.0","id":34,"method":"tools/call","params":{"name":"%s","arguments":%s}}\n' \
        "$tool" "$arguments" |
        HOME="$HOME_ROOT" KABOOM_STATE_DIR="$STATE_ROOT" KABOOM_TELEMETRY=off \
            "$PACKAGED_WRAPPER" --port "$PORT"
}

# Trigger lazy loaders before asking Doctor for the active recovery snapshot.
packaged_call "configure" '{"what":"store","store_action":"load","namespace":"session","key":"broken"}' >/dev/null
packaged_call "configure" '{"what":"list_sequences"}' >/dev/null
packaged_call "observe" '{"what":"recordings"}' >/dev/null
doctor_active="$(packaged_call "configure" '{"what":"doctor"}')"

begin_test "34.3" "Doctor reports active recoveries" \
    "Inspect recovery diagnostics after every corrupt loader ran" \
    "Operators need named, actionable state-family failures"
active_names="session_metadata_state response_mode_state saved_sequence_state event_recording_state"
for name in $active_names; do
    if printf '%s' "$doctor_active" | grep -q "$name" &&
        printf '%s' "$doctor_active" | grep -q 'lifecycle.*active'; then
        pass "$name is visible with lifecycle active"
    else
        fail "$name was not visible as an active Doctor recovery"
    fi
done
if printf '%s' "$doctor_active" | grep -q "state_recovery_failed" &&
    printf '%s' "$doctor_active" | grep -q 'correlation_id.*install_identity' &&
    printf '%s' "$doctor_active" | grep -q 'lifecycle.*recovered'; then
    pass "install identity recovery is visible through the canonical incident lifecycle"
else
    fail "install identity recovery was not visible through its canonical code and correlation"
fi
if printf '%s' "$doctor_active" | grep -q "restart_history_state" &&
    printf '%s' "$doctor_active" | grep -q 'lifecycle.*recovered'; then
    pass "restart_history_state records its automatic active-to-recovered transition"
else
    fail "restart_history_state did not retain its recovered lifecycle"
fi

# Repair lazy state in place and exercise the same loader again so Doctor retains
# the transition instead of merely forgetting the earlier failure.
printf '{"summary":false}' > "$PROJECT_STATE/session/response_mode.json"
printf '{"ok":true}' > "$PROJECT_STATE/session/broken.json"
rm -f "$PROJECT_STATE/sequences/broken.json" "$STATE_ROOT/recordings/broken/metadata.json"
packaged_call "configure" '{"what":"store","store_action":"save","namespace":"session","key":"response_mode","data":{"summary":false}}' >/dev/null
packaged_call "configure" '{"what":"store","store_action":"load","namespace":"session","key":"broken"}' >/dev/null
packaged_call "configure" '{"what":"list_sequences"}' >/dev/null
packaged_call "observe" '{"what":"recordings"}' >/dev/null
doctor_recovered="$(packaged_call "configure" '{"what":"doctor"}')"

begin_test "34.4" "Doctor retains recovered transitions" \
    "Repair corrupt fixtures and rerun their canonical loaders" \
    "A resolved incident remains diagnosable with lifecycle recovered"
for name in response_mode_state stored_session_state saved_sequence_state event_recording_state restart_history_state; do
    if printf '%s' "$doctor_recovered" | grep -q "$name" &&
        printf '%s' "$doctor_recovered" | grep -q 'lifecycle.*recovered'; then
        pass "$name is retained with lifecycle recovered"
    else
        fail "$name was not retained as a recovered Doctor diagnostic"
    fi
done

begin_test "34.5" "Corrupt values stay redacted" \
    "Search daemon logs and Doctor output for the unique raw fixture marker" \
    "Recovery evidence must identify state families without leaking persisted content"
# The daemon-log half of this scan only means something if the log exists and
# captured output: `grep -q` over a missing or empty file returns non-zero for
# reasons that have nothing to do with redaction, which would silently retire
# half of the assertion. (The Doctor half is guarded by 34.4.)
if [ ! -s "$DAEMON_LOG" ]; then
    fail "daemon log $DAEMON_LOG is missing or empty; the log half of the redaction scan could not run"
elif grep -q "$RAW_FIXTURE_SECRET" "$DAEMON_LOG" ||
    printf '%s\n%s' "$doctor_active" "$doctor_recovered" | grep -q "$RAW_FIXTURE_SECRET"; then
    fail "Raw persisted fixture content leaked into diagnostics"
else
    pass "Logs ($(wc -c < "$DAEMON_LOG" | tr -d ' ') bytes scanned) and Doctor diagnostics contain no raw persisted fixture values"
fi

finish_category
