#!/bin/bash
# uat-user-state.test.sh — Deterministic user daemon restoration ownership tests.
# Docs: docs/features/feature/self-testing/index.md

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=uat-user-state.sh
source "$SCRIPT_DIR/uat-user-state.sh"

fail() {
    echo "FAIL: $1" >&2
    exit 1
}

reset_fixture() {
    UAT_USER_STATE_SNAPSHOTTED=1
    UAT_USER_STATE_RESTORED=0
    UAT_USER_STATE_RESTORE_STATUS="not_required"
    UAT_PRIOR_DAEMON_RUNNING=1
    UAT_PRIOR_DAEMON_VERSION="0.9.0"
    UAT_PRIOR_LAUNCHAGENT_REGISTERED=0
    UAT_PRIOR_LAUNCHAGENT_RUNNING=0
    UAT_PRIOR_TRACKED_TAB_ID=""
    UAT_DISPOSABLE_TAB_ID=""
    launchagent_restores=0
    standalone_restores=0
}

uat_close_disposable_tab() { return 0; }
uat_stop_port() { return 0; }
uat_wait_for_daemon() { return 0; }
uat_restore_launchagent() { launchagent_restores=$((launchagent_restores + 1)); }
uat_restore_standalone_daemon() { standalone_restores=$((standalone_restores + 1)); }
uat_restore_stopped_launchagent_registration() { return 0; }

reset_fixture
UAT_PRIOR_LAUNCHAGENT_REGISTERED=1
uat_restore_user_state
[ "$launchagent_restores" -eq 1 ] || fail "registered service was not restored through launchctl"
[ "$standalone_restores" -eq 0 ] || fail "registered service was incorrectly restored as standalone"

reset_fixture
uat_restore_user_state
[ "$launchagent_restores" -eq 0 ] || fail "standalone daemon unexpectedly used launchctl"
[ "$standalone_restores" -eq 1 ] || fail "standalone daemon was not restored"

echo "PASS: UAT daemon restoration preserves service ownership"
