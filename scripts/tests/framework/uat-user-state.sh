#!/bin/bash
# uat-user-state.sh — Snapshot and restore the user's daemon and tracked browser tab.
# Docs: docs/features/feature/self-testing/index.md

UAT_USER_STATE_SNAPSHOTTED=0
UAT_USER_STATE_RESTORED=0
UAT_USER_STATE_RESTORE_STATUS="not_required"
UAT_PRIOR_DAEMON_RUNNING=0
UAT_PRIOR_DAEMON_PID=""
UAT_PRIOR_DAEMON_EXEC=""
UAT_PRIOR_DAEMON_VERSION=""
UAT_PRIOR_LAUNCHAGENT_REGISTERED=0
UAT_PRIOR_LAUNCHAGENT_RUNNING=0
UAT_PRIOR_TRACKED_TAB_ID=""
UAT_PRIOR_TRACKED_TAB_URL=""
UAT_USER_DAEMON_PORT=""
UAT_USER_WRAPPER=""
UAT_CONNECTED_READINESS_REASON=""
# The extension reconnect backoff is capped at 30s with up to 25% jitter.
# Forty-five seconds covers that bounded production delay plus daemon startup.
UAT_CONNECTED_READY_ATTEMPTS="${UAT_CONNECTED_READY_ATTEMPTS:-450}"
UAT_DISPOSABLE_TAB_ID=""
UAT_DISPOSABLE_TAB_URL=""
UAT_DISPOSABLE_TAB_CLOSED=0

uat_find_port_pid() {
    lsof -tiTCP:"$1" -sTCP:LISTEN 2>/dev/null | head -n 1
}

uat_process_executable() {
    lsof -a -p "$1" -d txt -Fn 2>/dev/null | sed -n 's/^n//p' | head -n 1
}

uat_daemon_version() {
    curl -s --max-time 2 "http://127.0.0.1:$1/health" 2>/dev/null |
        jq -r '.version // empty' 2>/dev/null
}

uat_launchagent_registered() {
    [ "$(uname -s)" = "Darwin" ] &&
        launchctl print "gui/$(id -u)/com.kaboom.daemon" >/dev/null 2>&1
}

uat_launchagent_running() {
    [ "$(uname -s)" = "Darwin" ] &&
        launchctl print "gui/$(id -u)/com.kaboom.daemon" 2>/dev/null |
            grep -q 'state = running'
}

uat_call_tool() {
    local wrapper="$1"
    local port="$2"
    local tool="$3"
    local arguments="$4"
    printf '%s\n' \
        '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"'"$tool"'","arguments":'"$arguments"'}}' |
        "$wrapper" --port "$port" 2>/dev/null
}

uat_capture_tracked_tab() {
    local response
    response="$(uat_call_tool "$2" "$1" "observe" '{"what":"tabs"}')" || return 0
    echo "$response" | jq -r '
        .result.content[0].text
        | split("\n")[-1]
        | fromjson
        | (.tabs[]? | select(.tracked == true) | [.id, .url] | @tsv)
    ' 2>/dev/null | head -n 1
}

uat_connected_health_payload() {
    curl -s --max-time 1 "http://127.0.0.1:$1/health" 2>/dev/null
}

uat_connected_tracked_tab() {
    uat_capture_tracked_tab "$1" "$2"
}

uat_check_connected_readiness() {
    local port="$1"
    local wrapper="$2"
    if ! uat_check_extension_readiness "$port"; then
        return 1
    fi
    local tracked=""

    tracked="$(uat_connected_tracked_tab "$port" "$wrapper")"
    if [ -z "$tracked" ]; then
        UAT_CONNECTED_READINESS_REASON="no tracked browser tab on port $port"
        return 1
    fi

    UAT_CONNECTED_READINESS_REASON="ready"
    return 0
}

