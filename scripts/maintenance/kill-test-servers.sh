#!/bin/bash
# kill-test-servers.sh — Kill zombie Kaboom test servers, preserve main server.
#
# Usage: ./scripts/maintenance/kill-test-servers.sh [main-port]
#   or:  make kill-zombies

MAIN_PORT="${1:-7890}"
KILLED=0

# Find PID of main server (to preserve)
MAIN_PID=""
if command -v lsof &>/dev/null; then
    MAIN_PID=$(lsof -ti tcp:"$MAIN_PORT" 2>/dev/null | head -1 || true)
fi

# Kill all Kaboom processes except main
for pid in $(pgrep -f 'kaboom' 2>/dev/null || true); do
    if [ -n "$MAIN_PID" ] && [ "$pid" = "$MAIN_PID" ]; then
        echo "Preserving main server (PID $pid, port $MAIN_PORT)"
        continue
    fi
    kill -TERM "$pid" 2>/dev/null && ((KILLED++)) || true
done

# Force-kill stragglers after 1s
if [ $KILLED -gt 0 ]; then
    sleep 1
    for pid in $(pgrep -f 'kaboom' 2>/dev/null || true); do
        [ -n "$MAIN_PID" ] && [ "$pid" = "$MAIN_PID" ] && continue
        kill -9 "$pid" 2>/dev/null || true
    done
fi

# Clean temp PID files (not main port)
for f in /tmp/kaboom-*.pid; do
    [ -f "$f" ] || continue
    [[ "$f" == *"$MAIN_PORT"* ]] && continue
    rm -f "$f"
done

# Clean test log files
rm -f /tmp/kaboom-test-*.jsonl

echo "Killed $KILLED zombie process(es). Main server on port $MAIN_PORT preserved."