uat_check_extension_readiness() {
    local port="$1"
    local health=""
    local connected=""
    local last_seen=""

    health="$(uat_connected_health_payload "$port")"
    if [ -z "$health" ]; then
        UAT_CONNECTED_READINESS_REASON="daemon health unavailable on port $port"
        return 1
    fi

    connected="$(printf '%s' "$health" |
        jq -r '.capture.extension_connected // false' 2>/dev/null)"
    if [ "$connected" != "true" ]; then
        last_seen="$(printf '%s' "$health" |
            jq -r '.capture.extension_last_seen // "never"' 2>/dev/null)"
        UAT_CONNECTED_READINESS_REASON="extension not connected on port $port (last seen: $last_seen)"
        return 1
    fi

    UAT_CONNECTED_READINESS_REASON="extension ready"
    return 0
}

uat_readiness_sleep() {
    sleep 0.1
}

uat_wait_for_connected_browser() {
    local port="$1"
    local wrapper="$2"
    local attempt=0

    while [ "$attempt" -lt "$UAT_CONNECTED_READY_ATTEMPTS" ]; do
        if uat_check_connected_readiness "$port" "$wrapper"; then
            return 0
        fi
        attempt=$((attempt + 1))
        [ "$attempt" -lt "$UAT_CONNECTED_READY_ATTEMPTS" ] && uat_readiness_sleep
    done

    echo "Connected UAT readiness timed out: $UAT_CONNECTED_READINESS_REASON" >&2
    return 1
}

uat_new_tab_id() {
    printf '%s' "$1" | jq -r '
        [
            (.. | objects | select(.action? == "new_tab") | .tab_id?),
            (
                .result.content[]?.text
                | split("\n")[]
                | fromjson?
                | .. | objects
                | select(.action? == "new_tab")
                | .tab_id?
            )
        ]
        | map(select(type == "number" and . > 0))
        | first // empty
    ' 2>/dev/null
}

uat_wait_for_disposable_tracking() {
    local attempt=0
    local tracked=""
    while [ "$attempt" -lt "$UAT_CONNECTED_READY_ATTEMPTS" ]; do
        tracked="$(uat_connected_tracked_tab "$UAT_USER_DAEMON_PORT" "$UAT_USER_WRAPPER")"
        if [ "$(printf '%s' "$tracked" | cut -f1)" = "$UAT_DISPOSABLE_TAB_ID" ]; then
            return 0
        fi
        attempt=$((attempt + 1))
        [ "$attempt" -lt "$UAT_CONNECTED_READY_ATTEMPTS" ] && uat_readiness_sleep
    done
    return 1
}

uat_create_disposable_tab() {
    local port="$1"
    local wrapper="$2"
    local response=""

    UAT_USER_DAEMON_PORT="$port"
    UAT_USER_WRAPPER="$wrapper"
    UAT_DISPOSABLE_TAB_URL="${KABOOM_UAT_TEST_URL:-http://127.0.0.1:${port}/tests/interact.html}"
    response="$(uat_call_tool "$wrapper" "$port" "interact" \
        '{"what":"new_tab","url":'"$(printf '%s' "$UAT_DISPOSABLE_TAB_URL" | jq -Rs .)"'}')"
    UAT_DISPOSABLE_TAB_ID="$(uat_new_tab_id "$response")"
    if [ -z "$UAT_DISPOSABLE_TAB_ID" ]; then
        echo "Failed to create disposable UAT tab: no created tab_id in response" >&2
        return 1
    fi
    if ! uat_call_tool "$wrapper" "$port" "interact" \
        '{"what":"switch_tab","tab_id":'"$UAT_DISPOSABLE_TAB_ID"',"set_tracked":true}' >/dev/null; then
        echo "Failed to select disposable UAT tab $UAT_DISPOSABLE_TAB_ID" >&2
        return 1
    fi
    if ! uat_wait_for_disposable_tracking; then
        echo "Failed to track disposable UAT tab $UAT_DISPOSABLE_TAB_ID" >&2
        return 1
    fi
}

uat_ensure_cleanup_daemon() {
    if [ -n "$(uat_connected_health_payload "$UAT_USER_DAEMON_PORT")" ]; then
        return 0
    fi
    [ -x "$UAT_USER_WRAPPER" ] || return 1
    nohup "$UAT_USER_WRAPPER" --daemon --port "$UAT_USER_DAEMON_PORT" \
        >/dev/null 2>&1 < /dev/null &
    uat_wait_for_daemon "$UAT_USER_DAEMON_PORT"
}

uat_close_disposable_tab() {
    [ -n "$UAT_DISPOSABLE_TAB_ID" ] || return 0
    [ "$UAT_DISPOSABLE_TAB_CLOSED" = "0" ] || return 0
    uat_ensure_cleanup_daemon || return 1
    uat_wait_for_extension "$UAT_USER_DAEMON_PORT" || return 1
    uat_call_tool "$UAT_USER_WRAPPER" "$UAT_USER_DAEMON_PORT" "interact" \
        '{"what":"close_tab","tab_id":'"$UAT_DISPOSABLE_TAB_ID"'}' >/dev/null || return 1
    UAT_DISPOSABLE_TAB_CLOSED=1
}

uat_snapshot_user_state() {
    UAT_USER_DAEMON_PORT="$1"
    UAT_USER_WRAPPER="$2"
    UAT_PRIOR_DAEMON_PID="$(uat_find_port_pid "$1")"
    if [ -n "$UAT_PRIOR_DAEMON_PID" ]; then
        UAT_PRIOR_DAEMON_RUNNING=1
        UAT_PRIOR_DAEMON_EXEC="$(uat_process_executable "$UAT_PRIOR_DAEMON_PID")"
        UAT_PRIOR_DAEMON_VERSION="$(uat_daemon_version "$1")"
    fi
    if uat_launchagent_registered; then
        UAT_PRIOR_LAUNCHAGENT_REGISTERED=1
    fi
    if uat_launchagent_running; then
        UAT_PRIOR_LAUNCHAGENT_RUNNING=1
    fi

    local tracked
    tracked="$(uat_capture_tracked_tab "$1" "$2")"
    UAT_PRIOR_TRACKED_TAB_ID="$(printf '%s' "$tracked" | cut -f1)"
    UAT_PRIOR_TRACKED_TAB_URL="$(printf '%s' "$tracked" | cut -f2-)"
    UAT_USER_STATE_SNAPSHOTTED=1
}

uat_stop_port() {
    lsof -tiTCP:"$1" -sTCP:LISTEN 2>/dev/null | xargs kill -9 2>/dev/null || true
}

uat_restore_launchagent() {
    local domain="gui/$(id -u)"
    local service="$domain/com.kaboom.daemon"
    local plist="$HOME/Library/LaunchAgents/com.kaboom.daemon.plist"
    if ! launchctl print "$service" >/dev/null 2>&1 && [ -f "$plist" ]; then
        launchctl bootstrap "$domain" "$plist" >/dev/null 2>&1 || return 1
    fi
    launchctl kickstart -k "$service" >/dev/null 2>&1
}

uat_restore_stopped_launchagent_registration() {
    local domain="gui/$(id -u)"
    local service="$domain/com.kaboom.daemon"
    local plist="$HOME/Library/LaunchAgents/com.kaboom.daemon.plist"
    launchctl print "$service" >/dev/null 2>&1 && return 0
    [ -f "$plist" ] || return 1
    launchctl bootstrap "$domain" "$plist" >/dev/null 2>&1 || return 1
    launchctl kill SIGTERM "$service" >/dev/null 2>&1 || true
}

uat_restore_standalone_daemon() {
    [ -x "$UAT_PRIOR_DAEMON_EXEC" ] || return 1
    nohup "$UAT_PRIOR_DAEMON_EXEC" --daemon --port "$UAT_USER_DAEMON_PORT" \
        >/dev/null 2>&1 < /dev/null &
}

uat_wait_for_daemon() {
    local attempt=0
    local expected_version="${2:-}"
    local actual_version=""
    while [ "$attempt" -lt 50 ]; do
        actual_version="$(uat_daemon_version "$1")"
        if [ -n "$actual_version" ]; then
            if [ -z "$expected_version" ] || [ "$actual_version" = "$expected_version" ]; then
                return 0
            fi
        fi
        attempt=$((attempt + 1))
        sleep 0.1
    done
    return 1
}

uat_wait_for_extension() {
    local port="$1"
    local attempt=0

    while [ "$attempt" -lt "$UAT_CONNECTED_READY_ATTEMPTS" ]; do
        if uat_check_extension_readiness "$port"; then
            return 0
        fi
        attempt=$((attempt + 1))
        [ "$attempt" -lt "$UAT_CONNECTED_READY_ATTEMPTS" ] && uat_readiness_sleep
    done

    echo "Connected UAT extension readiness timed out: $UAT_CONNECTED_READINESS_REASON" >&2
    return 1
}

uat_restore_tracked_tab() {
    local tab_id="$1"
    local tab_url="$2"
    uat_wait_for_extension "$UAT_USER_DAEMON_PORT" || return 1
    uat_call_tool "$UAT_PRIOR_DAEMON_EXEC" "$UAT_USER_DAEMON_PORT" "interact" \
        '{"what":"switch_tab","tab_id":'"$tab_id"',"set_tracked":true}' >/dev/null || return 1
    if [ -n "$tab_url" ]; then
        uat_call_tool "$UAT_PRIOR_DAEMON_EXEC" "$UAT_USER_DAEMON_PORT" "interact" \
            '{"what":"navigate","url":'"$(printf '%s' "$tab_url" | jq -Rs .)"'}' >/dev/null || return 1
    fi
}

uat_restore_user_state() {
    [ "$UAT_USER_STATE_SNAPSHOTTED" = "1" ] || return 0
    [ "$UAT_USER_STATE_RESTORED" = "0" ] || return 0
    UAT_USER_STATE_RESTORED=1
    UAT_USER_STATE_RESTORE_STATUS="restored"

    if ! uat_close_disposable_tab; then
        UAT_USER_STATE_RESTORE_STATUS="failed"
        echo "WARNING: failed to close disposable UAT tab $UAT_DISPOSABLE_TAB_ID" >&2
    fi
    uat_stop_port "$UAT_USER_DAEMON_PORT"
    if [ "$UAT_PRIOR_LAUNCHAGENT_REGISTERED" = "1" ] &&
        [ "$UAT_PRIOR_LAUNCHAGENT_RUNNING" = "0" ]; then
        if ! uat_restore_stopped_launchagent_registration; then
            UAT_USER_STATE_RESTORE_STATUS="failed"
            echo "WARNING: failed to restore stopped Kaboom LaunchAgent registration" >&2
        fi
    fi
    if [ "$UAT_PRIOR_DAEMON_RUNNING" = "1" ]; then
        # A registered service is the durable daemon owner even when launchctl
        # transiently reports it waiting/stopped while another process holds
        # the port. Restoring that state as an unmanaged nohup child allows the
        # runner shell or service supervisor to kill it after cleanup.
        if [ "$UAT_PRIOR_LAUNCHAGENT_REGISTERED" = "1" ]; then
            if ! uat_restore_launchagent; then
                UAT_USER_STATE_RESTORE_STATUS="failed"
                echo "WARNING: failed to restore Kaboom LaunchAgent" >&2
            fi
        else
            if ! uat_restore_standalone_daemon; then
                UAT_USER_STATE_RESTORE_STATUS="failed"
                echo "WARNING: failed to restore prior Kaboom daemon" >&2
            fi
        fi
        if uat_wait_for_daemon "$UAT_USER_DAEMON_PORT" "$UAT_PRIOR_DAEMON_VERSION" &&
            [ -n "$UAT_PRIOR_TRACKED_TAB_ID" ]; then
            if ! uat_restore_tracked_tab "$UAT_PRIOR_TRACKED_TAB_ID" "$UAT_PRIOR_TRACKED_TAB_URL"; then
                UAT_USER_STATE_RESTORE_STATUS="failed"
                echo "WARNING: failed to restore tracked tab $UAT_PRIOR_TRACKED_TAB_ID" >&2
            fi
        fi
    fi
}

uat_exit_for_signal() {
    case "$1" in
        INT) exit 130 ;;
        TERM) exit 143 ;;
        HUP) exit 129 ;;
        *) exit 1 ;;
    esac
}
